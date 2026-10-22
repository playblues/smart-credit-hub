package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"smart-credit-hub/internal/api"
	"smart-credit-hub/internal/app"
	"smart-credit-hub/internal/auth"
	"smart-credit-hub/internal/config"
	"smart-credit-hub/internal/domain"
	"smart-credit-hub/internal/mock"
	"smart-credit-hub/internal/scoring"
	"smart-credit-hub/internal/storage"
)

func setupApplicationsTestRouter(t *testing.T) (http.Handler, auth.Service, storage.ApplicationRepository, *app.PipelineService) {
	t.Helper()

	cfg := &config.Config{
		Port:               ":8080",
		JWTSecret:          "test-secret-key-apps-12345",
		JWTExpirationHours: 1,
		CasesFilePath:      config.FindCasesFilePath(""),
		CORSAllowedOrigins: []string{"*"},
		Environment:        "test",
	}

	mockSvc, err := mock.NewService(cfg.CasesFilePath)
	if err != nil {
		t.Fatalf("ошибка инициализации mock.Service: %v", err)
	}

	userRepo := storage.NewInMemoryUserRepository(auth.GetDemoUsers()...)
	tokenManager := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTExpirationHours)
	authSvc := auth.NewService(tokenManager, userRepo)

	appRepo := storage.NewInMemoryApplicationRepository()

	scoringModel, err := scoring.LoadModel("")
	if err != nil {
		t.Fatalf("ошибка загрузки скоринговой модели: %v", err)
	}
	scoringEngine := scoring.NewEngine(scoringModel)

	pipeline := app.NewPipelineService(appRepo, mockSvc, scoringEngine, nil)
	// Для тестов убираем задержку
	pipeline.SetStepDelay(0)

	router := api.NewRouter(api.Dependencies{
		Config:      cfg,
		AuthService: authSvc,
		MockService: mockSvc,
		AppRepo:     appRepo,
		Pipeline:    pipeline,
		Pinger:      nil,
	})

	return router, authSvc, appRepo, pipeline
}

// 1. Тест успешной подачи кредитной заявки клиентом
func TestApplications_Create_Success(t *testing.T) {
	router, authSvc, _, _ := setupApplicationsTestRouter(t)

	loginResp, err := authSvc.Authenticate(context.Background(), "client_alfa", "demo123")
	if err != nil {
		t.Fatalf("ошибка авторизации: %v", err)
	}

	reqBody := domain.CreateApplicationRequest{
		INN:                 "7701000001",
		ProductType:         domain.ProductWorkingCapital,
		RequestedAmount:     3000000,
		RequestedTermMonths: 12,
		Purpose:             domain.StringPtr("Закупка запчастей"),
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("ожидался статус 201 Created, получен: %d, тело: %s", rec.Code, rec.Body.String())
	}

	var appResp domain.Application
	if err := json.Unmarshal(rec.Body.Bytes(), &appResp); err != nil {
		t.Fatalf("ошибка парсинга Application: %v", err)
	}

	if appResp.ID == "" {
		t.Error("ID созданной заявки не должен быть пустым")
	}
	if appResp.INN != reqBody.INN {
		t.Errorf("ожидался ИНН %s, получен %s", reqBody.INN, appResp.INN)
	}
	if appResp.Status != domain.StatusSubmitted {
		t.Errorf("начальный статус должен быть SUBMITTED, получен: %s", appResp.Status)
	}
	if appResp.RequestedAmount != reqBody.RequestedAmount {
		t.Errorf("ожидалась сумма %f, получена %f", reqBody.RequestedAmount, appResp.RequestedAmount)
	}
}

// 2. Тест ошибок валидации при создании заявки
func TestApplications_Create_ValidationErrors(t *testing.T) {
	router, authSvc, _, _ := setupApplicationsTestRouter(t)

	loginResp, err := authSvc.Authenticate(context.Background(), "client_alfa", "demo123")
	if err != nil {
		t.Fatalf("ошибка авторизации: %v", err)
	}

	cases := []struct {
		name string
		req  domain.CreateApplicationRequest
	}{
		{
			name: "Сумма <= 0",
			req: domain.CreateApplicationRequest{
				INN:                 "7701000001",
				ProductType:         domain.ProductWorkingCapital,
				RequestedAmount:     0,
				RequestedTermMonths: 12,
			},
		},
		{
			name: "Срок <= 0",
			req: domain.CreateApplicationRequest{
				INN:                 "7701000001",
				ProductType:         domain.ProductWorkingCapital,
				RequestedAmount:     1000000,
				RequestedTermMonths: 0,
			},
		},
		{
			name: "Некорректный ИНН",
			req: domain.CreateApplicationRequest{
				INN:                 "123",
				ProductType:         domain.ProductWorkingCapital,
				RequestedAmount:     1000000,
				RequestedTermMonths: 12,
			},
		},
		{
			name: "Недопустимый продукт",
			req: domain.CreateApplicationRequest{
				INN:                 "7701000001",
				ProductType:         domain.ProductType("MORTGAGE"),
				RequestedAmount:     1000000,
				RequestedTermMonths: 12,
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			bodyBytes, _ := json.Marshal(tc.req)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+loginResp.Token)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("[%s] ожидался статус 400 Bad Request, получен: %d", tc.name, rec.Code)
			}
		})
	}
}

// 3. Тест попытки клиента подать заявку по чужому ИНН
func TestApplications_Create_MismatchedINN(t *testing.T) {
	router, authSvc, _, _ := setupApplicationsTestRouter(t)

	// Авторизуемся как client_alfa (ИНН 7701000001)
	loginResp, err := authSvc.Authenticate(context.Background(), "client_alfa", "demo123")
	if err != nil {
		t.Fatalf("ошибка авторизации: %v", err)
	}

	// Пытаемся подать заявку за чужую компанию (ИНН 7701000002)
	reqBody := domain.CreateApplicationRequest{
		INN:                 "7701000002",
		ProductType:         domain.ProductWorkingCapital,
		RequestedAmount:     5000000,
		RequestedTermMonths: 24,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("ожидался статус 403 Forbidden, получен: %d", rec.Code)
	}

	var errResp domain.ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &errResp)
	if errResp.Code != domain.ErrCodeForbidden {
		t.Errorf("ожидался код FORBIDDEN, получен: %s", errResp.Code)
	}
}

// 4. Тест получения списка заявок клиентом
func TestApplications_List_Client(t *testing.T) {
	router, authSvc, _, _ := setupApplicationsTestRouter(t)

	loginResp, err := authSvc.Authenticate(context.Background(), "client_alfa", "demo123")
	if err != nil {
		t.Fatalf("ошибка авторизации: %v", err)
	}

	// Подаем 2 заявки
	for i := 0; i < 2; i++ {
		reqBody := domain.CreateApplicationRequest{
			INN:                 "7701000001",
			ProductType:         domain.ProductWorkingCapital,
			RequestedAmount:     float64(1000000 * (i + 1)),
			RequestedTermMonths: 12,
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+loginResp.Token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
	}

	// Получаем список заявок
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil)
	reqList.Header.Set("Authorization", "Bearer "+loginResp.Token)
	recList := httptest.NewRecorder()
	router.ServeHTTP(recList, reqList)

	if recList.Code != http.StatusOK {
		t.Fatalf("ожидался статус 200 OK, получен: %d", recList.Code)
	}

	var apps []*domain.Application
	if err := json.Unmarshal(recList.Body.Bytes(), &apps); err != nil {
		t.Fatalf("ошибка парсинга списка заявок: %v", err)
	}

	if len(apps) != 2 {
		t.Errorf("ожидалось 2 заявки, получено: %d", len(apps))
	}
}

// 5. Тест разграничения доступа при получении заявки по ID
func TestApplications_GetByID_AccessControl(t *testing.T) {
	router, authSvc, _, _ := setupApplicationsTestRouter(t)

	// Клиент 1 создает заявку
	login1, _ := authSvc.Authenticate(context.Background(), "client_alfa", "demo123")
	reqBody := domain.CreateApplicationRequest{
		INN:                 "7701000001",
		ProductType:         domain.ProductWorkingCapital,
		RequestedAmount:     3000000,
		RequestedTermMonths: 12,
	}
	bodyBytes, _ := json.Marshal(reqBody)
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewReader(bodyBytes))
	reqCreate.Header.Set("Content-Type", "application/json")
	reqCreate.Header.Set("Authorization", "Bearer "+login1.Token)
	recCreate := httptest.NewRecorder()
	router.ServeHTTP(recCreate, reqCreate)

	var createdApp domain.Application
	_ = json.Unmarshal(recCreate.Body.Bytes(), &createdApp)

	// 1. Клиент 1 читает свою заявку -> 200 OK
	reqGetOwn := httptest.NewRequest(http.MethodGet, "/api/v1/applications/"+createdApp.ID, nil)
	reqGetOwn.Header.Set("Authorization", "Bearer "+login1.Token)
	recGetOwn := httptest.NewRecorder()
	router.ServeHTTP(recGetOwn, reqGetOwn)

	if recGetOwn.Code != http.StatusOK {
		t.Errorf("клиент 1 должен успешно прочитать свою заявку, получен статус: %d", recGetOwn.Code)
	}

	// 2. Клиент 2 пытается прочитать заявку клиента 1 -> 403 Forbidden
	login2, _ := authSvc.Authenticate(context.Background(), "client_smirnov", "demo123")
	reqGetAlien := httptest.NewRequest(http.MethodGet, "/api/v1/applications/"+createdApp.ID, nil)
	reqGetAlien.Header.Set("Authorization", "Bearer "+login2.Token)
	recGetAlien := httptest.NewRecorder()
	router.ServeHTTP(recGetAlien, reqGetAlien)

	if recGetAlien.Code != http.StatusForbidden {
		t.Errorf("клиенту 2 должен быть запрещен доступ к чужой заявке (403), получен статус: %d", recGetAlien.Code)
	}

	// 3. Андеррайтер читает заявку клиента 1 -> 200 OK
	loginUnderwriter, _ := authSvc.Authenticate(context.Background(), "underwriter", "demo123")
	reqGetUnderwriter := httptest.NewRequest(http.MethodGet, "/api/v1/applications/"+createdApp.ID, nil)
	reqGetUnderwriter.Header.Set("Authorization", "Bearer "+loginUnderwriter.Token)
	recGetUnderwriter := httptest.NewRecorder()
	router.ServeHTTP(recGetUnderwriter, reqGetUnderwriter)

	if recGetUnderwriter.Code != http.StatusOK {
		t.Errorf("андеррайтер должен иметь доступ к любой заявке, получен статус: %d", recGetUnderwriter.Code)
	}

	// 4. Запрос несуществующей заявки -> 404 Not Found
	reqNotFound := httptest.NewRequest(http.MethodGet, "/api/v1/applications/00000000-0000-0000-0000-000000000000", nil)
	reqNotFound.Header.Set("Authorization", "Bearer "+login1.Token)
	recNotFound := httptest.NewRecorder()
	router.ServeHTTP(recNotFound, reqNotFound)

	if recNotFound.Code != http.StatusNotFound {
		t.Errorf("для несуществующей заявки ожидался статус 404, получен: %d", recNotFound.Code)
	}
}

// 6. Тест полного цикла кредитного конвейера (Pipeline) до финального статуса
func TestPipeline_Execution_Success_AutoApproved(t *testing.T) {
	_, _, appRepo, pipeline := setupApplicationsTestRouter(t)

	// Создаем заявку вручную в статусе SUBMITTED для кейса CASE_1_GREEN (7701000001)
	userID := "00000000-0000-0000-0000-000000000001"
	appID := "11111111-1111-1111-1111-111111111111"

	appToProcess := &domain.Application{
		ID:                  appID,
		UserID:              &userID,
		INN:                 "7701000001",
		Status:              domain.StatusSubmitted,
		ProductType:         domain.ProductWorkingCapital,
		RequestedAmount:     3000000,
		RequestedTermMonths: 12,
		Purpose:             domain.StringPtr("Тестовый оборотный кредит"),
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	if err := appRepo.Create(context.Background(), nil, appToProcess); err != nil {
		t.Fatalf("ошибка сохранения начальной заявки: %v", err)
	}

	// Запуск синхронного прогона конвейера
	if err := pipeline.ProcessSync(context.Background(), appID); err != nil {
		t.Fatalf("ошибка выполнения конвейера: %v", err)
	}

	// Проверяем обновленное состояние в репозитории
	updatedApp, err := appRepo.GetByID(context.Background(), appID)
	if err != nil {
		t.Fatalf("ошибка чтения заявки после конвейера: %v", err)
	}

	// Для CASE_1_GREEN ожидается AUTO_APPROVED
	if updatedApp.Status != domain.StatusAutoApproved {
		t.Errorf("ожидался статус AUTO_APPROVED, получен: %s", updatedApp.Status)
	}
	if updatedApp.ApprovedAmount == nil || *updatedApp.ApprovedAmount != 3000000 {
		t.Errorf("ожидалась одобренная сумма 3000000, получено: %v", updatedApp.ApprovedAmount)
	}
	if updatedApp.ApprovedTermMonths == nil || *updatedApp.ApprovedTermMonths != 12 {
		t.Errorf("ожидался одобренный срок 12, получено: %v", updatedApp.ApprovedTermMonths)
	}
	if updatedApp.InterestRate == nil || *updatedApp.InterestRate != 0.185 {
		t.Errorf("ожидалась ставка 0.185, получено: %v", updatedApp.InterestRate)
	}

	// Проверка наличия снапшота компании и результата скоринга
	if updatedApp.CompanySnapshot == nil {
		t.Error("company_snapshot должен быть сохранен")
	} else if updatedApp.CompanySnapshot.INN != "7701000001" {
		t.Errorf("ИНН в снапшоте: %s, ожидался 7701000001", updatedApp.CompanySnapshot.INN)
	}

	if updatedApp.ScoringResult == nil {
		t.Error("scoring_result должен быть сохранен")
	} else {
		if updatedApp.ScoringResult.RiskZone != domain.RiskZoneGreen {
			t.Errorf("ожидалась зона GREEN, получена: %s", updatedApp.ScoringResult.RiskZone)
		}
		if updatedApp.ScoringResult.Score != 995 {
			t.Errorf("ожидался скоринговый балл 995, получен: %d", updatedApp.ScoringResult.Score)
		}
	}

	// Проверка истории переходов (SUBMITTED -> ENRICHMENT -> SCORING -> AUTO_APPROVED)
	if len(updatedApp.StatusHistory) < 4 {
		t.Errorf("ожидалось не менее 4 записей в истории аудита, получено: %d", len(updatedApp.StatusHistory))
	}
}
