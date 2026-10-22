package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"smart-credit-hub/internal/api"
	"smart-credit-hub/internal/api/handlers"
	"smart-credit-hub/internal/app"
	"smart-credit-hub/internal/auth"
	"smart-credit-hub/internal/config"
	"smart-credit-hub/internal/domain"
	"smart-credit-hub/internal/mock"
	"smart-credit-hub/internal/scoring"
	"smart-credit-hub/internal/storage"
)

func setupActionsTestRouter(t *testing.T) (http.Handler, auth.Service, storage.ApplicationRepository, *handlers.ClientActionsHandler, *handlers.BankHandler) {
	t.Helper()

	tempUploadDir := filepath.Join(os.TempDir(), "test_smart_credit_hub_uploads")
	_ = os.MkdirAll(tempUploadDir, 0755)
	t.Cleanup(func() {
		_ = os.RemoveAll(tempUploadDir)
	})

	cfg := &config.Config{
		Port:               ":8080",
		JWTSecret:          "test-secret-key-actions-12345",
		JWTExpirationHours: 1,
		CasesFilePath:      config.FindCasesFilePath(""),
		CORSAllowedOrigins: []string{"*"},
		UploadDir:          tempUploadDir,
		Environment:        "test",
	}

	mockSvc, err := mock.NewService(cfg.CasesFilePath)
	if err != nil {
		t.Fatalf("ошибка mock.Service: %v", err)
	}

	userRepo := storage.NewInMemoryUserRepository(auth.GetDemoUsers()...)
	tokenManager := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTExpirationHours)
	authSvc := auth.NewService(tokenManager, userRepo)

	appRepo := storage.NewInMemoryApplicationRepository()

	scoringModel, err := scoring.LoadModel("")
	if err != nil {
		t.Fatalf("ошибка scoringModel: %v", err)
	}
	scoringEngine := scoring.NewEngine(scoringModel)

	pipeline := app.NewPipelineService(appRepo, mockSvc, scoringEngine, nil)
	pipeline.SetStepDelay(0)

	clientActionsH := handlers.NewClientActionsHandler(appRepo, tempUploadDir)
	clientActionsH.SetDisburseDelay(0) // моментальное зачисление для тестов

	bankH := handlers.NewBankHandler(appRepo, scoringModel)

	router := api.NewRouter(api.Dependencies{
		Config:               cfg,
		AuthService:          authSvc,
		MockService:          mockSvc,
		AppRepo:              appRepo,
		Pipeline:             pipeline,
		ScoringModel:         scoringModel,
		ClientActionsHandler: clientActionsH,
		BankHandler:          bankH,
		Pinger:               nil,
	})

	return router, authSvc, appRepo, clientActionsH, bankH
}

// 1. Тест принятия контр-оффера клиентом (COUNTER_OFFERED -> APPROVED)
func TestClientActions_AcceptCounterOffer(t *testing.T) {
	router, authSvc, appRepo, _, _ := setupActionsTestRouter(t)

	loginResp, _ := authSvc.Authenticate(context.Background(), "client_alfa", "demo123")
	userID := "00000000-0000-0000-0000-000000000001"
	appID := "app_counter_offer_1"

	// Создаем заявку в статусе COUNTER_OFFERED
	testApp := &domain.Application{
		ID:                  appID,
		UserID:              &userID,
		INN:                 "7701000001",
		Status:              domain.StatusCounterOffered,
		ProductType:         domain.ProductWorkingCapital,
		RequestedAmount:     5000000,
		RequestedTermMonths: 24,
		CounterOffer: &domain.CounterOffer{
			Amount:     3250000,
			TermMonths: 24,
			Rate:       0.205,
			PTI:        0.2493,
			Reason:     "Тестовый контр-оффер",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = appRepo.Create(context.Background(), nil, testApp)
	// Переводим в COUNTER_OFFERED в репозитории
	_ = appRepo.UpdateStatus(context.Background(), nil, appID, domain.StatusSubmitted, domain.StatusCounterOffered, "SYSTEM", nil, &domain.ApplicationUpdateFields{
		CounterOffer: testApp.CounterOffer,
	})

	// Отправляем POST accept
	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications/"+appID+"/counter-offer/accept", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидался статус 200 OK, получен: %d, тело: %s", rec.Code, rec.Body.String())
	}

	var updatedApp domain.Application
	_ = json.Unmarshal(rec.Body.Bytes(), &updatedApp)

	if updatedApp.Status != domain.StatusApproved {
		t.Errorf("ожидался статус APPROVED, получен: %s", updatedApp.Status)
	}
	if updatedApp.ApprovedAmount == nil || *updatedApp.ApprovedAmount != 3250000 {
		t.Errorf("ожидалась сумма 3250000, получено: %v", updatedApp.ApprovedAmount)
	}
	if updatedApp.ApprovedTermMonths == nil || *updatedApp.ApprovedTermMonths != 24 {
		t.Errorf("ожидался срок 24, получено: %v", updatedApp.ApprovedTermMonths)
	}
	if updatedApp.InterestRate == nil || *updatedApp.InterestRate != 0.205 {
		t.Errorf("ожидалась ставка 0.205, получено: %v", updatedApp.InterestRate)
	}

	// Повторная попытка принять оффер из APPROVED -> 409 Conflict
	recRepeat := httptest.NewRecorder()
	router.ServeHTTP(recRepeat, req)
	if recRepeat.Code != http.StatusConflict {
		t.Errorf("ожидался 409 Conflict при повторном принятии, получен: %d", recRepeat.Code)
	}
}

// 2. Тест отклонения контр-оффера клиентом (COUNTER_OFFERED -> CLIENT_DECLINED)
func TestClientActions_RejectCounterOffer(t *testing.T) {
	router, authSvc, appRepo, _, _ := setupActionsTestRouter(t)

	loginResp, _ := authSvc.Authenticate(context.Background(), "client_alfa", "demo123")
	userID := "00000000-0000-0000-0000-000000000001"
	appID := "app_counter_offer_2"

	testApp := &domain.Application{
		ID:                  appID,
		UserID:              &userID,
		INN:                 "7701000001",
		Status:              domain.StatusSubmitted,
		ProductType:         domain.ProductWorkingCapital,
		RequestedAmount:     5000000,
		RequestedTermMonths: 24,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	_ = appRepo.Create(context.Background(), nil, testApp)
	_ = appRepo.UpdateStatus(context.Background(), nil, appID, domain.StatusSubmitted, domain.StatusCounterOffered, "SYSTEM", nil, &domain.ApplicationUpdateFields{
		CounterOffer: &domain.CounterOffer{Amount: 3250000, TermMonths: 24, Rate: 0.205},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications/"+appID+"/counter-offer/reject", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидался статус 200 OK, получен: %d", rec.Code)
	}

	var updatedApp domain.Application
	_ = json.Unmarshal(rec.Body.Bytes(), &updatedApp)

	if updatedApp.Status != domain.StatusClientDeclined {
		t.Errorf("ожидался статус CLIENT_DECLINED, получен: %s", updatedApp.Status)
	}
}

// 3. Тест подписания и фонового зачисления средств (AUTO_APPROVED | APPROVED -> SIGNED -> DISBURSED)
func TestClientActions_SignAndDisburse(t *testing.T) {
	router, authSvc, appRepo, _, _ := setupActionsTestRouter(t)

	loginResp, _ := authSvc.Authenticate(context.Background(), "client_alfa", "demo123")
	userID := "00000000-0000-0000-0000-000000000001"
	appID := "app_sign_1"

	testApp := &domain.Application{
		ID:                  appID,
		UserID:              &userID,
		INN:                 "7701000001",
		Status:              domain.StatusSubmitted,
		ProductType:         domain.ProductWorkingCapital,
		RequestedAmount:     3000000,
		RequestedTermMonths: 12,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	_ = appRepo.Create(context.Background(), nil, testApp)
	_ = appRepo.UpdateStatus(context.Background(), nil, appID, domain.StatusSubmitted, domain.StatusAutoApproved, "SYSTEM", nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications/"+appID+"/sign", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидался статус 200 OK, получен: %d", rec.Code)
	}

	var signedApp domain.Application
	_ = json.Unmarshal(rec.Body.Bytes(), &signedApp)

	if signedApp.Status != domain.StatusSigned {
		t.Errorf("сразу после подписания ожидался статус SIGNED, получен: %s", signedApp.Status)
	}

	// Даем фоновой горутине время выполниться (с delay=0)
	time.Sleep(50 * time.Millisecond)

	finalApp, err := appRepo.GetByID(context.Background(), appID)
	if err != nil {
		t.Fatalf("ошибка чтения заявки: %v", err)
	}

	if finalApp.Status != domain.StatusDisbursed {
		t.Errorf("после выдачи ожидался статус DISBURSED, получен: %s", finalApp.Status)
	}
}

// 4. Тест решений андеррайтера (APPROVE, REJECT, REQUEST_DOCUMENTS, COUNTER_OFFER)
func TestBankHandler_Decisions(t *testing.T) {
	router, authSvc, appRepo, _, _ := setupActionsTestRouter(t)

	underwriterLogin, _ := authSvc.Authenticate(context.Background(), "underwriter", "demo123")

	setupManualReviewApp := func(id string) {
		userID := "00000000-0000-0000-0000-000000000003"
		testApp := &domain.Application{
			ID:                  id,
			UserID:              &userID,
			INN:                 "7701000003",
			Status:              domain.StatusSubmitted,
			ProductType:         domain.ProductWorkingCapital,
			RequestedAmount:     2500000,
			RequestedTermMonths: 12,
			CompanySnapshot: &domain.CompanySnapshot{
				INN:           "7701000003",
				AnnualRevenue: 12000000,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		_ = appRepo.Create(context.Background(), nil, testApp)
		_ = appRepo.UpdateStatus(context.Background(), nil, id, domain.StatusSubmitted, domain.StatusManualReview, "SYSTEM", nil, nil)
	}

	// 1. APPROVE
	t.Run("Decision_APPROVE", func(t *testing.T) {
		appID := "app_decision_approve"
		setupManualReviewApp(appID)

		reqBody := domain.UnderwriterDecisionRequest{
			Action:  domain.ActionApprove,
			Comment: domain.StringPtr("Бизнес проверен, одобрено"),
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/bank/applications/"+appID+"/decision", bytes.NewReader(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+underwriterLogin.Token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("ожидался 200 OK, получен: %d, тело: %s", rec.Code, rec.Body.String())
		}

		var appResp domain.Application
		_ = json.Unmarshal(rec.Body.Bytes(), &appResp)
		if appResp.Status != domain.StatusApproved {
			t.Errorf("ожидался статус APPROVED, получен: %s", appResp.Status)
		}
	})

	// 2. REJECT (комментарий обязателен)
	t.Run("Decision_REJECT", func(t *testing.T) {
		appID := "app_decision_reject"
		setupManualReviewApp(appID)

		reqBody := domain.UnderwriterDecisionRequest{
			Action:  domain.ActionReject,
			Comment: domain.StringPtr("Высокая текущая кредитная нагрузка"),
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/bank/applications/"+appID+"/decision", bytes.NewReader(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+underwriterLogin.Token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("ожидался 200 OK, получен: %d", rec.Code)
		}

		var appResp domain.Application
		_ = json.Unmarshal(rec.Body.Bytes(), &appResp)
		if appResp.Status != domain.StatusRejected {
			t.Errorf("ожидался статус REJECTED, получен: %s", appResp.Status)
		}
		if appResp.RejectReasonClient == nil || *appResp.RejectReasonClient != "Высокая текущая кредитная нагрузка" {
			t.Errorf("не сохранена причина отказа: %v", appResp.RejectReasonClient)
		}
	})

	// 3. REQUEST_DOCUMENTS
	t.Run("Decision_REQUEST_DOCUMENTS", func(t *testing.T) {
		appID := "app_decision_req_docs"
		setupManualReviewApp(appID)

		reqBody := domain.UnderwriterDecisionRequest{
			Action:  domain.ActionRequestDocuments,
			Comment: domain.StringPtr("Предоставьте бухгалтерский баланс за последний квартал"),
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/bank/applications/"+appID+"/decision", bytes.NewReader(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+underwriterLogin.Token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("ожидался 200 OK, получен: %d", rec.Code)
		}

		var appResp domain.Application
		_ = json.Unmarshal(rec.Body.Bytes(), &appResp)
		if appResp.Status != domain.StatusWaitingDocuments {
			t.Errorf("ожидался статус WAITING_DOCUMENTS, получен: %s", appResp.Status)
		}
		if appResp.UnderwriterComment == nil || *appResp.UnderwriterComment != "Предоставьте бухгалтерский баланс за последний квартал" {
			t.Errorf("не сохранен комментарий андеррайтера: %v", appResp.UnderwriterComment)
		}
	})

	// 4. COUNTER_OFFER
	t.Run("Decision_COUNTER_OFFER", func(t *testing.T) {
		appID := "app_decision_counter_offer"
		setupManualReviewApp(appID)

		amt := 1500000.0
		term := 12
		reqBody := domain.UnderwriterDecisionRequest{
			Action:                 domain.ActionCounterOffer,
			CounterOfferAmount:     &amt,
			CounterOfferTermMonths: &term,
			Comment:                domain.StringPtr("Банк согласовал лимит 1.5 млн"),
		}
		bodyBytes, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/bank/applications/"+appID+"/decision", bytes.NewReader(bodyBytes))
		req.Header.Set("Authorization", "Bearer "+underwriterLogin.Token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("ожидался 200 OK, получен: %d", rec.Code)
		}

		var appResp domain.Application
		_ = json.Unmarshal(rec.Body.Bytes(), &appResp)
		if appResp.Status != domain.StatusCounterOffered {
			t.Errorf("ожидался статус COUNTER_OFFERED, получен: %s", appResp.Status)
		}
		if appResp.CounterOffer == nil || appResp.CounterOffer.Amount != 1500000 {
			t.Errorf("контр-оффер не сохранен корректно: %v", appResp.CounterOffer)
		}
	})
}

// 5. Тест загрузки документов (multipart/form-data) и возврата в MANUAL_REVIEW
func TestClientActions_UploadDocument(t *testing.T) {
	router, authSvc, appRepo, _, _ := setupActionsTestRouter(t)

	loginResp, _ := authSvc.Authenticate(context.Background(), "client_alfa", "demo123")
	userID := "00000000-0000-0000-0000-000000000001"
	appID := "app_upload_doc_1"

	// Создаем заявку в статусе WAITING_DOCUMENTS
	testApp := &domain.Application{
		ID:                  appID,
		UserID:              &userID,
		INN:                 "7701000001",
		Status:              domain.StatusSubmitted,
		ProductType:         domain.ProductWorkingCapital,
		RequestedAmount:     3000000,
		RequestedTermMonths: 12,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	_ = appRepo.Create(context.Background(), nil, testApp)
	_ = appRepo.UpdateStatus(context.Background(), nil, appID, domain.StatusSubmitted, domain.StatusWaitingDocuments, "UNDERWRITER", nil, nil)

	// Формируем multipart/form-data запрос
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	_ = writer.WriteField("document_type", "FINANCIAL_STATEMENT")
	part, err := writer.CreateFormFile("file", "balance_2025.pdf")
	if err != nil {
		t.Fatalf("ошибка создания form file: %v", err)
	}
	_, _ = io.WriteString(part, "%PDF-1.4 mock pdf content for testing document upload")
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications/"+appID+"/documents", body)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("ожидался статус 201 Created, получен: %d, тело: %s", rec.Code, rec.Body.String())
	}

	var docResp domain.Document
	_ = json.Unmarshal(rec.Body.Bytes(), &docResp)

	if docResp.ID == "" {
		t.Error("ID документа не должен быть пустым")
	}
	if docResp.Name != "balance_2025.pdf" {
		t.Errorf("ожидалось имя balance_2025.pdf, получено: %s", docResp.Name)
	}
	if docResp.DocumentType != domain.DocFinancialStatement {
		t.Errorf("ожидался тип FINANCIAL_STATEMENT, получен: %s", docResp.DocumentType)
	}

	// Проверяем, что заявка автоматически вернулась в MANUAL_REVIEW
	updatedApp, err := appRepo.GetByID(context.Background(), appID)
	if err != nil {
		t.Fatalf("ошибка чтения заявки: %v", err)
	}

	if updatedApp.Status != domain.StatusManualReview {
		t.Errorf("после загрузки документа статус должен стать MANUAL_REVIEW, получен: %s", updatedApp.Status)
	}
	if len(updatedApp.Documents) != 1 {
		t.Errorf("ожидался 1 прикрепленный документ, получено: %d", len(updatedApp.Documents))
	}
}

// 6. Тест получения очереди заявок андеррайтером с фильтрами
func TestBankHandler_ListWithFilters(t *testing.T) {
	router, authSvc, appRepo, _, _ := setupActionsTestRouter(t)

	underwriterLogin, _ := authSvc.Authenticate(context.Background(), "underwriter", "demo123")
	clientLogin, _ := authSvc.Authenticate(context.Background(), "client_alfa", "demo123")

	// 1. Клиент пытается открыть /bank/applications -> 403 Forbidden
	reqClient := httptest.NewRequest(http.MethodGet, "/api/v1/bank/applications", nil)
	reqClient.Header.Set("Authorization", "Bearer "+clientLogin.Token)
	recClient := httptest.NewRecorder()
	router.ServeHTTP(recClient, reqClient)
	if recClient.Code != http.StatusForbidden {
		t.Errorf("клиенту должен быть запрещен доступ в /bank (403), получен: %d", recClient.Code)
	}

	// 2. Добавляем 2 заявки: одну в MANUAL_REVIEW (GREY), одну в AUTO_APPROVED (GREEN)
	userID := "00000000-0000-0000-0000-000000000001"
	app1 := &domain.Application{
		ID:                  "app_bank_1",
		UserID:              &userID,
		INN:                 "7701000001",
		Status:              domain.StatusManualReview,
		ProductType:         domain.ProductWorkingCapital,
		RequestedAmount:     2500000,
		RequestedTermMonths: 12,
		ScoringResult:       &domain.ScoringResult{RiskZone: domain.RiskZoneGrey},
		CreatedAt:           time.Now().Add(-10 * time.Minute),
	}
	app2 := &domain.Application{
		ID:                  "app_bank_2",
		UserID:              &userID,
		INN:                 "7701000001",
		Status:              domain.StatusAutoApproved,
		ProductType:         domain.ProductWorkingCapital,
		RequestedAmount:     3000000,
		RequestedTermMonths: 12,
		ScoringResult:       &domain.ScoringResult{RiskZone: domain.RiskZoneGreen},
		CreatedAt:           time.Now(),
	}
	_ = appRepo.Create(context.Background(), nil, app1)
	_ = appRepo.Create(context.Background(), nil, app2)

	// 3. Андеррайтер запрашивает все заявки -> 200 OK
	reqAll := httptest.NewRequest(http.MethodGet, "/api/v1/bank/applications", nil)
	reqAll.Header.Set("Authorization", "Bearer "+underwriterLogin.Token)
	recAll := httptest.NewRecorder()
	router.ServeHTTP(recAll, reqAll)

	if recAll.Code != http.StatusOK {
		t.Fatalf("ожидался 200 OK для андеррайтера, получен: %d", recAll.Code)
	}

	var allApps []*domain.Application
	_ = json.Unmarshal(recAll.Body.Bytes(), &allApps)
	if len(allApps) < 2 {
		t.Errorf("ожидалось не менее 2 заявок, получено: %d", len(allApps))
	}

	// 4. Фильтр по status=MANUAL_REVIEW
	reqFiltered := httptest.NewRequest(http.MethodGet, "/api/v1/bank/applications?status=MANUAL_REVIEW", nil)
	reqFiltered.Header.Set("Authorization", "Bearer "+underwriterLogin.Token)
	recFiltered := httptest.NewRecorder()
	router.ServeHTTP(recFiltered, reqFiltered)

	var filteredApps []*domain.Application
	_ = json.Unmarshal(recFiltered.Body.Bytes(), &filteredApps)
	for _, a := range filteredApps {
		if a.Status != domain.StatusManualReview {
			t.Errorf("ожидался статус только MANUAL_REVIEW, получен: %s", a.Status)
		}
	}
}
