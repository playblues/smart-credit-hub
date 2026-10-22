package auth

import (
	"context"
	"strings"
	"sync"

	"smart-credit-hub/internal/domain"
)

// Service интерфейс сервиса аутентификации и авторизации.
type Service interface {
	Authenticate(ctx context.Context, login, password string) (*domain.LoginResponse, error)
	ValidateToken(tokenString string) (*Claims, error)
	GetUserByID(ctx context.Context, userID string) (*domain.User, error)
	GetUsers() []domain.User
}

type service struct {
	tokenManager *TokenManager
	mu           sync.RWMutex
	usersByID    map[string]domain.User
	usersByLogin map[string]domain.User
	usersByINN   map[string]domain.User
}

// NewService создает сервис аутентификации с предзаполненными демо-пользователями.
func NewService(tokenManager *TokenManager) Service {
	s := &service{
		tokenManager: tokenManager,
		usersByID:    make(map[string]domain.User),
		usersByLogin: make(map[string]domain.User),
		usersByINN:   make(map[string]domain.User),
	}

	s.initDemoUsers()
	return s
}

// initDemoUsers инициализирует 6 демо-пользователей: 5 клиентов из cases.json + 1 андеррайтер.
func (s *service) initDemoUsers() {
	demoUsers := []domain.User{
		{
			ID:          "usr_client_1",
			Login:       "client_alfa",
			Password:    "demo123",
			Role:        domain.RoleClient,
			INN:         domain.StringPtr("7701000001"),
			DisplayName: "ООО «Альфа Логистика»",
		},
		{
			ID:          "usr_client_2",
			Login:       "client_smirnov",
			Password:    "demo123",
			Role:        domain.RoleClient,
			INN:         domain.StringPtr("7701000002"),
			DisplayName: "ИП Смирнов Алексей Петрович",
		},
		{
			ID:          "usr_client_3",
			Login:       "client_technostart",
			Password:    "demo123",
			Role:        domain.RoleClient,
			INN:         domain.StringPtr("7701000003"),
			DisplayName: "ООО «ТехноСтарт»",
		},
		{
			ID:          "usr_client_4",
			Login:       "client_negativ",
			Password:    "demo123",
			Role:        domain.RoleClient,
			INN:         domain.StringPtr("7701000004"),
			DisplayName: "ООО «Негатив Трейд»",
		},
		{
			ID:          "usr_client_5",
			Login:       "client_megastroy",
			Password:    "demo123",
			Role:        domain.RoleClient,
			INN:         domain.StringPtr("7701000005"),
			DisplayName: "ООО «Мегастрой»",
		},
		{
			ID:          "usr_underwriter_1",
			Login:       "underwriter",
			Password:    "demo123",
			Role:        domain.RoleUnderwriter,
			INN:         nil,
			DisplayName: "Главный Андеррайтер",
		},
	}

	for _, u := range demoUsers {
		s.usersByID[u.ID] = u
		s.usersByLogin[strings.ToLower(u.Login)] = u
		if u.INN != nil {
			s.usersByINN[*u.INN] = u
		}
	}
}

// Authenticate проверяет учетные данные пользователя и выдает JWT токен.
func (s *service) Authenticate(ctx context.Context, login, password string) (*domain.LoginResponse, error) {
	login = strings.TrimSpace(login)
	password = strings.TrimSpace(password)

	if login == "" || password == "" {
		return nil, domain.ErrInvalidCredentials()
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Поиск по логину, либо по ИНН (для удобства ввода в фронтенде и тестах)
	user, exists := s.usersByLogin[strings.ToLower(login)]
	if !exists {
		user, exists = s.usersByINN[login]
	}

	if !exists {
		return nil, domain.ErrInvalidCredentials()
	}

	// Сверка пароля (для демо-пользователей "demo123")
	if user.Password != password {
		return nil, domain.ErrInvalidCredentials()
	}

	token, err := s.tokenManager.GenerateToken(&user)
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.usersByID[userID]
	if !exists {
		return nil, domain.ErrNotFound("Пользователь не найден", nil)
	}

	return &user, nil
}

// GetUsers возвращает список всех пользователей (для отладки/демо).
func (s *service) GetUsers() []domain.User {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]domain.User, 0, len(s.usersByID))
	for _, u := range s.usersByID {
		users = append(users, u)
	}
	return users
}
