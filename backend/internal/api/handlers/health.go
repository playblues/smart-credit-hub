package handlers

import (
	"net/http"

	"smart-credit-hub/internal/domain"
)

// HealthHandler обрабатывает запросы проверки работоспособности сервиса.
type HealthHandler struct{}

// NewHealthHandler создает новый экземпляр HealthHandler.
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Check обрабатывает GET /healthz (liveness probe).
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	domain.RespondWithJSON(w, http.StatusOK, domain.HealthResponse{
		Status: "ok",
	})
}
