package middleware

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
)

type contextKey string

const (
	// RequestIDHeader - HTTP-заголовок сквозного идентификатора запроса.
	RequestIDHeader = "X-Request-ID"
	// RequestIDKey - ключ контекста для хранения идентификатора запроса.
	RequestIDKey contextKey = "request_id"
)

// RequestID middleware проверяет наличие X-Request-ID в заголовках.
// Если заголовок отсутствует, генерирует UUIDv4 и прокидывает его в контекст и ответ.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get(RequestIDHeader)
		if reqID == "" {
			reqID = generateUUID()
		}

		w.Header().Set(RequestIDHeader, reqID)
		ctx := context.WithValue(r.Context(), RequestIDKey, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID извлекает идентификатор запроса из контекста.
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok {
		return reqID
	}
	return ""
}

// generateUUID генерирует валидный UUID v4 без сторонних библиотек.
func generateUUID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])

	// Установка бит версии 4 и варианта RFC4122
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4],
		uuid[4:6],
		uuid[6:8],
		uuid[8:10],
		uuid[10:16],
	)
}
