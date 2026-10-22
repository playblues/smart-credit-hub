package middleware

import (
	"context"
	"net/http"
	"strings"

	"smart-credit-hub/internal/auth"
	"smart-credit-hub/internal/domain"
)

const (
	// UserClaimsKey - ключ контекста для хранения JWT-claims текущего пользователя.
	UserClaimsKey contextKey = "user_claims"
)

// Auth middleware проверяет наличие и валидность Bearer JWT-токена в заголовке Authorization.
// При успешной проверке помещает данные пользователя (*auth.Claims) в контекст запроса.
func Auth(authService auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				domain.RespondWithError(
					w,
					http.StatusUnauthorized,
					domain.ErrCodeUnauthorized,
					"Требуется заголовок авторизации",
					nil,
				)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
				domain.RespondWithError(
					w,
					http.StatusUnauthorized,
					domain.ErrCodeUnauthorized,
					"Некорректный формат токена авторизации (ожидается 'Bearer <token>')",
					nil,
				)
				return
			}

			tokenString := strings.TrimSpace(parts[1])
			claims, err := authService.ValidateToken(tokenString)
			if err != nil {
				domain.RespondWithError(
					w,
					http.StatusUnauthorized,
					domain.ErrCodeUnauthorized,
					"Недействительный или истекший токен авторизации",
					domain.StringPtr(err.Error()),
				)
				return
			}

			ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole middleware ограничивает доступ к ресурсу только указанными ролями.
// При несоответствии роли возвращает HTTP 403 Forbidden со схемой ErrorResponse (code: "FORBIDDEN").
func RequireRole(allowedRoles ...domain.Role) func(http.Handler) http.Handler {
	allowedMap := make(map[domain.Role]struct{}, len(allowedRoles))
	for _, r := range allowedRoles {
		allowedMap[r] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetClaims(r.Context())
			if !ok || claims == nil {
				domain.RespondWithError(
					w,
					http.StatusUnauthorized,
					domain.ErrCodeUnauthorized,
					"Пользователь не аутентифицирован",
					nil,
				)
				return
			}

			if _, allowed := allowedMap[claims.Role]; !allowed {
				domain.RespondWithError(
					w,
					http.StatusForbidden,
					domain.ErrCodeForbidden,
					"Доступ запрещен для текущей роли пользователя",
					nil,
				)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetClaims извлекает *auth.Claims из контекста запроса.
func GetClaims(ctx context.Context) (*auth.Claims, bool) {
	if ctx == nil {
		return nil, false
	}
	claims, ok := ctx.Value(UserClaimsKey).(*auth.Claims)
	return claims, ok && claims != nil
}
