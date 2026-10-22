package auth_test

import (
	"context"
	"testing"
	"time"

	"smart-credit-hub/internal/auth"
	"smart-credit-hub/internal/domain"
	"smart-credit-hub/internal/storage"
)

func TestTokenManager_GenerateAndValidate(t *testing.T) {
	secret := "test-secret-key-super-secure"
	tm := auth.NewTokenManager(secret, 2)

	inn := "7701000001"
	user := &domain.User{
		ID:          "00000000-0000-0000-0000-000000000001",
		Login:       "client_alfa",
		Role:        domain.RoleClient,
		INN:         &inn,
		DisplayName: "ООО «Альфа»",
	}

	token, err := tm.GenerateToken(user)
	if err != nil {
		t.Fatalf("ошибка генерации токена: %v", err)
	}
	if token == "" {
		t.Fatal("токен пустой")
	}

	claims, err := tm.ValidateToken(token)
	if err != nil {
		t.Fatalf("ошибка валидации токена: %v", err)
	}

	if claims.UserID != user.ID {
		t.Errorf("ожидался user_id %s, получен %s", user.ID, claims.UserID)
	}
	if claims.Role != user.Role {
		t.Errorf("ожидалась роль %s, получена %s", user.Role, claims.Role)
	}
	if claims.INN == nil || *claims.INN != inn {
		t.Errorf("ожидался ИНН %s, получен %v", inn, claims.INN)
	}
	if claims.DisplayName != user.DisplayName {
		t.Errorf("ожидался display_name %s, получен %s", user.DisplayName, claims.DisplayName)
	}

	// Проверка невалидного ключа
	wrongTM := auth.NewTokenManager("different-secret-key", 2)
	_, err = wrongTM.ValidateToken(token)
	if err == nil {
		t.Error("ожидалась ошибка валидации с чужим секретным ключом")
	}
}

func TestAuthService_Authenticate(t *testing.T) {
	tm := auth.NewTokenManager("test-secret", 1)
	userRepo := storage.NewInMemoryUserRepository(auth.GetDemoUsers()...)
	svc := auth.NewService(tm, userRepo)
	ctx := context.Background()

	// 1. Успешный вход по логину
	resp, err := svc.Authenticate(ctx, "client_alfa", "demo123")
	if err != nil {
		t.Fatalf("ошибка входа по логину: %v", err)
	}
	if resp.Role != domain.RoleClient {
		t.Errorf("ожидалась роль CLIENT, получена: %s", resp.Role)
	}

	// 2. Успешный вход по ИНН
	respINN, err := svc.Authenticate(ctx, "7701000001", "demo123")
	if err != nil {
		t.Fatalf("ошибка входа по ИНН: %v", err)
	}
	if respINN.Role != domain.RoleClient {
		t.Errorf("ожидалась роль CLIENT, получена: %s", respINN.Role)
	}

	// 3. Неверный пароль
	_, err = svc.Authenticate(ctx, "client_alfa", "wrongpass")
	if err == nil {
		t.Error("ожидалась ошибка при неверном пароле")
	}

	// 4. Несуществующий пользователь
	_, err = svc.Authenticate(ctx, "nonexistent", "demo123")
	if err == nil {
		t.Error("ожидалась ошибка при несуществующем пользователе")
	}

	// 5. Поиск по ID
	user, err := svc.GetUserByID(ctx, "00000000-0000-0000-0000-000000000001")
	if err != nil {
		t.Fatalf("ошибка получения пользователя по ID: %v", err)
	}
	if user.Login != "client_alfa" {
		t.Errorf("ожидался логин client_alfa, получен: %s", user.Login)
	}
}

func TestTokenManager_Expiration(t *testing.T) {
	tm := auth.NewTokenManager("secret", 1)
	user := &domain.User{
		ID:          "usr_exp",
		Login:       "exp",
		Role:        domain.RoleClient,
		DisplayName: "Exp",
	}

	token, err := tm.GenerateToken(user)
	if err != nil {
		t.Fatalf("ошибка генерации: %v", err)
	}

	claims, err := tm.ValidateToken(token)
	if err != nil {
		t.Fatalf("ошибка валидации: %v", err)
	}

	if claims.ExpiresAt.Before(time.Now()) {
		t.Errorf("время истечения токена в прошлом")
	}
}
