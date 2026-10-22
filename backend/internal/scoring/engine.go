package scoring

import (
	"fmt"
	"math"

	"smart-credit-hub/internal/domain"
)

// Engine реализует кредитный скоринг и матрицу принятия решений.
type Engine struct {
	config *ModelConfig
}

// NewEngine создает новый экземпляр скорингового движка с заданной конфигурацией.
func NewEngine(cfg *ModelConfig) *Engine {
	return &Engine{config: cfg}
}

// AnnuityPayment рассчитывает ежемесячный аннуитетный платеж.
// r := annualRate / 12.0
// annuity := amount * r / (1.0 - math.Pow(1.0 + r, -float64(months)))
func AnnuityPayment(amount, annualRate float64, months int) float64 {
	if months <= 0 {
		months = 1
	}
	r := annualRate / 12.0
	if r <= 0 {
		return amount / float64(months)
	}
	return amount * r / (1.0 - math.Pow(1.0+r, -float64(months)))
}

// PTI рассчитывает коэффициент долговой нагрузки (Payment-to-Income):
// (12.0 * annuity) / annualRevenue
func PTI(amount, rate float64, months int, annualRevenue float64) float64 {
	if annualRevenue <= 0 {
		return 1.0
	}
	annuity := AnnuityPayment(amount, rate, months)
	return (12.0 * annuity) / annualRevenue
}

// RoundDownStep округляет сумму строго вниз с заданным шагом:
// math.Floor(x / step) * step
func RoundDownStep(x, step float64) float64 {
	if step <= 0 {
		return x
	}
	return math.Floor(x/step) * step
}

// checkStopFactors проверяет входные данные на наличие критических стоп-факторов (fail-fast).
func (e *Engine) checkStopFactors(c CompanyInput, b BKIInput) *StopFactorConfig {
	for _, sf := range e.config.StopFactors {
		switch sf.ID {
		case "bki_overdue_90":
			if b.MaxOverdueDays >= 90 {
				return &sf
			}
		case "company_liquidating":
			if c.IsLiquidating {
				return &sf
			}
		case "business_age_under_6m":
			if c.BusinessAgeMonths < 6 {
				return &sf
			}
		}
	}
	return nil
}

// calculateCounterOffer рассчитывает параметры встречного предложения (контр-оффера).
func (e *Engine) calculateCounterOffer(r RequestInput, c CompanyInput, zone domain.RiskZone) (CounterOfferResult, bool) {
	prod := e.config.Products[string(r.ProductType)]
	coRules := e.config.CounterOfferRules

	overLimit := r.RequestedAmount > prod.MaxAmount || r.RequestedTermMonths > prod.MaxTermMonths

	amount := math.Min(r.RequestedAmount, prod.MaxAmount)
	term := r.RequestedTermMonths
	if term > prod.MaxTermMonths {
		term = prod.MaxTermMonths
	}
	rate := prod.BaseRate
	reason := coRules.ReasonClientLimit

	if zone == domain.RiskZoneYellow {
		amount = RoundDownStep(amount*coRules.AmountCutMultiplier, coRules.AmountStep)
		rate = math.Round((prod.BaseRate+coRules.RateMarkup)*10000.0) / 10000.0
		reason = coRules.ReasonClientCut
	}

	for amount > coRules.AmountStep && PTI(amount, rate, term, c.AnnualRevenue) > coRules.MaxPTI {
		amount -= coRules.AmountStep
		reason = coRules.ReasonClientCut
	}

	ptiVal := math.Round(PTI(amount, rate, term, c.AnnualRevenue)*10000.0) / 10000.0

	return CounterOfferResult{
		Amount:     amount,
		TermMonths: term,
		Rate:       rate,
		PTI:        ptiVal,
		Reason:     reason,
	}, overLimit
}

// Evaluate производит полный расчет скоринга в строгом соответствии с эталонным алгоритмом.
func (e *Engine) Evaluate(input ScoringInput) ScoringOutput {
	c := input.Company
	b := input.BKI
	r := input.Request

	// 1. Проверка стоп-факторов ДО расчета скоринга (fail-fast)
	if sf := e.checkStopFactors(c, b); sf != nil {
		stopFactorID := sf.ID
		reasonClient := sf.ReasonClient
		return ScoringOutput{
			Status:        domain.StatusRejected,
			RiskZone:      domain.RiskZoneRed,
			StopFactor:    &stopFactorID,
			ReasonClient:  &reasonClient,
			ModelVersion:  e.config.Version,
			FactorsBank:   []string{sf.ReasonBank},
			FactorsClient: []string{sf.ReasonClient},
		}
	}

	// 2. Расчет скоринга (z-score, PD, score)
	f := e.config.ScoringFormula
	w := f.Weights

	ratio := c.AnnualRevenue / r.RequestedAmount
	if ratio > f.RatioCap {
		ratio = f.RatioCap
	}

	delinquencyVal := 0.0
	if b.HadPastDelinquency {
		delinquencyVal = 1.0
	}

	z := f.Intercept +
		w.RevenueToLoanRatio*ratio +
		w.BusinessAgeYears*(float64(c.BusinessAgeMonths)/12.0) +
		w.BKIActiveLoansCount*float64(b.ActiveLoansCount) +
		w.BKIHadPastDelinquency*delinquencyVal

	pd := 1.0 / (1.0 + math.Exp(-z))

	// Округления строго по контракту
	zRounded := math.Round(z*1000.0) / 1000.0
	pdRounded := math.Round(pd*10000.0) / 10000.0
	scoreVal := int(math.Round(1000.0 * (1.0 - pd)))

	// 3. Определение зоны риска
	var zone domain.RiskZone
	switch {
	case pd <= e.config.RiskZones["green"].MaxPD:
		zone = domain.RiskZoneGreen
	case pd <= e.config.RiskZones["yellow"].MaxPD:
		zone = domain.RiskZoneYellow
	case pd <= e.config.RiskZones["grey"].MaxPD:
		zone = domain.RiskZoneGrey
	default:
		zone = domain.RiskZoneRed
	}

	out := ScoringOutput{
		ZScore:       &zRounded,
		PD:           &pdRounded,
		Score:        &scoreVal,
		RiskZone:     zone,
		ModelVersion: e.config.Version,
	}

	// 4. Формирование списков факторов
	out.FactorsClient = e.generateClientFactors(ratio, c, b, zone)
	out.FactorsBank = e.generateBankFactors(zRounded, pdRounded, scoreVal, zone, ratio, c, b)

	// 5. Логика решения
	prod := e.config.Products[string(r.ProductType)]

	switch zone {
	case domain.RiskZoneRed:
		out.Status = domain.StatusRejected
		reasonClient := "Совокупный уровень риска превышает допустимый"
		out.ReasonClient = &reasonClient

	case domain.RiskZoneGrey:
		out.Status = domain.StatusManualReview

	default: // GREEN или YELLOW
		co, overLimit := e.calculateCounterOffer(r, c, zone)
		ptiReq := PTI(r.RequestedAmount, prod.BaseRate, r.RequestedTermMonths, c.AnnualRevenue)
		needsOffer := zone == domain.RiskZoneYellow || overLimit || ptiReq > e.config.CounterOfferRules.MaxPTI

		if needsOffer {
			out.Status = domain.StatusCounterOffered
			out.CounterOffer = &co
		} else {
			out.Status = domain.StatusAutoApproved
			reqAmount := r.RequestedAmount
			reqTerm := r.RequestedTermMonths
			baseRate := prod.BaseRate
			out.ApprovedAmount = &reqAmount
			out.ApprovedTermMonths = &reqTerm
			out.InterestRate = &baseRate
		}
	}

	return out
}

func (e *Engine) generateClientFactors(ratio float64, c CompanyInput, b BKIInput, zone domain.RiskZone) []string {
	factors := make([]string, 0, 4)

	if ratio >= 2.0 {
		factors = append(factors, "Высокая финансовая устойчивость: выручка компании значительно превышает сумму кредита")
	} else if ratio >= 1.0 {
		factors = append(factors, "Достаточный уровень выручки относительно запрашиваемого кредита")
	} else {
		factors = append(factors, "Запрашиваемая сумма кредита превышает годовой оборот компании")
	}

	if c.BusinessAgeMonths >= 36 {
		factors = append(factors, "Опыт ведения бизнеса более 3 лет")
	} else if c.BusinessAgeMonths >= 12 {
		factors = append(factors, "Стабильный бизнес более 1 года")
	} else {
		factors = append(factors, "Бизнес на ранней стадии развития (менее 1 года)")
	}

	if !b.HadPastDelinquency && b.ActiveLoansCount <= 1 {
		factors = append(factors, "Безупречная кредитная история")
	} else if b.HadPastDelinquency {
		factors = append(factors, "Наличие сведений о прошлых просрочках в БКИ")
	} else {
		factors = append(factors, "Наличие действующих кредитных обязательств")
	}

	return factors
}

func (e *Engine) generateBankFactors(z, pd float64, score int, zone domain.RiskZone, ratio float64, c CompanyInput, b BKIInput) []string {
	f := e.config.ScoringFormula
	w := f.Weights

	return []string{
		fmt.Sprintf("Ratio cap: %.1f, actual ratio: %.2f (weight: %.2f)", f.RatioCap, ratio, w.RevenueToLoanRatio),
		fmt.Sprintf("Business age: %d months (%.2f years, weight: %.2f)", c.BusinessAgeMonths, float64(c.BusinessAgeMonths)/12.0, w.BusinessAgeYears),
		fmt.Sprintf("BKI active loans count: %d (weight: %.2f)", b.ActiveLoansCount, w.BKIActiveLoansCount),
		fmt.Sprintf("BKI had past delinquency: %t (weight: %.2f)", b.HadPastDelinquency, w.BKIHadPastDelinquency),
		fmt.Sprintf("Calculated z-score: %.3f, PD: %.4f, Score: %d, Zone: %s", z, pd, score, zone),
	}
}
