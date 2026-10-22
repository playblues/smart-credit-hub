package storage_test

import (
	"context"
	"testing"

	"smart-credit-hub/internal/domain"
	"smart-credit-hub/internal/storage"
)

func TestInMemoryUserRepository(t *testing.T) {
	inn := "7701000001"
	testUser := domain.User{
		ID:           "00000000-0000-0000-0000-000000000001",
		Login:        "client_alfa",
		PasswordHash: "demo123",
		Role:         domain.RoleClient,
		INN:          &inn,
		DisplayName:  "ООО «Альфа Логистика»",
	}

	repo := storage.NewInMemoryUserRepository(testUser)
	ctx := context.Background()

	// 1. Поиск по логину (регистронезависимый)
	userByLogin, err := repo.GetByLoginOrINN(ctx, "CLIENT_ALFA")
	if err != nil {
		t.Fatalf("ошибка поиска по логину: %v", err)
	}
	if userByLogin.ID != testUser.ID {
		t.Errorf("ожидался ID %s, получен %s", testUser.ID, userByLogin.ID)
	}
	if !userByLogin.CheckPassword("demo123") {
		t.Error("ожидалась успешная проверка пароля demo123")
	}

	// 2. Поиск по ИНН
	userByINN, err := repo.GetByLoginOrINN(ctx, "7701000001")
	if err != nil {
		t.Fatalf("ошибка поиска по ИНН: %v", err)
	}
	if userByINN.Login != testUser.Login {
		t.Errorf("ожидался логин %s, получен %s", testUser.Login, userByINN.Login)
	}

	// 3. Поиск по ID
	userByID, err := repo.GetByID(ctx, testUser.ID)
	if err != nil {
		t.Fatalf("ошибка поиска по ID: %v", err)
	}
	if userByID.DisplayName != testUser.DisplayName {
		t.Errorf("ожидался display_name %s, получен %s", testUser.DisplayName, userByID.DisplayName)
	}

	// 4. Поиск несуществующего пользователя по логину/ИНН
	_, err = repo.GetByLoginOrINN(ctx, "unknown_user")
	if err == nil {
		t.Error("ожидалась ошибка для несуществующего логина")
	}

	// 5. Поиск несуществующего пользователя по ID
	_, err = repo.GetByID(ctx, "00000000-0000-0000-0000-999999999999")
	if err == nil {
		t.Error("ожидалась ошибка для несуществующего ID")
	}
}
