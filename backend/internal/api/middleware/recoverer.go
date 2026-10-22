package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"smart-credit-hub/internal/domain"
)

// Recoverer перехватывает возможные паники во время обработки запроса,
// логирует стек вызова через log/slog и возвращает клиенту HTTP 500 с ErrorResponse.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				reqID := GetRequestID(r.Context())
				stack := string(debug.Stack())

				slog.ErrorContext(r.Context(), "PANIC recovered in HTTP handler",
					slog.String("request_id", reqID),
					slog.Any("panic", rvr),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("stack", stack),
				)

				domain.RespondWithError(
					w,
					http.StatusInternalServerError,
					domain.ErrCodeInternalError,
					"Внутренняя ошибка сервера",
					nil,
				)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
