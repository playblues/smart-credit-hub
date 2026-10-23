package app

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"smart-credit-hub/internal/domain"
	"smart-credit-hub/internal/mock"
	"smart-credit-hub/internal/scoring"
	"smart-credit-hub/internal/storage"
)

// PipelineService реализует фоновый асинхронный процесс кредитного конвейера.
type PipelineService struct {
	appRepo       storage.ApplicationRepository
	mockService   mock.Service
	scoringEngine *scoring.Engine
	stateMachine  *StateMachine
	pool          *pgxpool.Pool
	stepDelay     time.Duration
}

// NewPipelineService создает сервис кредитного конвейера.
func NewPipelineService(
	appRepo storage.ApplicationRepository,
	mockService mock.Service,
	scoringEngine *scoring.Engine,
	pool *pgxpool.Pool,
) *PipelineService {
	return &PipelineService{
		appRepo:       appRepo,
		mockService:   mockService,
		scoringEngine: scoringEngine,
		stateMachine:  NewStateMachine(),
		pool:          pool,
		stepDelay:     1500 * time.Millisecond,
	}
}

// SetStepDelay переопределяет задержку между шагами конвейера (для быстрых юнит-тестов).
func (s *PipelineService) SetStepDelay(d time.Duration) {
	s.stepDelay = d
}

// StartProcessing запускает конвейер обработки заявки в фоновой горутине с защитой от паник.
func (s *PipelineService) StartProcessing(appID string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("КРИТИЧЕСКАЯ ПАНИКА в кредитном конвейере",
					slog.String("application_id", appID),
					slog.Any("panic", r),
					slog.String("stack", string(debug.Stack())),
				)

				failComment := fmt.Sprintf("Критический сбой обработки: %v", r)
				fromStatus := domain.StatusSubmitted
				if currApp, err := s.appRepo.GetByID(context.Background(), appID); err == nil && currApp != nil {
					fromStatus = currApp.Status
				}

				// Фиксация ошибки в статусе заявки
				_ = s.appRepo.UpdateStatus(
					context.Background(),
					nil,
					appID,
					fromStatus,
					domain.StatusRejected,
					"SYSTEM",
					&failComment,
					nil,
				)
			}
		}()

		s.process(context.Background(), appID)
	}()
}

// ProcessSync выполняет обработку синхронно (для использования в тестах).
func (s *PipelineService) ProcessSync(ctx context.Context, appID string) error {
	return s.process(ctx, appID)
}

func (s *PipelineService) process(ctx context.Context, appID string) error {
	slog.Info("Запуск кредитного конвейера для заявки", slog.String("app_id", appID))

	// Загрузка начального состояния заявки
	app, err := s.appRepo.GetByID(ctx, appID)
	if err != nil {
		slog.Error("Не удалось загрузить заявку для конвейера", "app_id", appID, "error", err)
		return err
	}

	if app.Status != domain.StatusSubmitted {
		slog.Warn("Заявка уже не в статусе SUBMITTED, конвейер прерван", "app_id", appID, "status", app.Status)
		return nil
	}

	// ----------------------------------------------------
	// ШАГ 1: SUBMITTED -> ENRICHMENT (Обогащение данными ФНС и БКИ)
	// ----------------------------------------------------
	if s.stepDelay > 0 {
		time.Sleep(s.stepDelay)
	}

	if err := s.stateMachine.ValidateTransition(app.Status, domain.StatusEnrichment); err != nil {
		slog.Error("Недопустимый переход в ENRICHMENT", "app_id", appID, "error", err)
		return err
	}

	// Запрос данных ФНС и БКИ
	fnsComp, fnsErr := s.mockService.GetFNSByINN(ctx, app.INN)
	if fnsErr != nil {
		slog.Warn("Данные ФНС не найдены для ИНН", "inn", app.INN, "error", fnsErr)
	}

	bkiHist, bkiErr := s.mockService.GetBKIByINN(ctx, app.INN)
	if bkiErr != nil {
		slog.Warn("Данные БКИ не найдены для ИНН", "inn", app.INN, "error", bkiErr)
	}

	snapshot := &domain.CompanySnapshot{
		INN: app.INN,
	}
	if fnsComp != nil {
		snapshot.Name = fnsComp.Name
		snapshot.AnnualRevenue = fnsComp.AnnualRevenue
		snapshot.BusinessAgeMonths = fnsComp.BusinessAgeMonths
		snapshot.IsLiquidating = fnsComp.IsLiquidating
	}
	if bkiHist != nil {
		snapshot.BKIActiveLoans = bkiHist.ActiveLoansCount
		snapshot.BKIMaxOverdueDays = bkiHist.MaxOverdueDays
		snapshot.BKIHadPastDelinquency = bkiHist.HadPastDelinquency
	}

	enrichmentComment := "Запрос данных в ФНС и БКИ"
	err = s.appRepo.UpdateStatus(
		ctx,
		nil,
		appID,
		domain.StatusSubmitted,
		domain.StatusEnrichment,
		"SYSTEM",
		&enrichmentComment,
		&domain.ApplicationUpdateFields{
			CompanySnapshot: snapshot,
		},
	)
	if err != nil {
		slog.Error("Ошибка обновления статуса на ENRICHMENT", "app_id", appID, "error", err)
		return err
	}

	// ----------------------------------------------------
	// ШАГ 2: ENRICHMENT -> SCORING (Скоринг и оценка риска)
	// ----------------------------------------------------
	if s.stepDelay > 0 {
		time.Sleep(s.stepDelay)
	}

	if err := s.stateMachine.ValidateTransition(domain.StatusEnrichment, domain.StatusScoring); err != nil {
		slog.Error("Недопустимый переход в SCORING", "app_id", appID, "error", err)
		return err
	}

	scoringComment := "Скоринг и оценка кредитного риска"
	err = s.appRepo.UpdateStatus(
		ctx,
		nil,
		appID,
		domain.StatusEnrichment,
		domain.StatusScoring,
		"SYSTEM",
		&scoringComment,
		nil,
	)
	if err != nil {
		slog.Error("Ошибка обновления статуса на SCORING", "app_id", appID, "error", err)
		return err
	}

	// ----------------------------------------------------
	// ШАГ 3: Расчет скоринга и перевод в целевой статус
	// ----------------------------------------------------
	purposeStr := ""
	if app.Purpose != nil {
		purposeStr = *app.Purpose
	}

	scoringInput := scoring.ScoringInput{
		Company: scoring.CompanyInput{
			INN:               snapshot.INN,
			Name:              snapshot.Name,
			AnnualRevenue:     snapshot.AnnualRevenue,
			BusinessAgeMonths: snapshot.BusinessAgeMonths,
			IsLiquidating:     snapshot.IsLiquidating,
		},
		BKI: scoring.BKIInput{
			ActiveLoansCount:   snapshot.BKIActiveLoans,
			MaxOverdueDays:     snapshot.BKIMaxOverdueDays,
			HadPastDelinquency: snapshot.BKIHadPastDelinquency,
		},
		Request: scoring.RequestInput{
			ProductType:         app.ProductType,
			RequestedAmount:     app.RequestedAmount,
			RequestedTermMonths: app.RequestedTermMonths,
			Purpose:             purposeStr,
		},
	}

	scoreOut := s.scoringEngine.Evaluate(scoringInput)

	scoringResult := &domain.ScoringResult{
		RiskZone:      scoreOut.RiskZone,
		StopFactor:    scoreOut.StopFactor,
		FactorsClient: scoreOut.FactorsClient,
		FactorsBank:   scoreOut.FactorsBank,
		ModelVersion:  scoreOut.ModelVersion,
	}
	if scoreOut.ZScore != nil {
		scoringResult.ZScore = *scoreOut.ZScore
	}
	if scoreOut.PD != nil {
		scoringResult.PD = *scoreOut.PD
	}
	if scoreOut.Score != nil {
		scoringResult.Score = *scoreOut.Score
	}

	finalFields := &domain.ApplicationUpdateFields{
		ScoringResult:   scoringResult,
		CompanySnapshot: snapshot,
	}

	var finalComment string
	switch scoreOut.Status {
	case domain.StatusAutoApproved:
		finalFields.ApprovedAmount = scoreOut.ApprovedAmount
		finalFields.ApprovedTermMonths = scoreOut.ApprovedTermMonths
		finalFields.InterestRate = scoreOut.InterestRate
		finalComment = "Заявка автоматически одобрена скоринговой моделью"

	case domain.StatusCounterOffered:
		if scoreOut.CounterOffer != nil {
			finalFields.CounterOffer = &domain.CounterOffer{
				Amount:     scoreOut.CounterOffer.Amount,
				TermMonths: scoreOut.CounterOffer.TermMonths,
				Rate:       scoreOut.CounterOffer.Rate,
				PTI:        scoreOut.CounterOffer.PTI,
				Reason:     scoreOut.CounterOffer.Reason,
			}
		}
		finalComment = "Сформировано встречное предложение (контр-оффер)"

	case domain.StatusManualReview:
		finalComment = "Заявка передана на ручное рассмотрение андеррайтеру"

	case domain.StatusRejected:
		finalFields.RejectReasonClient = scoreOut.ReasonClient
		finalComment = "Отказ в предоставлении кредита по результатам скоринга"

	default:
		finalComment = fmt.Sprintf("Завершение скоринга со статусом %s", scoreOut.Status)
	}

	if err := s.stateMachine.ValidateTransition(domain.StatusScoring, scoreOut.Status); err != nil {
		slog.Error("Недопустимый переход из SCORING", "app_id", appID, "target_status", scoreOut.Status, "error", err)
		return err
	}

	err = s.appRepo.UpdateStatus(
		ctx,
		nil,
		appID,
		domain.StatusScoring,
		scoreOut.Status,
		"SYSTEM",
		&finalComment,
		finalFields,
	)
	if err != nil {
		slog.Error("Ошибка обновления финального статуса", "app_id", appID, "status", scoreOut.Status, "error", err)
		return err
	}

	slog.Info("Кредитный конвейер успешно завершил обработку заявки",
		slog.String("app_id", appID),
		slog.String("final_status", string(scoreOut.Status)),
		slog.String("risk_zone", string(scoreOut.RiskZone)),
	)

	return nil
}
