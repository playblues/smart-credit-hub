package scoring

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"

	"smart-credit-hub/internal/domain"
)

//go:embed default_model.json
var defaultModelJSON []byte

// StopFactorConfig описывает конфигурацию одного стоп-фактора из model.json.
type StopFactorConfig struct {
	ID           string `json:"id"`
	Field        string `json:"field"`
	Op           string `json:"op"`
	Threshold    any    `json:"threshold"`
	ReasonClient string `json:"reason_client"`
	ReasonBank   string `json:"reason_bank"`
}

// WeightsConfig веса признаков линейной комбинации z-score.
type WeightsConfig struct {
	RevenueToLoanRatio    float64 `json:"revenue_to_loan_ratio"`
	BusinessAgeYears      float64 `json:"business_age_years"`
	BKIActiveLoansCount   float64 `json:"bki_active_loans_count"`
	BKIHadPastDelinquency float64 `json:"bki_had_past_delinquency"`
}

// FormulaConfig параметры математической формулы скоринга.
type FormulaConfig struct {
	Intercept float64       `json:"intercept"`
	RatioCap  float64       `json:"ratio_cap"`
	Weights   WeightsConfig `json:"weights"`
	PD        string        `json:"pd"`
	Score     string        `json:"score"`
}

// RiskZoneRule правило отнесения к зоне риска и действие по умолчанию.
type RiskZoneRule struct {
	MaxPD  float64 `json:"max_pd"`
	Action string  `json:"action"`
}

// ProductConfig лимиты и базовая процентная ставка кредитного продукта.
type ProductConfig struct {
	Name          string  `json:"name"`
	MaxAmount     float64 `json:"max_amount"`
	MaxTermMonths int     `json:"max_term_months"`
	BaseRate      float64 `json:"base_rate"`
}

// LimitsValidation правила валидации превышения лимитов.
type LimitsValidation struct {
	ExceedingAction string `json:"exceeding_action"`
	ReasonClient    string `json:"reason_client"`
}

// CounterOfferRules правила расчета встречного предложения (контр-оффера).
type CounterOfferRules struct {
	MaxPTI              float64 `json:"max_pti"`
	AmountCutMultiplier float64 `json:"amount_cut_multiplier"`
	AmountStep          float64 `json:"amount_step"`
	RateMarkup          float64 `json:"rate_markup"`
	PTIDefinition       string  `json:"pti_definition"`
	ReasonClientCut     string  `json:"reason_client_cut"`
	ReasonClientLimit   string  `json:"reason_client_limit"`
}

// ModelConfig полная конфигурация скоринговой модели из scoring/model.json.
type ModelConfig struct {
	Version           string                   `json:"version"`
	Name              string                   `json:"name"`
	StopFactors       []StopFactorConfig       `json:"stop_factors"`
	ScoringFormula    FormulaConfig            `json:"scoring_formula"`
	RiskZones         map[string]RiskZoneRule  `json:"risk_zones"`
	Products          map[string]ProductConfig `json:"products"`
	LimitsValidation  LimitsValidation         `json:"limits_validation"`
	CounterOfferRules CounterOfferRules        `json:"counter_offer_rules"`
}

// CompanyInput входные данные компании для расчета скоринга.
type CompanyInput struct {
	INN               string  `json:"inn"`
	Name              string  `json:"name"`
	AnnualRevenue     float64 `json:"annual_revenue"`
	BusinessAgeMonths int     `json:"business_age_months"`
	IsLiquidating     bool    `json:"is_liquidating"`
}

// BKIInput входные данные кредитной истории БКИ.
type BKIInput struct {
	ActiveLoansCount   int  `json:"active_loans_count"`
	MaxOverdueDays     int  `json:"max_overdue_days"`
	HadPastDelinquency bool `json:"had_past_delinquency"`
}

// RequestInput параметры запроса клиента.
type RequestInput struct {
	ProductType         domain.ProductType `json:"product_type"`
	RequestedAmount     float64            `json:"requested_amount"`
	RequestedTermMonths int                `json:"requested_term_months"`
	Purpose             string             `json:"purpose"`
}

// ScoringInput совокупный входной пакет данных для скоринговой модели.
type ScoringInput struct {
	Company CompanyInput `json:"company"`
	BKI     BKIInput     `json:"bki"`
	Request RequestInput `json:"request"`
}

// CounterOfferResult результат расчета встречного предложения.
type CounterOfferResult struct {
	Amount     float64 `json:"amount"`
	TermMonths int     `json:"term_months"`
	Rate       float64 `json:"rate"`
	PTI        float64 `json:"pti"`
	Reason     string  `json:"reason"`
}

// ScoringOutput итоговый результат выполнения скоринга заявки.
type ScoringOutput struct {
	Status             domain.ApplicationStatus `json:"status"`
	RiskZone           domain.RiskZone          `json:"risk_zone"`
	StopFactor         *string                  `json:"stop_factor,omitempty"`
	ReasonClient       *string                  `json:"reason_client,omitempty"`
	ZScore             *float64                 `json:"z_score,omitempty"`
	PD                 *float64                 `json:"pd,omitempty"`
	Score              *int                     `json:"score,omitempty"`
	ApprovedAmount     *float64                 `json:"approved_amount,omitempty"`
	ApprovedTermMonths *int                     `json:"approved_term_months,omitempty"`
	InterestRate       *float64                 `json:"interest_rate,omitempty"`
	CounterOffer       *CounterOfferResult      `json:"counter_offer,omitempty"`
	FactorsClient      []string                 `json:"factors_client,omitempty"`
	FactorsBank        []string                 `json:"factors_bank,omitempty"`
	ModelVersion       string                   `json:"model_version,omitempty"`
}

// LoadModel загружает конфигурацию модели из файла или встроенного резервного ресурса.
func LoadModel(path string) (*ModelConfig, error) {
	candidates := []string{
		path,
		"scoring/model.json",
		"../scoring/model.json",
		"../../scoring/model.json",
		"../../../scoring/model.json",
		"../../../../scoring/model.json",
	}

	var data []byte
	var err error

	for _, cand := range candidates {
		if cand == "" {
			continue
		}
		if content, readErr := os.ReadFile(cand); readErr == nil {
			data = content
			break
		}
	}

	if data == nil {
		if len(defaultModelJSON) > 0 {
			data = defaultModelJSON
		} else {
			return nil, fmt.Errorf("не удалось найти файл model.json ни по одному из стандартных путей")
		}
	}

	var cfg ModelConfig
	if err = json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("ошибка парсинга model.json: %w", err)
	}

	return &cfg, nil
}
