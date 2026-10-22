package mock_test

import (
	"context"
	"testing"

	"smart-credit-hub/internal/config"
	"smart-credit-hub/internal/mock"
)

func TestMockService_Cases(t *testing.T) {
	casesPath := config.FindCasesFilePath("")
	svc, err := mock.NewService(casesPath)
	if err != nil {
		t.Fatalf("ошибка инициализации mock.Service: %v", err)
	}

	ctx := context.Background()

	// 1. Проверка всех 5 эталонных компаний из кейсов
	testINNs := []string{"7701000001", "7701000002", "7701000003", "7701000004", "7701000005"}

	for _, inn := range testINNs {
		comp, err := svc.GetFNSByINN(ctx, inn)
		if err != nil {
			t.Errorf("не удалось получить компанию с ИНН %s: %v", inn, err)
		}
		if comp == nil || comp.INN != inn {
			t.Errorf("некорректный ИНН компании, ожидался %s", inn)
		}

		bki, err := svc.GetBKIByINN(ctx, inn)
		if err != nil {
			t.Errorf("не удалось получить данные БКИ для ИНН %s: %v", inn, err)
		}
		if bki == nil || bki.INN != inn {
			t.Errorf("некорректный ИНН БКИ, ожидался %s", inn)
		}
	}

	// 2. Проверка несуществующего ИНН
	_, err = svc.GetFNSByINN(ctx, "0000000000")
	if err == nil {
		t.Error("ожидалась ошибка для несуществующего ИНН в ФНС")
	}

	_, err = svc.GetBKIByINN(ctx, "0000000000")
	if err == nil {
		t.Error("ожидалась ошибка для несуществующего ИНН в БКИ")
	}

	// 3. Проверка GetAllCases
	cases := svc.GetAllCases()
	if len(cases) != 5 {
		t.Errorf("ожидалось 5 кейсов, получено: %d", len(cases))
	}
}
