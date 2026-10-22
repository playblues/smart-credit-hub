package middleware

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// responseWriterWrapper обертка над http.ResponseWriter для захвата HTTP-статуса и размера ответа.
type responseWriterWrapper struct {
	http.ResponseWriter
	status      int
	bytesWr     int64
	wroteHeader bool
}

func newResponseWriterWrapper(w http.ResponseWriter) *responseWriterWrapper {
	return &responseWriterWrapper{
		ResponseWriter: w,
		status:         http.StatusOK,
	}
}

func (rw *responseWriterWrapper) WriteHeader(status int) {
	if !rw.wroteHeader {
		rw.status = status
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(status)
	}
}

func (rw *responseWriterWrapper) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWr += int64(n)
	return n, err
}

// Hijack реализует http.Hijacker при необходимости (например, для WebSocket).
func (rw *responseWriterWrapper) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, errors.New("http.Hijacker не поддерживается внутренним ResponseWriter")
}

// Logger middleware логирует все входящие HTTP-запросы через log/slog.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := newResponseWriterWrapper(w)
		reqID := GetRequestID(r.Context())

		next.ServeHTTP(ww, r)

		duration := time.Since(start)

		slog.InfoContext(r.Context(), "HTTP request completed",
			slog.String("request_id", reqID),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", ww.status),
			slog.Int64("bytes", ww.bytesWr),
			slog.Int64("duration_ms", duration.Milliseconds()),
			slog.String("client_ip", r.RemoteAddr),
			slog.String("user_agent", r.UserAgent()),
		)
	})
}
