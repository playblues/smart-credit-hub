package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"smart-credit-hub/internal/api"
	"smart-credit-hub/internal/api/middleware"
	"smart-credit-hub/internal/auth"
	"smart-credit-hub/internal/config"
	"smart-credit-hub/internal/domain"
	"smart-credit-hub/internal/mock"
	"smart-credit-hub/internal/storage"
)

// mockPinger мок для проверки логики healthcheck
type mockPinger struct {
	err error
}

func (m *mockPinger) Ping(ctx context.Context) error {
	return m.err
}

// setupTestRouter инициализирует тестовое окружение и возвращает настроенный HTTP-роутер и сервисы.
func setupTestRouter(t *testing.T) (http.Handler, auth.Service, mock.Service) {
	t.Helper()

	cfg := &config.Config{
		Port:               ":8080",
		JWTSecret:          "test-secret-key-12345",
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

	router := api.NewRouter(api.Dependencies{
		Config:      cfg,
		AuthService: authSvc,
		MockService: mockSvc,
		Pinger:      nil,
	})

	return router, authSvc, mockSvc
}

// 1. Тест успешной авторизации клиента
func TestLogin_Success_Client(t *testing.T) {
	router, _, _ := setupTestRouter(t)

	reqBody := domain.LoginRequest{
		Login:    "client_alfa",
		Password: "demo123",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидался статус 200 OK, получен: %d, тело: %s", rec.Code, rec.Body.String())
	}

	var resp domain.LoginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("ошибка разбора LoginResponse: %v", err)
	}

	if resp.Token == "" {
		t.Errorf("ожидался непустой JWT-токен")
	}
	if resp.Role != domain.RoleClient {
		t.Errorf("ожидалась роль CLIENT, получена: %s", resp.Role)
	}
	if resp.INN == nil || *resp.INN != "7701000001" {
		t.Errorf("ожидался ИНН 7701000001, получен: %v", resp.INN)
	}
	if resp.DisplayName != "ООО «Альфа Логистика»" {
		t.Errorf("ожидалось имя 'ООО «Альфа Логистика»', получено: %s", resp.DisplayName)
	}
}

// 2. Тест успешной авторизации андеррайтера
func TestLogin_Success_Underwriter(t *testing.T) {
	router, _, _ := setupTestRouter(t)

	reqBody := domain.LoginRequest{
		Login:    "underwriter",
		Password: "demo123",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидался статус 200 OK, получен: %d, тело: %s", rec.Code, rec.Body.String())
	}

	var resp domain.LoginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("ошибка разбора LoginResponse: %v", err)
	}

	if resp.Token == "" {
		t.Errorf("ожидался непустой JWT-токен")
	}
	if resp.Role != domain.RoleUnderwriter {
		t.Errorf("ожидалась роль UNDERWRITER, получена: %s", resp.Role)
	}
	if resp.INN != nil {
		t.Errorf("у андеррайтера ИНН должен быть nil, получено: %v", resp.INN)
	}
	if resp.DisplayName != "Главный Андеррайтер" {
		t.Errorf("ожидалось имя 'Главный Андеррайтер', получено: %s", resp.DisplayName)
	}
}

// 3. Тест авторизации с неверным паролем (должен вернуть 401 UNAUTHORIZED)
func TestLogin_InvalidPassword(t *testing.T) {
	router, _, _ := setupTestRouter(t)

	reqBody := domain.LoginRequest{
		Login:    "client_alfa",
		Password: "wrong-password",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ожидался статус 401 Unauthorized, получен: %d", rec.Code)
	}

	var errResp domain.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("ошибка разбора ErrorResponse: %v", err)
	}

	if errResp.Code != domain.ErrCodeUnauthorized {
		t.Errorf("ожидался код ошибки UNAUTHORIZED, получен: %s", errResp.Code)
	}
}

// 4. Тест получения мок-данных ФНС по существующему ИНН 7701000001
func TestMockFNS_Success(t *testing.T) {
	router, _, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mock/fns/7701000001", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидался статус 200 OK, получен: %d, тело: %s", rec.Code, rec.Body.String())
	}

	var comp domain.FNSCompany
	if err := json.Unmarshal(rec.Body.Bytes(), &comp); err != nil {
		t.Fatalf("ошибка разбора FNSCompany: %v", err)
	}

	if comp.INN != "7701000001" {
		t.Errorf("ожидался ИНН 7701000001, получен: %s", comp.INN)
	}
	if comp.Name != "ООО «Альфа Логистика»" {
		t.Errorf("ожидалось имя 'ООО «Альфа Логистика»', получено: %s", comp.Name)
	}
	if comp.AnnualRevenue != 50000000 {
		t.Errorf("ожидалась выручка 50000000, получено: %f", comp.AnnualRevenue)
	}
	if comp.BusinessAgeMonths != 48 {
		t.Errorf("ожидался возраст бизнеса 48 месяцев, получено: %d", comp.BusinessAgeMonths)
	}
	if comp.IsLiquidating {
		t.Errorf("компания не должна быть в стадии ликвидации")
	}
}

// 5. Тест получения мок-данных ФНС по несуществующему ИНН (должен вернуть 404 NOT_FOUND)
func TestMockFNS_NotFound(t *testing.T) {
	router, _, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mock/fns/9999999999", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("ожидался статус 404 Not Found, получен: %d", rec.Code)
	}

	var errResp domain.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("ошибка разбора ErrorResponse: %v", err)
	}

	if errResp.Code != domain.ErrCodeNotFound {
		t.Errorf("ожидался код ошибки NOT_FOUND, получен: %s", errResp.Code)
	}
}

// 6. Тест получения мок-данных БКИ по существующему ИНН 7701000001
func TestMockBKI_Success(t *testing.T) {
	router, _, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mock/bki/7701000001", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидался статус 200 OK, получен: %d, тело: %s", rec.Code, rec.Body.String())
	}

	var bki domain.BKICreditHistory
	if err := json.Unmarshal(rec.Body.Bytes(), &bki); err != nil {
		t.Fatalf("ошибка разбора BKICreditHistory: %v", err)
	}

	if bki.INN != "7701000001" {
		t.Errorf("ожидался ИНН 7701000001, получен: %s", bki.INN)
	}
	if bki.ActiveLoansCount != 1 {
		t.Errorf("ожидался active_loans_count=1, получено: %d", bki.ActiveLoansCount)
	}
	if bki.MaxOverdueDays != 0 {
		t.Errorf("ожидался max_overdue_days=0, получено: %d", bki.MaxOverdueDays)
	}
	if bki.HadPastDelinquency {
		t.Errorf("ожидалось had_past_delinquency=false, получено: true")
	}
}

// 7. Тест получения мок-данных БКИ по несуществующему ИНН (404 NOT_FOUND)
func TestMockBKI_NotFound(t *testing.T) {
	router, _, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mock/bki/9999999999", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("ожидался статус 404 Not Found, получен: %d", rec.Code)
	}

	var errResp domain.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("ошибка разбора ErrorResponse: %v", err)
	}

	if errResp.Code != domain.ErrCodeNotFound {
		t.Errorf("ожидался код ошибки NOT_FOUND, получен: %s", errResp.Code)
	}
}

// 8. Тест эндпоинта проверки жизнеспособности GET /healthz
func TestHealthCheck(t *testing.T) {
	router, _, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидался статус 200 OK, получен: %d", rec.Code)
	}

	var resp domain.HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("ошибка разбора HealthResponse: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("ожидался статус 'ok', получен: %s", resp.Status)
	}
}

// 9. Тест проверки доступности БД в healthcheck (успешный ping -> 200 OK)
func TestHealthCheck_Database_Success(t *testing.T) {
	cfg := &config.Config{Port: ":8080", CORSAllowedOrigins: []string{"*"}}
	mockSvc, _ := mock.NewService("")
	authSvc := auth.NewService(auth.NewTokenManager("sec", 1), storage.NewInMemoryUserRepository())

	router := api.NewRouter(api.Dependencies{
		Config:      cfg,
		AuthService: authSvc,
		MockService: mockSvc,
		Pinger:      &mockPinger{err: nil},
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидался 200 OK, получен: %d", rec.Code)
	}

	var resp domain.HealthResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Database != "connected" {
		t.Errorf("ожидалось database: connected, получено: %s", resp.Database)
	}
}

// 10. Тест проверки недоступности БД в healthcheck (ошибка ping -> 503 Service Unavailable)
func TestHealthCheck_Database_Failure(t *testing.T) {
	cfg := &config.Config{Port: ":8080", CORSAllowedOrigins: []string{"*"}}
	mockSvc, _ := mock.NewService("")
	authSvc := auth.NewService(auth.NewTokenManager("sec", 1), storage.NewInMemoryUserRepository())

	router := api.NewRouter(api.Dependencies{
		Config:      cfg,
		AuthService: authSvc,
		MockService: mockSvc,
		Pinger:      &mockPinger{err: errors.New("connection refused")},
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("ожидался 503 Service Unavailable, получен: %d", rec.Code)
	}

	var errResp domain.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("ошибка разбора ErrorResponse: %v", err)
	}

	if errResp.Code != "SERVICE_UNAVAILABLE" {
		t.Errorf("ожидался код ошибки SERVICE_UNAVAILABLE, получен: %s", errResp.Code)
	}
}

// 11. Тест проверки Middleware авторизации и ролевого доступа (RequireRole)
func TestMiddleware_RequireRole(t *testing.T) {
	_, authSvc, _ := setupTestRouter(t)

	// Создаем тестовый защищенный хэндлер
	protectedHandler := middleware.Auth(authSvc)(
		middleware.RequireRole(domain.RoleUnderwriter)(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"access": "granted"}`))
			}),
		),
	)

	// 1. Запрос без токена -> 401
	reqNoToken := httptest.NewRequest(http.MethodGet, "/bank/applications", nil)
	recNoToken := httptest.NewRecorder()
	protectedHandler.ServeHTTP(recNoToken, reqNoToken)
	if recNoToken.Code != http.StatusUnauthorized {
		t.Errorf("без токена ожидался 401, получен: %d", recNoToken.Code)
	}

	// 2. Запрос с токеном клиента -> 403 Forbidden (так как эндпоинт только для UNDERWRITER)
	clientLogin, err := authSvc.Authenticate(reqNoToken.Context(), "client_alfa", "demo123")
	if err != nil {
		t.Fatalf("ошибка логина клиента: %v", err)
	}

	reqClient := httptest.NewRequest(http.MethodGet, "/bank/applications", nil)
	reqClient.Header.Set("Authorization", "Bearer "+clientLogin.Token)
	recClient := httptest.NewRecorder()
	protectedHandler.ServeHTTP(recClient, reqClient)
	if recClient.Code != http.StatusForbidden {
		t.Errorf("клиенту в эндпоинт андеррайтера должен возвращаться 403, получен: %d", recClient.Code)
	}

	// 3. Запрос с токеном андеррайтера -> 200 OK
	underwriterLogin, err := authSvc.Authenticate(reqNoToken.Context(), "underwriter", "demo123")
	if err != nil {
		t.Fatalf("ошибка логина андеррайтера: %v", err)
	}

	reqUnderwriter := httptest.NewRequest(http.MethodGet, "/bank/applications", nil)
	reqUnderwriter.Header.Set("Authorization", "Bearer "+underwriterLogin.Token)
	recUnderwriter := httptest.NewRecorder()
	protectedHandler.ServeHTTP(recUnderwriter, reqUnderwriter)
	if recUnderwriter.Code != http.StatusOK {
		t.Errorf("андеррайтеру должен возвращаться 200, получен: %d", recUnderwriter.Code)
	}
}

// 12. Тест информационной визитки на GET /
func TestRoot_WelcomeInfo(t *testing.T) {
	router, _, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидался код 200 OK на GET /, получен: %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("ожидался Content-Type application/json; charset=utf-8, получен: %s", contentType)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("ошибка парсинга JSON: %v", err)
	}

	if resp["service"] != "Alfa SME Credit Pipeline API" {
		t.Errorf("неверное имя сервиса: %v", resp["service"])
	}
	if resp["version"] != "1.0.0" {
		t.Errorf("неверная версия: %v", resp["version"])
	}
	if resp["status"] != "operational" {
		t.Errorf("неверный статус: %v", resp["status"])
	}
	if resp["frontend_url"] != "http://localhost:3000" {
		t.Errorf("неверный frontend_url: %v", resp["frontend_url"])
	}
	if resp["healthcheck"] != "/healthz" {
		t.Errorf("неверный healthcheck: %v", resp["healthcheck"])
	}
	if resp["api_prefix"] != "/api/v1" {
		t.Errorf("неверный api_prefix: %v", resp["api_prefix"])
	}
}

// 13. Тест кастомного 404 обработчика для несуществующих маршрутов
func TestNotFound_CustomJSON(t *testing.T) {
	router, _, _ := setupTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/unknown-route-for-testing", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("ожидался код 404 Not Found, получен: %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("ожидался Content-Type application/json; charset=utf-8, получен: %s", contentType)
	}

	var errResp domain.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("ошибка парсинга ErrorResponse: %v", err)
	}

	if errResp.Code != domain.ErrCodeNotFound {
		t.Errorf("ожидался код ошибки NOT_FOUND, получен: %s", errResp.Code)
	}
	if errResp.Message != "Маршрут не найден" {
		t.Errorf("ожидалось сообщение 'Маршрут не найден', получено: %s", errResp.Message)
	}
}
