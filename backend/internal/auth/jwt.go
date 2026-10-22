package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"smart-credit-hub/internal/domain"
)

var (
	ErrTokenExpired = errors.New("токен авторизации истек")
	ErrTokenInvalid = errors.New("недействительный токен авторизации")
)

// Claims описывает полезную нагрузку JWT в соответствии с требованиями.
type Claims struct {
	UserID      string      `json:"user_id"`
	Role        domain.Role `json:"role"`
	INN         *string     `json:"inn,omitempty"`
	DisplayName string      `json:"display_name"`
	jwt.RegisteredClaims
}

// TokenManager отвечает за создание и валидацию JWT.
type TokenManager struct {
	secretKey     []byte
	lifetimeHours int
}

// NewTokenManager создает новый экземпляр TokenManager.
func NewTokenManager(secretKey string, lifetimeHours int) *TokenManager {
	if lifetimeHours <= 0 {
		lifetimeHours = 24
	}
	return &TokenManager{
		secretKey:     []byte(secretKey),
		lifetimeHours: lifetimeHours,
	}
}

// GenerateToken генерирует подписанный JWT для переданного пользователя.
func (tm *TokenManager) GenerateToken(user *domain.User) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:      user.ID,
		Role:        user.Role,
		INN:         user.INN,
		DisplayName: user.DisplayName,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			Issuer:    "smart-credit-hub",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(tm.lifetimeHours) * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(tm.secretKey)
	if err != nil {
		return "", fmt.Errorf("ошибка генерации токена: %w", err)
	}

	return signedToken, nil
}

// ValidateToken проверяет валидность JWT и возвращает Claims.
func (tm *TokenManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("неподдерживаемый алгоритм подписи: %v", t.Header["alg"])
		}
		return tm.secretKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}

	return claims, nil
}
