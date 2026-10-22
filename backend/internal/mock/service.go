package mock

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"smart-credit-hub/internal/domain"
)

// Service интерфейс мок-сервиса внешних систем (ФНС и БКИ).
type Service interface {
	GetFNSByINN(ctx context.Context, inn string) (*domain.FNSCompany, error)
	GetBKIByINN(ctx context.Context, inn string) (*domain.BKICreditHistory, error)
	GetAllCases() []domain.TestCaseItem
}

type service struct {
	mu        sync.RWMutex
	fnsByINN  map[string]domain.FNSCompany
	bkiByINN  map[string]domain.BKICreditHistory
	casesList []domain.TestCaseItem
}

// NewService создает и инициализирует сервис мок-данных ФНС и БКИ.
// Пытается загрузить данные из filePath, а в случае отсутствия файла использует встроенные эталонные данные.
func NewService(filePath string) (Service, error) {
	s := &service{
		fnsByINN: make(map[string]domain.FNSCompany),
		bkiByINN: make(map[string]domain.BKICreditHistory),
	}

	if err := s.loadFromFile(filePath); err != nil {
		slog.Warn("Не удалось загрузить cases.json из файла, используются встроенные эталонные данные",
			"path", filePath,
			"error", err,
		)
		s.loadDefaultData()
	}

	return s, nil
}

// loadFromFile парсит JSON-файл с кейсами.
func (s *service) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("чтение файла: %w", err)
	}

	var cases []domain.TestCaseItem
	if err := json.Unmarshal(data, &cases); err != nil {
		return fmt.Errorf("десериализация json: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.casesList = cases
	for _, c := range cases {
		// Обогащаем ОКВЭД по умолчанию, если не задан в кейсе
		comp := c.Company
		if comp.OKVED == "" {
			comp.OKVED = "49.41"
		}
		s.fnsByINN[comp.INN] = comp

		bki := c.BKI
		bki.INN = comp.INN
		s.bkiByINN[comp.INN] = bki
	}

	slog.Info("Мок-данные ФНС и БКИ успешно загружены из файла",
		"path", path,
		"records_count", len(cases),
	)
	return nil
}

// loadDefaultData загружает 5 эталонных кейсов из ТЗ хакатона как надежный fallback.
func (s *service) loadDefaultData() {
	defaultCases := []domain.TestCaseItem{
		{
			CaseID:      "CASE_1_GREEN",
			Description: "Зрелый бизнес, выручка 50 млн при запросе 3 млн, чистая БКИ. Автоодобрение.",
			Company: domain.FNSCompany{
				INN:               "7701000001",
				Name:              "ООО «Альфа Логистика»",
				OKVED:             "49.41",
				AnnualRevenue:     50000000,
				BusinessAgeMonths: 48,
				IsLiquidating:     false,
			},
			BKI: domain.BKICreditHistory{
				INN:                "7701000001",
				ActiveLoansCount:   1,
				MaxOverdueDays:     0,
				HadPastDelinquency: false,
			},
		},
		{
			CaseID:      "CASE_2_YELLOW",
			Description: "Запрос 5 млн при обороте 8 млн, PTI 37% > 35%. Контр-оффер на 3.25 млн.",
			Company: domain.FNSCompany{
				INN:               "7701000002",
				Name:              "ИП Смирнов Алексей Петрович",
				OKVED:             "47.19",
				AnnualRevenue:     8000000,
				BusinessAgeMonths: 18,
				IsLiquidating:     false,
			},
			BKI: domain.BKICreditHistory{
				INN:                "7701000002",
				ActiveLoansCount:   2,
				MaxOverdueDays:     12,
				HadPastDelinquency: true,
			},
		},
		{
			CaseID:      "CASE_3_GREY",
			Description: "Молодой бизнес (8 мес), 4 кредита. Серая зона: АРМ андеррайтера, запрос баланса.",
			Company: domain.FNSCompany{
				INN:               "7701000003",
				Name:              "ООО «ТехноСтарт»",
				OKVED:             "62.01",
				AnnualRevenue:     12000000,
				BusinessAgeMonths: 8,
				IsLiquidating:     false,
			},
			BKI: domain.BKICreditHistory{
				INN:                "7701000003",
				ActiveLoansCount:   4,
				MaxOverdueDays:     25,
				HadPastDelinquency: true,
			},
		},
		{
			CaseID:      "CASE_4_RED",
			Description: "Просрочка 115 дней в БКИ. Стоп-фактор, отказ.",
			Company: domain.FNSCompany{
				INN:               "7701000004",
				Name:              "ООО «Негатив Трейд»",
				OKVED:             "46.90",
				AnnualRevenue:     20000000,
				BusinessAgeMonths: 36,
				IsLiquidating:     false,
			},
			BKI: domain.BKICreditHistory{
				INN:                "7701000004",
				ActiveLoansCount:   4,
				MaxOverdueDays:     115,
				HadPastDelinquency: true,
			},
		},
		{
			CaseID:      "CASE_5_LIMIT",
			Description: "Хороший заёмщик просит выше лимита продукта (15 млн при лимите 10 млн). Контр-оффер на лимит.",
			Company: domain.FNSCompany{
				INN:               "7701000005",
				Name:              "ООО «Мегастрой»",
				OKVED:             "41.20",
				AnnualRevenue:     200000000,
				BusinessAgeMonths: 60,
				IsLiquidating:     false,
			},
			BKI: domain.BKICreditHistory{
				INN:                "7701000005",
				ActiveLoansCount:   0,
				MaxOverdueDays:     0,
				HadPastDelinquency: false,
			},
		},
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.casesList = defaultCases
	for _, c := range defaultCases {
		s.fnsByINN[c.Company.INN] = c.Company
		s.bkiByINN[c.Company.INN] = c.BKI
	}
}

// GetFNSByINN возвращает данные компании по ИНН из сервиса ФНС.
func (s *service) GetFNSByINN(ctx context.Context, inn string) (*domain.FNSCompany, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	comp, exists := s.fnsByINN[inn]
	if !exists {
		return nil, domain.ErrNotFound(fmt.Sprintf("Компания с ИНН %s не найдена в ФНС", inn), nil)
	}

	return &comp, nil
}

// GetBKIByINN возвращает кредитную историю по ИНН из сервиса БКИ.
func (s *service) GetBKIByINN(ctx context.Context, inn string) (*domain.BKICreditHistory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bki, exists := s.bkiByINN[inn]
	if !exists {
		return nil, domain.ErrNotFound(fmt.Sprintf("Кредитная история для ИНН %s не найдена в БКИ", inn), nil)
	}

	return &bki, nil
}

// GetAllCases возвращает все тестовые кейсы.
func (s *service) GetAllCases() []domain.TestCaseItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cases := make([]domain.TestCaseItem, len(s.casesList))
	copy(cases, s.casesList)
	return cases
}
