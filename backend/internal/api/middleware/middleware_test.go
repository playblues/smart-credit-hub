package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"smart-credit-hub/internal/api/middleware"
)

func TestRequestID(t *testing.T) {
	// 1. Автоматическая генерация X-Request-ID
	handler := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := middleware.GetRequestID(r.Context())
		if reqID == "" {
			t.Error("ожидался непустой request_id в контексте")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	respID := rec.Header().Get(middleware.RequestIDHeader)
	if respID == "" {
		t.Error("ожидался заголовок X-Request-ID в ответе")
	}

	// 2. Проброс существующего X-Request-ID
	customID := "custom-req-id-12345"
	reqWithCustom := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqWithCustom.Header.Set(middleware.RequestIDHeader, customID)
	recCustom := httptest.NewRecorder()
	handler.ServeHTTP(recCustom, reqWithCustom)

	if recCustom.Header().Get(middleware.RequestIDHeader) != customID {
		t.Errorf("ожидался проброс custom X-Request-ID %s, получен: %s", customID, recCustom.Header().Get(middleware.RequestIDHeader))
	}
}

func TestRecoverer(t *testing.T) {
	panicHandler := middleware.Recoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("непредвиденная критическая ошибка")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Recoverer middleware должен был перехватить панику, но она всплыла: %v", r)
		}
	}()

	panicHandler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("ожидался HTTP статус 500 Internal Server Error, получен: %d", rec.Code)
	}
}

func TestCORS(t *testing.T) {
	corsHandler := middleware.CORS([]string{"http://localhost:3000"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()

	corsHandler.ServeHTTP(rec, req)

	allowOrigin := rec.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin != "http://localhost:3000" {
		t.Errorf("ожидался Access-Control-Allow-Origin = http://localhost:3000, получен: %s", allowOrigin)
	}
}
