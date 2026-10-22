package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"smart-credit-hub/internal/domain"
	"smart-credit-hub/internal/mock"
)

// MockHandler инкапсулирует эндпоинты мок-сервисов ФНС и БКИ.
type MockHandler struct {
	mockService mock.Service
}

// NewMockHandler создает новый экземпляр MockHandler.
func NewMockHandler(mockService mock.Service) *MockHandler {
	return &MockHandler{
		mockService: mockService,
	}
}

// GetFNS обрабатывает GET /api/v1/mock/fns/{inn}.
// Возвращает данные компании из ФНС «Прозрачный бизнес».
func (h *MockHandler) GetFNS(w http.ResponseWriter, r *http.Request) {
	inn := strings.TrimSpace(chi.URLParam(r, "inn"))
	if inn == "" {
		domain.RespondWithError(
			w,
			http.StatusBadRequest,
			domain.ErrCodeBadRequest,
			"Параметр ИНН обязателен",
			nil,
		)
		return
	}

	if len(inn) < 10 || len(inn) > 12 {
		domain.RespondWithError(
			w,
			http.StatusBadRequest,
			domain.ErrCodeBadRequest,
			"Длина ИНН должна составлять от 10 до 12 символов",
			nil,
		)
		return
	}

	company, err := h.mockService.GetFNSByINN(r.Context(), inn)
	if err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	domain.RespondWithJSON(w, http.StatusOK, company)
}

// GetBKI обрабатывает GET /api/v1/mock/bki/{inn}.
// Возвращает кредитную историю заемщика из БКИ.
func (h *MockHandler) GetBKI(w http.ResponseWriter, r *http.Request) {
	inn := strings.TrimSpace(chi.URLParam(r, "inn"))
	if inn == "" {
		domain.RespondWithError(
			w,
			http.StatusBadRequest,
			domain.ErrCodeBadRequest,
			"Параметр ИНН обязателен",
			nil,
		)
		return
	}

	if len(inn) < 10 || len(inn) > 12 {
		domain.RespondWithError(
			w,
			http.StatusBadRequest,
			domain.ErrCodeBadRequest,
			"Длина ИНН должна составлять от 10 до 12 символов",
			nil,
		)
		return
	}

	bki, err := h.mockService.GetBKIByINN(r.Context(), inn)
	if err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	domain.RespondWithJSON(w, http.StatusOK, bki)
}
