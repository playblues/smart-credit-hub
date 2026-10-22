package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"smart-credit-hub/internal/auth"
	"smart-credit-hub/internal/domain"
)

// AuthHandler инкапсулирует хэндлеры аутентификации.
type AuthHandler struct {
	authService auth.Service
}

// NewAuthHandler создает экземпляр AuthHandler.
func NewAuthHandler(authService auth.Service) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// Login обрабатывает POST /api/v1/auth/login.
// Сверяет логин/пароль и возвращает токен, роль, ИНН и имя пользователя.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Content-Type") != "" && !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		domain.RespondWithError(
			w,
			http.StatusBadRequest,
			domain.ErrCodeBadRequest,
			"Заголовок Content-Type должен быть application/json",
			nil,
		)
		return
	}

	var req domain.LoginRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		domain.RespondWithError(
			w,
			http.StatusBadRequest,
			domain.ErrCodeBadRequest,
			"Некорректный JSON в теле запроса",
			domain.StringPtr(err.Error()),
		)
		return
	}

	req.Login = strings.TrimSpace(req.Login)
	req.Password = strings.TrimSpace(req.Password)

	if req.Login == "" || req.Password == "" {
		domain.RespondWithError(
			w,
			http.StatusBadRequest,
			domain.ErrCodeBadRequest,
			"Поля login и password обязательны для заполнения",
			nil,
		)
		return
	}

	resp, err := h.authService.Authenticate(r.Context(), req.Login, req.Password)
	if err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	domain.RespondWithJSON(w, http.StatusOK, resp)
}
