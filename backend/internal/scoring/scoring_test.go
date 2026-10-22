package scoring_test

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"smart-credit-hub/internal/domain"
	"smart-credit-hub/internal/scoring"
)

type GoldenExpectedCounterOffer struct {
	Amount     float64 `json:"amount"`
	TermMonths int     `json:"term_months"`
	Rate       float64 `json:"rate"`
	PTI        float64 `json:"pti"`
	Reason     string  `json:"reason"`
}

type GoldenExpectedResult struct {
	Status             string                      `json:"status"`
	RiskZone           string                      `json:"risk_zone"`
	ZScore             *float64                    `json:"z_score"`
	PD                 *float64                    `json:"pd"`
	Score              *int                        `json:"score"`
	StopFactor         *string                     `json:"stop_factor"`
	ReasonClient       *string                     `json:"reason_client"`
	ApprovedAmount     *float64                    `json:"approved_amount"`
	ApprovedTermMonths *int                        `json:"approved_term_months"`
	InterestRate       *float64                    `json:"interest_rate"`
	CounterOffer       *GoldenExpectedCounterOffer `json:"counter_offer"`
}

type GoldenTestCase struct {
	CaseID         string               `json:"case_id"`
	Description    string               `json:"description"`
	Company        scoring.CompanyInput `json:"company"`
	BKI            scoring.BKIInput     `json:"bki"`
	Request        scoring.RequestInput `json:"request"`
	ExpectedResult GoldenExpectedResult `json:"expected_result"`
}

func loadGoldenCases(t *testing.T) []GoldenTestCase {
	t.Helper()

	candidates := []string{
		"scoring/testdata/cases.json",
		"../scoring/testdata/cases.json",
		"../../scoring/testdata/cases.json",
		"../../../scoring/testdata/cases.json",
		"../../../../scoring/testdata/cases.json",
	}

	var data []byte
	var err error

	for _, cand := range candidates {
		if content, readErr := os.ReadFile(cand); readErr == nil {
			data = content
			break
		}
	}

	if data == nil {
		t.Fatalf("не удалось найти cases.json по путям %v", candidates)
	}

	var cases []GoldenTestCase
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("ошибка десериализации cases.json: %v", err)
	}

	return cases
}

func TestEvaluate_AllGoldenCases(t *testing.T) {
	cfg, err := scoring.LoadModel("")
	if err != nil {
		t.Fatalf("ошибка загрузки конфигурации модели: %v", err)
	}

	engine := scoring.NewEngine(cfg)
	cases := loadGoldenCases(t)

	if len(cases) != 5 {
		t.Fatalf("ожидалось ровно 5 золотых кейсов, получено: %d", len(cases))
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.CaseID, func(t *testing.T) {
			input := scoring.ScoringInput{
				Company: tc.Company,
				BKI:     tc.BKI,
				Request: tc.Request,
			}

			got := engine.Evaluate(input)
			exp := tc.ExpectedResult

			// 1. Проверка статуса заявки
			if string(got.Status) != exp.Status {
				t.Errorf("[%s] Статус не совпадает: получено %s, ожидалось %s", tc.CaseID, got.Status, exp.Status)
			}

			// 2. Проверка зоны риска
			if string(got.RiskZone) != exp.RiskZone {
				t.Errorf("[%s] Зона риска не совпадает: получено %s, ожидалось %s", tc.CaseID, got.RiskZone, exp.RiskZone)
			}

			// 3. Проверка стоп-фактора
			if exp.StopFactor != nil {
				if got.StopFactor == nil || *got.StopFactor != *exp.StopFactor {
					t.Errorf("[%s] Стоп-фактор не совпадает: получено %v, ожидалось %s", tc.CaseID, got.StopFactor, *exp.StopFactor)
				}
			} else if got.StopFactor != nil {
				t.Errorf("[%s] Стоп-фактор не ожидался, получен: %s", tc.CaseID, *got.StopFactor)
			}

			// 4. Проверка причины для клиента
			if exp.ReasonClient != nil {
				if got.ReasonClient == nil || *got.ReasonClient != *exp.ReasonClient {
					t.Errorf("[%s] Причина клиента не совпадает: получено %v, ожидалось %s", tc.CaseID, got.ReasonClient, *exp.ReasonClient)
				}
			}

			// 5. Проверка z-score
			if exp.ZScore != nil {
				if got.ZScore == nil || math.Abs(*got.ZScore-*exp.ZScore) > 0.001 {
					t.Errorf("[%s] z-score не совпадает: получено %v, ожидалось %f", tc.CaseID, got.ZScore, *exp.ZScore)
				}
			}

			// 6. Проверка PD
			if exp.PD != nil {
				if got.PD == nil || math.Abs(*got.PD-*exp.PD) > 0.0001 {
					t.Errorf("[%s] PD не совпадает: получено %v, ожидалось %f", tc.CaseID, got.PD, *exp.PD)
				}
			}

			// 7. Проверка скорингового балла
			if exp.Score != nil {
				if got.Score == nil || *got.Score != *exp.Score {
					t.Errorf("[%s] Score не совпадает: получено %v, ожидалось %d", tc.CaseID, got.Score, *exp.Score)
				}
			}

			// 8. Проверка одобренных параметров (для AUTO_APPROVED)
			if exp.ApprovedAmount != nil {
				if got.ApprovedAmount == nil || *got.ApprovedAmount != *exp.ApprovedAmount {
					t.Errorf("[%s] ApprovedAmount не совпадает: получено %v, ожидалось %f", tc.CaseID, got.ApprovedAmount, *exp.ApprovedAmount)
				}
			}
			if exp.ApprovedTermMonths != nil {
				if got.ApprovedTermMonths == nil || *got.ApprovedTermMonths != *exp.ApprovedTermMonths {
					t.Errorf("[%s] ApprovedTermMonths не совпадает: получено %v, ожидалось %d", tc.CaseID, got.ApprovedTermMonths, *exp.ApprovedTermMonths)
				}
			}
			if exp.InterestRate != nil {
				if got.InterestRate == nil || math.Abs(*got.InterestRate-*exp.InterestRate) > 0.0001 {
					t.Errorf("[%s] InterestRate не совпадает: получено %v, ожидалось %f", tc.CaseID, got.InterestRate, *exp.InterestRate)
				}
			}

			// 9. Проверка контр-оффера (для COUNTER_OFFERED)
			if exp.CounterOffer != nil {
				if got.CounterOffer == nil {
					t.Fatalf("[%s] Контр-оффер ожидался, но не сформирован", tc.CaseID)
				}
				co := got.CounterOffer
				expCO := exp.CounterOffer

				if co.Amount != expCO.Amount {
					t.Errorf("[%s] Контр-оффер Amount не совпадает: получено %f, ожидалось %f", tc.CaseID, co.Amount, expCO.Amount)
				}
				if co.TermMonths != expCO.TermMonths {
					t.Errorf("[%s] Контр-оффер TermMonths не совпадает: получено %d, ожидалось %d", tc.CaseID, co.TermMonths, expCO.TermMonths)
				}
				if math.Abs(co.Rate-expCO.Rate) > 0.0001 {
					t.Errorf("[%s] Контр-оффер Rate не совпадает: получено %f, ожидалось %f", tc.CaseID, co.Rate, expCO.Rate)
				}
				if math.Abs(co.PTI-expCO.PTI) > 0.0001 {
					t.Errorf("[%s] Контр-оффер PTI не совпадает: получено %f, ожидалось %f", tc.CaseID, co.PTI, expCO.PTI)
				}
				if co.Reason != expCO.Reason {
					t.Errorf("[%s] Контр-оффер Reason не совпадает:\nполучено:  %s\nожидалось: %s", tc.CaseID, co.Reason, expCO.Reason)
				}
			} else if got.CounterOffer != nil {
				t.Errorf("[%s] Контр-оффер не ожидался, получен: %+v", tc.CaseID, got.CounterOffer)
			}

			// 10. Проверка наличия факторов объяснимости
			if len(got.FactorsClient) == 0 {
				t.Errorf("[%s] Список factors_client пуст", tc.CaseID)
			}
			if len(got.FactorsBank) == 0 {
				t.Errorf("[%s] Список factors_bank пуст", tc.CaseID)
			}
		})
	}
}

func TestAnnuityPaymentAndPTI(t *testing.T) {
	// Сумма 3 млн, 18.5% годовых, 12 месяцев
	amount := 3000000.0
	rate := 0.185
	months := 12
	revenue := 50000000.0

	annuity := scoring.AnnuityPayment(amount, rate, months)
	if annuity <= 0 {
		t.Fatalf("аннуитетный платеж должен быть положительным, получено: %f", annuity)
	}

	pti := scoring.PTI(amount, rate, months, revenue)
	if pti <= 0 || pti >= 1.0 {
		t.Errorf("PTI должен быть в диапазоне (0, 1), получено: %f", pti)
	}

	// Проверка срезания вниз с шагом 50 000
	step := 50000.0
	val := 3280000.0
	expected := 3250000.0
	got := scoring.RoundDownStep(val, step)
	if got != expected {
		t.Errorf("RoundDownStep(%f, %f) = %f; ожидалось %f", val, step, got, expected)
	}
}

func TestStopFactors(t *testing.T) {
	cfg, err := scoring.LoadModel("")
	if err != nil {
		t.Fatalf("ошибка загрузки модели: %v", err)
	}
	engine := scoring.NewEngine(cfg)

	// 1. Проверка стоп-фактора возраста бизнеса (< 6 месяцев)
	inputYoung := scoring.ScoringInput{
		Company: scoring.CompanyInput{
			INN:               "7701999001",
			Name:              "Слишком молодой бизнес",
			AnnualRevenue:     10000000,
			BusinessAgeMonths: 3,
			IsLiquidating:     false,
		},
		BKI: scoring.BKIInput{
			ActiveLoansCount:   0,
			MaxOverdueDays:     0,
			HadPastDelinquency: false,
		},
		Request: scoring.RequestInput{
			ProductType:         domain.ProductWorkingCapital,
			RequestedAmount:     1000000,
			RequestedTermMonths: 12,
		},
	}

	resYoung := engine.Evaluate(inputYoung)
	if resYoung.Status != domain.StatusRejected || resYoung.StopFactor == nil || *resYoung.StopFactor != "business_age_under_6m" {
		t.Errorf("ожидался отказ по business_age_under_6m, получено: %v, sf=%v", resYoung.Status, resYoung.StopFactor)
	}

	// 2. Проверка стоп-фактора ликвидации компании
	inputLiq := scoring.ScoringInput{
		Company: scoring.CompanyInput{
			INN:               "7701999002",
			Name:              "Ликвидируемая компания",
			AnnualRevenue:     50000000,
			BusinessAgeMonths: 36,
			IsLiquidating:     true,
		},
		BKI: scoring.BKIInput{
			ActiveLoansCount:   0,
			MaxOverdueDays:     0,
			HadPastDelinquency: false,
		},
		Request: scoring.RequestInput{
			ProductType:         domain.ProductWorkingCapital,
			RequestedAmount:     1000000,
			RequestedTermMonths: 12,
		},
	}

	resLiq := engine.Evaluate(inputLiq)
	if resLiq.Status != domain.StatusRejected || resLiq.StopFactor == nil || *resLiq.StopFactor != "company_liquidating" {
		t.Errorf("ожидался отказ по company_liquidating, получено: %v, sf=%v", resLiq.Status, resLiq.StopFactor)
	}
}
