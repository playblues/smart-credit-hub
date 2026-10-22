package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"smart-credit-hub/internal/domain"
)

// UserRepository интерфейс для доступа к данным пользователей.
type UserRepository interface {
	GetByLoginOrINN(ctx context.Context, identifier string) (*domain.User, error)
	GetByID(ctx context.Context, id string) (*domain.User, error)
}

// postgresUserRepository реализация UserRepository поверх PostgreSQL пула соединений.
type postgresUserRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresUserRepository создает репозиторий пользователей с подключением к Postgres.
func NewPostgresUserRepository(pool *pgxpool.Pool) UserRepository {
	return &postgresUserRepository{pool: pool}
}

// GetByLoginOrINN ищет пользователя по логину или ИНН компании.
func (r *postgresUserRepository) GetByLoginOrINN(ctx context.Context, identifier string) (*domain.User, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, domain.ErrNotFound("Идентификатор пользователя не указан", nil)
	}

	query := `
		SELECT id, login, password_hash, role, inn, display_name, created_at
		FROM users
		WHERE LOWER(login) = LOWER($1) OR inn = $1
		LIMIT 1;
	`

	var u domain.User
	var roleStr string
	err := r.pool.QueryRow(ctx, query, identifier).Scan(
		&u.ID,
		&u.Login,
		&u.PasswordHash,
		&roleStr,
		&u.INN,
		&u.DisplayName,
		&u.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound(fmt.Sprintf("Пользователь с логином/ИНН '%s' не найден", identifier), nil)
		}
		return nil, fmt.Errorf("ошибка запроса пользователя: %w", err)
	}

	u.Role = domain.Role(roleStr)
	u.Password = u.PasswordHash

	return &u, nil
}

// GetByID ищет пользователя по UUID.
func (r *postgresUserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, domain.ErrNotFound("ID пользователя не указан", nil)
	}

	query := `
		SELECT id, login, password_hash, role, inn, display_name, created_at
		FROM users
		WHERE id = $1
		LIMIT 1;
	`

	var u domain.User
	var roleStr string
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.Login,
		&u.PasswordHash,
		&roleStr,
		&u.INN,
		&u.DisplayName,
		&u.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound(fmt.Sprintf("Пользователь с ID '%s' не найден", id), nil)
		}
		return nil, fmt.Errorf("ошибка запроса пользователя по ID: %w", err)
	}

	u.Role = domain.Role(roleStr)
	u.Password = u.PasswordHash

	return &u, nil
}

// inMemoryUserRepository реализация UserRepository для автономных тестов без Postgres.
type inMemoryUserRepository struct {
	mu           sync.RWMutex
	usersByID    map[string]domain.User
	usersByLogin map[string]domain.User
	usersByINN   map[string]domain.User
}

// NewInMemoryUserRepository создает репозиторий в памяти с возможностью предзаполнения.
func NewInMemoryUserRepository(initialUsers ...domain.User) UserRepository {
	repo := &inMemoryUserRepository{
		usersByID:    make(map[string]domain.User),
		usersByLogin: make(map[string]domain.User),
		usersByINN:   make(map[string]domain.User),
	}

	for _, u := range initialUsers {
		repo.Save(u)
	}

	return repo
}

// Save сохраняет пользователя в память.
func (r *inMemoryUserRepository) Save(u domain.User) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if u.PasswordHash == "" && u.Password != "" {
		u.PasswordHash = u.Password
	}
	if u.Password == "" && u.PasswordHash != "" {
		u.Password = u.PasswordHash
	}

	r.usersByID[u.ID] = u
	r.usersByLogin[strings.ToLower(u.Login)] = u
	if u.INN != nil && *u.INN != "" {
		r.usersByINN[*u.INN] = u
	}
}

// GetByLoginOrINN ищет пользователя в памяти.
func (r *inMemoryUserRepository) GetByLoginOrINN(ctx context.Context, identifier string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	identifier = strings.TrimSpace(identifier)
	if u, ok := r.usersByLogin[strings.ToLower(identifier)]; ok {
		return &u, nil
	}
	if u, ok := r.usersByINN[identifier]; ok {
		return &u, nil
	}

	return nil, domain.ErrNotFound(fmt.Sprintf("Пользователь '%s' не найден", identifier), nil)
}

// GetByID ищет пользователя по ID в памяти.
func (r *inMemoryUserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if u, ok := r.usersByID[id]; ok {
		return &u, nil
	}

	return nil, domain.ErrNotFound(fmt.Sprintf("Пользователь с ID '%s' не найден", id), nil)
}
