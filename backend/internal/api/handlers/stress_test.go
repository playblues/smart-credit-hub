package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"smart-credit-hub/internal/domain"
)

// TestHTTPStress_DoubleAcceptCounterOffer проверяет защиту от Double-Click и Race Condition на уровне HTTP.
// 10 параллельных запросов на /api/v1/applications/{id}/counter-offer/accept
// Ожидаемый результат: ровно 1 запрос получает HTTP 200 OK, ровно 9 запросов получают HTTP 409 Conflict,
// итоговый статус заявки строго APPROVED.
func TestHTTPStress_DoubleAcceptCounterOffer(t *testing.T) {
	router, authSvc, appRepo, _, _ := setupActionsTestRouter(t)

	loginResp, err := authSvc.Authenticate(context.Background(), "client_alfa", "demo123")
	if err != nil {
		t.Fatalf("auth error: %v", err)
	}

	userID := "00000000-0000-0000-0000-000000000001"
	appID := "stress_counter_offer_1"

	// Создаем заявку в статусе COUNTER_OFFERED
	testApp := &domain.Application{
		ID:                  appID,
		UserID:              &userID,
		INN:                 "7701000001",
		Status:              domain.StatusCounterOffered,
		ProductType:         domain.ProductWorkingCapital,
		RequestedAmount:     10_000_000,
		RequestedTermMonths: 24,
		CounterOffer: &domain.CounterOffer{
			Amount:     7_000_000,
			TermMonths: 18,
			Rate:       0.185,
			PTI:        0.28,
			Reason:     "Тестовый контр-оффер",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := appRepo.Create(context.Background(), nil, testApp); err != nil {
		t.Fatalf("create app error: %v", err)
	}

	const concurrency = 10
	var successCount int64
	var conflictCount int64
	var otherCount int64

	startBarrier := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startBarrier // одновременный залп

			req := httptest.NewRequest(http.MethodPost, "/api/v1/applications/"+appID+"/counter-offer/accept", nil)
			req.Header.Set("Authorization", "Bearer "+loginResp.Token)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			switch rec.Code {
			case http.StatusOK:
				atomic.AddInt64(&successCount, 1)
			case http.StatusConflict:
				atomic.AddInt64(&conflictCount, 1)
			default:
				atomic.AddInt64(&otherCount, 1)
			}
		}()
	}

	close(startBarrier)
	wg.Wait()

	if successCount != 1 {
		t.Errorf("ожидался ровно 1 успешный ответ 200 OK, получено: %d", successCount)
	}
	if conflictCount != int64(concurrency-1) {
		t.Errorf("ожидалось %d ответов 409 Conflict, получено: %d", concurrency-1, conflictCount)
	}
	if otherCount != 0 {
		t.Errorf("неожиданные HTTP статусы (не 200 и не 409): %d", otherCount)
	}

	// Проверяем итоговое состояние в хранилище
	finalApp, err := appRepo.GetByID(context.Background(), appID)
	if err != nil {
		t.Fatalf("get app error: %v", err)
	}
	if finalApp.Status != domain.StatusApproved {
		t.Errorf("ожидался статус APPROVED, получен: %s", finalApp.Status)
	}
}

// TestHTTPStress_ConcurrentUnderwriterDecisions проверяет гонку андеррайтеров:
// 10 одновременных запросов с противоположными решениями (5 APPROVE, 5 REJECT) на /api/v1/bank/applications/{id}/decision
// Ожидаемый результат: ровно 1 запрос побеждает (HTTP 200), ровно 9 отклоняются с 409 Conflict.
func TestHTTPStress_ConcurrentUnderwriterDecisions(t *testing.T) {
	router, authSvc, appRepo, _, _ := setupActionsTestRouter(t)

	underwriterResp, err := authSvc.Authenticate(context.Background(), "underwriter", "demo123")
	if err != nil {
		t.Fatalf("auth error: %v", err)
	}

	appID := "stress_underwriter_race_1"
	testApp := &domain.Application{
		ID:                  appID,
		INN:                 "7701000002",
		Status:              domain.StatusManualReview,
		ProductType:         domain.ProductWorkingCapital,
		RequestedAmount:     15_000_000,
		RequestedTermMonths: 36,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	if err := appRepo.Create(context.Background(), nil, testApp); err != nil {
		t.Fatalf("create app error: %v", err)
	}

	const concurrency = 10
	var successCount int64
	var conflictCount int64
	var otherCount int64

	startBarrier := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		action := domain.ActionApprove
		if i%2 == 1 {
			action = domain.ActionReject
		}

		go func(act domain.DecisionAction) {
			defer wg.Done()
			<-startBarrier // одновременный залп

			comment := "Стресс-тест гонки андеррайтеров"
			bodyBytes, _ := json.Marshal(domain.UnderwriterDecisionRequest{
				Action:  act,
				Comment: &comment,
			})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/bank/applications/"+appID+"/decision", bytes.NewReader(bodyBytes))
			req.Header.Set("Authorization", "Bearer "+underwriterResp.Token)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			switch rec.Code {
			case http.StatusOK:
				atomic.AddInt64(&successCount, 1)
			case http.StatusConflict:
				atomic.AddInt64(&conflictCount, 1)
			default:
				atomic.AddInt64(&otherCount, 1)
			}
		}(action)
	}

	close(startBarrier)
	wg.Wait()

	if successCount != 1 {
		t.Errorf("ожидался ровно 1 успешный ответ 200 OK, получено: %d", successCount)
	}
	if conflictCount != int64(concurrency-1) {
		t.Errorf("ожидалось %d ответов 409 Conflict, получено: %d", concurrency-1, conflictCount)
	}
	if otherCount != 0 {
		t.Errorf("неожиданные HTTP статусы: %d", otherCount)
	}

	// Итоговый статус должен быть либо APPROVED, либо REJECTED, но никак не остаться MANUAL_REVIEW
	finalApp, err := appRepo.GetByID(context.Background(), appID)
	if err != nil {
		t.Fatalf("get app error: %v", err)
	}
	if finalApp.Status != domain.StatusApproved && finalApp.Status != domain.StatusRejected {
		t.Errorf("недопустимый финальный статус заявки: %s", finalApp.Status)
	}
}

// TestHTTPStress_DoubleSignDisburse проверяет защиту от повторного подписания и двойной выдачи средств.
// 10 одновременных вызовов /api/v1/applications/{id}/sign на заявке в статусе APPROVED
// Ожидаемый результат: ровно 1 запрос возвращает 200 OK и переводит заявку в SIGNED (и далее DISBURSED),
// 9 запросов возвращают 409 Conflict.
func TestHTTPStress_DoubleSignDisburse(t *testing.T) {
	router, authSvc, appRepo, _, _ := setupActionsTestRouter(t)

	loginResp, err := authSvc.Authenticate(context.Background(), "client_alfa", "demo123")
	if err != nil {
		t.Fatalf("auth error: %v", err)
	}

	userID := "00000000-0000-0000-0000-000000000001"
	appID := "stress_sign_race_1"

	amt := 5_000_000.0
	term := 12
	rate := 0.165

	testApp := &domain.Application{
		ID:                  appID,
		UserID:              &userID,
		INN:                 "7701000003",
		Status:              domain.StatusApproved,
		ProductType:         domain.ProductWorkingCapital,
		RequestedAmount:     5_000_000,
		RequestedTermMonths: 12,
		ApprovedAmount:      &amt,
		ApprovedTermMonths:  &term,
		InterestRate:        &rate,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	if err := appRepo.Create(context.Background(), nil, testApp); err != nil {
		t.Fatalf("create app error: %v", err)
	}

	const concurrency = 10
	var successCount int64
	var conflictCount int64
	var otherCount int64

	startBarrier := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startBarrier

			req := httptest.NewRequest(http.MethodPost, "/api/v1/applications/"+appID+"/sign", nil)
			req.Header.Set("Authorization", "Bearer "+loginResp.Token)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			switch rec.Code {
			case http.StatusOK:
				atomic.AddInt64(&successCount, 1)
			case http.StatusConflict:
				atomic.AddInt64(&conflictCount, 1)
			default:
				atomic.AddInt64(&otherCount, 1)
			}
		}()
	}

	close(startBarrier)
	wg.Wait()

	if successCount != 1 {
		t.Errorf("ожидался ровно 1 успешный ответ 200 OK, получено: %d", successCount)
	}
	if conflictCount != int64(concurrency-1) {
		t.Errorf("ожидалось %d ответов 409 Conflict, получено: %d", concurrency-1, conflictCount)
	}
	if otherCount != 0 {
		t.Errorf("неожиданные HTTP статусы: %d", otherCount)
	}

	// Даем 50мс для фоновой выдачи средств (disburse delay = 0)
	time.Sleep(50 * time.Millisecond)

	finalApp, err := appRepo.GetByID(context.Background(), appID)
	if err != nil {
		t.Fatalf("get app error: %v", err)
	}
	// Финальный статус должен быть DISBURSED
	if finalApp.Status != domain.StatusDisbursed {
		t.Errorf("ожидался финальный статус DISBURSED, получен: %s", finalApp.Status)
	}
}
