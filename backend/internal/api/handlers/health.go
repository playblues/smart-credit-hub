package handlers

import (
	"context"
	"net/http"
	"time"

	"smart-credit-hub/internal/domain"
)

// Pinger интерфейс для проверки жизнеспособности зависимостей (например, пула соединений БД).
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler обрабатывает запросы проверки работоспособности сервиса.
type HealthHandler struct {
	pinger Pinger
}

// NewHealthHandler создает новый экземпляр HealthHandler с опциональным Pinger.
func NewHealthHandler(pinger Pinger) *HealthHandler {
	return &HealthHandler{
		pinger: pinger,
	}
}

// Check обрабатывает GET /healthz (liveness/readiness probe).
// При наличии Pinger выполняет ping БД. Если БД недоступна — отдает 503 Service Unavailable.
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	dbStatus := "disabled"

	if h.pinger != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := h.pinger.Ping(ctx); err != nil {
			domain.RespondWithError(
				w,
				http.StatusServiceUnavailable,
				"SERVICE_UNAVAILABLE",
				"База данных недоступна",
				domain.StringPtr(err.Error()),
			)
			return
		}
		dbStatus = "connected"
	}

	domain.RespondWithJSON(w, http.StatusOK, domain.HealthResponse{
		Status:   "ok",
		Database: dbStatus,
	})
}
