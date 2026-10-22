package domain

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Коды ошибок в соответствии со спецификацией docs/openapi.yaml
const (
	ErrCodeUnauthorized  = "UNAUTHORIZED"
	ErrCodeForbidden     = "FORBIDDEN"
	ErrCodeNotFound      = "NOT_FOUND"
	ErrCodeBadRequest    = "BAD_REQUEST"
	ErrCodeConflictState = "CONFLICT_STATE"
	ErrCodeInternalError = "INTERNAL_ERROR"
)

// ErrorResponse представляет контракт ошибки согласно OpenAPI спецификации (ErrorResponse schema).
type ErrorResponse struct {
	Code    string  `json:"code"`
	Message string  `json:"message"`
	Details *string `json:"details"`
}

// AppError описывает структурированную доменную ошибку с HTTP-статусом.
type AppError struct {
	HTTPStatus int
	Code       string
	Message    string
	Details    *string
	Err        error
}

// Error реализует интерфейс error.
func (e *AppError) Error() string {
	if e.Details != nil && *e.Details != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, *e.Details)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap возвращает исходную ошибку, если она была обернута.
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError создает новую структурированную ошибку приложения.
func NewAppError(status int, code, message string, details *string) *AppError {
	return &AppError{
		HTTPStatus: status,
		Code:       code,
		Message:    message,
		Details:    details,
	}
}

// Predefined ошибки для часто используемых сценариев
func ErrInvalidCredentials() *AppError {
	return NewAppError(http.StatusUnauthorized, ErrCodeUnauthorized, "Неверный логин или пароль", nil)
}

func ErrUnauthorized(details *string) *AppError {
	return NewAppError(http.StatusUnauthorized, ErrCodeUnauthorized, "Требуется авторизация", details)
}

func ErrForbidden(details *string) *AppError {
	return NewAppError(http.StatusForbidden, ErrCodeForbidden, "Доступ запрещен для данной роли", details)
}

func ErrNotFound(message string, details *string) *AppError {
	return NewAppError(http.StatusNotFound, ErrCodeNotFound, message, details)
}

func ErrBadRequest(message string, details *string) *AppError {
	return NewAppError(http.StatusBadRequest, ErrCodeBadRequest, message, details)
}

func ErrInternal(details *string) *AppError {
	return NewAppError(http.StatusInternalServerError, ErrCodeInternalError, "Внутренняя ошибка сервера", details)
}

// RespondWithError отправляет структурированный ErrorResponse в http.ResponseWriter.
func RespondWithError(w http.ResponseWriter, status int, code, message string, details *string) {
	resp := ErrorResponse{
		Code:    code,
		Message: message,
		Details: details,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

// RespondWithAppError отправляет AppError в http.ResponseWriter.
func RespondWithAppError(w http.ResponseWriter, err error) {
	if appErr, ok := err.(*AppError); ok {
		RespondWithError(w, appErr.HTTPStatus, appErr.Code, appErr.Message, appErr.Details)
		return
	}

	// Неизвестная ошибка мапится в 500 INTERNAL_ERROR
	RespondWithError(w, http.StatusInternalServerError, ErrCodeInternalError, "Внутренняя ошибка сервера", nil)
}

// RespondWithJSON сериализует payload в JSON и отправляет с указанным HTTP-статусом.
func RespondWithJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// StringPtr хелпер для быстрого создания указателя на строку.
func StringPtr(s string) *string {
	return &s
}
