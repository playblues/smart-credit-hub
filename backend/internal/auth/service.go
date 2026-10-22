package auth

import (
	"context"
	"strings"

	"smart-credit-hub/internal/domain"
	"smart-credit-hub/internal/storage"
)

// Service интерфейс сервиса аутентификации и авторизации.
type Service interface {
	Authenticate(ctx context.Context, login, password string) (*domain.LoginResponse, error)
	ValidateToken(tokenString string) (*Claims, error)
	GetUserByID(ctx context.Context, userID string) (*domain.User, error)
}

type service struct {
	tokenManager *TokenManager
	userRepo     storage.UserRepository
}

// NewService создает сервис аутентификации с использованием UserRepository.
func NewService(tokenManager *TokenManager, userRepo storage.UserRepository) Service {
	return &service{
		tokenManager: tokenManager,
		userRepo:     userRepo,
	}
}

// GetDemoUsers возвращает эталонный список 6 демо-пользователей (для сидирования и тестов).
func GetDemoUsers() []domain.User {
	return []domain.User{
		{
			ID:           "00000000-0000-0000-0000-000000000001",
			Login:        "client_alfa",
			PasswordHash: "demo123",
			Password:     "demo123",
			Role:         domain.RoleClient,
			INN:          domain.StringPtr("7701000001"),
			DisplayName:  "ООО «Альфа Логистика»",
		},
		{
			ID:           "00000000-0000-0000-0000-000000000002",
			Login:        "client_smirnov",
			PasswordHash: "demo123",
			Password:     "demo123",
			Role:         domain.RoleClient,
			INN:          domain.StringPtr("7701000002"),
			DisplayName:  "ИП Смирнов Алексей Петрович",
		},
		{
			ID:           "00000000-0000-0000-0000-000000000003",
			Login:        "client_technostart",
			PasswordHash: "demo123",
			Password:     "demo123",
			Role:         domain.RoleClient,
			INN:          domain.StringPtr("7701000003"),
			DisplayName:  "ООО «ТехноСтарт»",
		},
		{
			ID:           "00000000-0000-0000-0000-000000000004",
			Login:        "client_negativ",
			PasswordHash: "demo123",
			Password:     "demo123",
			Role:         domain.RoleClient,
			INN:          domain.StringPtr("7701000004"),
			DisplayName:  "ООО «Негатив Трейд»",
		},
		{
			ID:           "00000000-0000-0000-0000-000000000005",
			Login:        "client_megastroy",
			PasswordHash: "demo123",
			Password:     "demo123",
			Role:         domain.RoleClient,
			INN:          domain.StringPtr("7701000005"),
			DisplayName:  "ООО «Мегастрой»",
		},
		{
			ID:           "00000000-0000-0000-0000-000000000099",
			Login:        "underwriter",
			PasswordHash: "demo123",
			Password:     "demo123",
			Role:         domain.RoleUnderwriter,
			INN:          nil,
			DisplayName:  "Главный Андеррайтер",
		},
	}
}

// Authenticate проверяет учетные данные пользователя через UserRepository и выдает JWT токен.
func (s *service) Authenticate(ctx context.Context, login, password string) (*domain.LoginResponse, error) {
	login = strings.TrimSpace(login)
	password = strings.TrimSpace(password)

	if login == "" || password == "" {
		return nil, domain.ErrInvalidCredentials()
	}

	user, err := s.userRepo.GetByLoginOrINN(ctx, login)
	if err != nil {
		return nil, domain.ErrInvalidCredentials()
	}

	// Сверка пароля (для демо-пользователей "demo123" или безопасного хэша)
	if !user.CheckPassword(password) {
		return nil, domain.ErrInvalidCredentials()
	}

	token, err := s.tokenManager.GenerateToken(user)
	if err != nil {
		return nil, domain.ErrInternal(domain.StringPtr("Не удалось сгенерировать токен авторизации"))
	}

	return &domain.LoginResponse{
		Token:       token,
		Role:        user.Role,
		INN:         user.INN,
		DisplayName: user.DisplayName,
	}, nil
}

// ValidateToken проверяет токен через TokenManager.
func (s *service) ValidateToken(tokenString string) (*Claims, error) {
	return s.tokenManager.ValidateToken(tokenString)
}

// GetUserByID возвращает пользователя по идентификатору.
func (s *service) GetUserByID(ctx context.Context, userID string) (*domain.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}
