package app_test

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"smart-credit-hub/internal/app"
	"smart-credit-hub/internal/domain"
	"smart-credit-hub/internal/storage"
)

// TestConcurrency_OptimisticLocking_StatusUpdate тестирует защиту от состояния гонки (Race Condition / Double Submission)
// при одновременной попытке 10 параллельных горутин перевести статус заявки.
func TestConcurrency_OptimisticLocking_StatusUpdate(t *testing.T) {
	repo := storage.NewInMemoryApplicationRepository()
	appID := "app_concurrent_race_1"

	userID := "user_100"
	initialApp := &domain.Application{
		ID:                  appID,
		UserID:              &userID,
		INN:                 "7701000002",
		Status:              domain.StatusCounterOffered,
		ProductType:         domain.ProductWorkingCapital,
		RequestedAmount:     5000000,
		RequestedTermMonths: 24,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
		CounterOffer: &domain.CounterOffer{
			Amount:     3250000,
			TermMonths: 24,
			Rate:       0.205,
			PTI:        0.249,
		},
	}

	if err := repo.Create(context.Background(), nil, initialApp); err != nil {
		t.Fatalf("ошибка создания начальной заявки: %v", err)
	}

	const concurrency = 10
	var successCount int64
	var conflictCount int64
	var otherErrorCount int64

	startBarrier := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(workerID int) {
			defer wg.Done()
			<-startBarrier // одновременный старт всех 10 горутин

			comment := "Принятие встречного предложения клиентом"
			extra := &domain.ApplicationUpdateFields{
				ApprovedAmount:     &initialApp.CounterOffer.Amount,
				ApprovedTermMonths: &initialApp.CounterOffer.TermMonths,
				InterestRate:       &initialApp.CounterOffer.Rate,
			}

			err := repo.UpdateStatus(
				context.Background(),
				nil,
				appID,
				domain.StatusCounterOffered,
				domain.StatusApproved,
				"CLIENT",
				&comment,
				extra,
			)

			if err == nil {
				atomic.AddInt64(&successCount, 1)
			} else {
				if appErr, ok := err.(*domain.AppError); ok && appErr.HTTPStatus == http.StatusConflict && appErr.Code == domain.ErrCodeConflictState {
					atomic.AddInt64(&conflictCount, 1)
				} else {
					atomic.AddInt64(&otherErrorCount, 1)
					t.Errorf("worker %d получил неожиданную ошибку: %v", workerID, err)
				}
			}
		}(i)
	}

	close(startBarrier)
	wg.Wait()

	// Проверка: ровно одна горутина должна была победить в гонке
	if successCount != 1 {
		t.Fatalf("Ожидался ровно 1 успешный переход, получено успехов: %d", successCount)
	}
	if conflictCount != int64(concurrency-1) {
		t.Fatalf("Ожидалось %d конфликтов (409), получено: %d", concurrency-1, conflictCount)
	}
	if otherErrorCount != 0 {
		t.Fatalf("Получены непредвиденные ошибки: %d", otherErrorCount)
	}

	// Проверка целостности данных в репозитории
	updatedApp, err := repo.GetByID(context.Background(), appID)
	if err != nil {
		t.Fatalf("ошибка получения заявки: %v", err)
	}

	if updatedApp.Status != domain.StatusApproved {
		t.Errorf("Итоговый статус заявки не совпадает: получено %s, ожидалось %s", updatedApp.Status, domain.StatusApproved)
	}

	// В аудит логе должно быть ровно 2 события: создание заявки и ровно 1 переход
	if len(updatedApp.StatusHistory) != 2 {
		t.Errorf("Ожидалось ровно 2 записи аудита (Create + 1 Update), получено: %d", len(updatedApp.StatusHistory))
	}
}

// TestConcurrency_UnderwriterDecisionRace проверяет конкурентное вынесение разных решений андеррайтерами.
func TestConcurrency_UnderwriterDecisionRace(t *testing.T) {
	repo := storage.NewInMemoryApplicationRepository()
	appID := "app_underwriter_race_2"

	userID := "user_200"
	initialApp := &domain.Application{
		ID:                  appID,
		UserID:              &userID,
		INN:                 "7701000003",
		Status:              domain.StatusManualReview,
		ProductType:         domain.ProductOverdraft,
		RequestedAmount:     2500000,
		RequestedTermMonths: 12,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	if err := repo.Create(context.Background(), nil, initialApp); err != nil {
		t.Fatalf("ошибка создания заявки: %v", err)
	}

	targetDecisions := []domain.ApplicationStatus{
		domain.StatusApproved,
		domain.StatusRejected,
		domain.StatusWaitingDocuments,
		domain.StatusCounterOffered,
	}

	const concurrency = 12
	var successCount int64
	var conflictCount int64

	startBarrier := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		targetStatus := targetDecisions[i%len(targetDecisions)]
		go func(status domain.ApplicationStatus) {
			defer wg.Done()
			<-startBarrier

			comment := "Решение андеррайтера"
			err := repo.UpdateStatus(
				context.Background(),
				nil,
				appID,
				domain.StatusManualReview,
				status,
				"UNDERWRITER",
				&comment,
				nil,
			)

			if err == nil {
				atomic.AddInt64(&successCount, 1)
			} else {
				if appErr, ok := err.(*domain.AppError); ok && appErr.HTTPStatus == http.StatusConflict {
					atomic.AddInt64(&conflictCount, 1)
				}
			}
		}(targetStatus)
	}

	close(startBarrier)
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("Ожидался ровно 1 успешный вердикт андеррайтера, зафиксировано: %d", successCount)
	}
	if conflictCount != int64(concurrency-1) {
		t.Fatalf("Ожидалось %d конфликтов при конкурентных решениях, получено: %d", concurrency-1, conflictCount)
	}
}

// TestStateMachine_AllInvalidTransitionsMatrix тестирует полную матрицу 12x12 переходов.
// Все запрещенные комбинации обязаны возвращать HTTP 409 CONFLICT_STATE.
func TestStateMachine_AllInvalidTransitionsMatrix(t *testing.T) {
	sm := app.NewStateMachine()

	allStatuses := []domain.ApplicationStatus{
		domain.StatusSubmitted,
		domain.StatusEnrichment,
		domain.StatusScoring,
		domain.StatusAutoApproved,
		domain.StatusCounterOffered,
		domain.StatusManualReview,
		domain.StatusWaitingDocuments,
		domain.StatusApproved,
		domain.StatusRejected,
		domain.StatusClientDeclined,
		domain.StatusSigned,
		domain.StatusDisbursed,
	}

	var invalidChecked int
	var validChecked int

	for _, from := range allStatuses {
		for _, to := range allStatuses {
			allowed := sm.CanTransition(from, to)
			err := sm.ValidateTransition(from, to)

			if allowed {
				validChecked++
				if err != nil {
					t.Errorf("Переход [%s -> %s] помечен как разрешенный, но ValidateTransition вернул ошибку: %v", from, to, err)
				}
			} else {
				invalidChecked++
				if err == nil {
					t.Errorf("Переход [%s -> %s] запрещен, но ValidateTransition не вернул ошибку", from, to)
					continue
				}

				appErr, ok := err.(*domain.AppError)
				if !ok {
					t.Errorf("Ожидался *domain.AppError, получено: %T", err)
					continue
				}

				if appErr.HTTPStatus != http.StatusConflict {
					t.Errorf("[%s -> %s] Ожидался HTTP статус 409 Conflict, получено: %d", from, to, appErr.HTTPStatus)
				}
				if appErr.Code != domain.ErrCodeConflictState {
					t.Errorf("[%s -> %s] Ожидался код CONFLICT_STATE, получено: %s", from, to, appErr.Code)
				}
			}
		}
	}

	// Согласно state-machine.md, допустимых переходов ровно 16
	if validChecked != 16 {
		t.Errorf("Ожидалось ровно 16 валидных переходов в матрице, найдено: %d", validChecked)
	}
	// Всего комбинаций 144, невалидных должно быть ровно 144 - 16 = 128
	if invalidChecked != (len(allStatuses)*len(allStatuses) - 16) {
		t.Errorf("Ожидалось 128 невалидных переходов, проверено: %d", invalidChecked)
	}

	// Отдельная проверка терминальных статусов:
	terminalStatuses := []domain.ApplicationStatus{
		domain.StatusRejected,
		domain.StatusClientDeclined,
		domain.StatusDisbursed,
	}

	for _, term := range terminalStatuses {
		if !sm.IsTerminal(term) {
			t.Errorf("Статус %s обязан определяться как терминальный", term)
		}
		for _, target := range allStatuses {
			if sm.CanTransition(term, target) {
				t.Errorf("Терминальный статус %s не имеет права переходить в %s", term, target)
			}
		}
	}
}
