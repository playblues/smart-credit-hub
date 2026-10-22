package app_test

import (
	"testing"

	"smart-credit-hub/internal/app"
	"smart-credit-hub/internal/domain"
)

func TestStateMachine_ValidTransitions(t *testing.T) {
	sm := app.NewStateMachine()

	validCases := [][2]domain.ApplicationStatus{
		{domain.StatusSubmitted, domain.StatusEnrichment},
		{domain.StatusEnrichment, domain.StatusScoring},
		{domain.StatusScoring, domain.StatusAutoApproved},
		{domain.StatusScoring, domain.StatusCounterOffered},
		{domain.StatusScoring, domain.StatusManualReview},
		{domain.StatusScoring, domain.StatusRejected},
		{domain.StatusCounterOffered, domain.StatusApproved},
		{domain.StatusCounterOffered, domain.StatusClientDeclined},
		{domain.StatusManualReview, domain.StatusWaitingDocuments},
		{domain.StatusWaitingDocuments, domain.StatusManualReview},
		{domain.StatusManualReview, domain.StatusApproved},
		{domain.StatusManualReview, domain.StatusCounterOffered},
		{domain.StatusManualReview, domain.StatusRejected},
		{domain.StatusAutoApproved, domain.StatusSigned},
		{domain.StatusApproved, domain.StatusSigned},
		{domain.StatusSigned, domain.StatusDisbursed},
	}

	for _, tc := range validCases {
		from, to := tc[0], tc[1]
		if err := sm.ValidateTransition(from, to); err != nil {
			t.Errorf("переход из %s в %s должен быть разрешен, получена ошибка: %v", from, to, err)
		}
		if !sm.CanTransition(from, to) {
			t.Errorf("CanTransition(%s, %s) вернул false", from, to)
		}
	}
}

func TestStateMachine_InvalidTransitions(t *testing.T) {
	sm := app.NewStateMachine()

	invalidCases := [][2]domain.ApplicationStatus{
		{domain.StatusSubmitted, domain.StatusApproved},
		{domain.StatusSubmitted, domain.StatusSigned},
		{domain.StatusEnrichment, domain.StatusAutoApproved},
		{domain.StatusRejected, domain.StatusApproved},
		{domain.StatusRejected, domain.StatusSubmitted},
		{domain.StatusClientDeclined, domain.StatusSigned},
		{domain.StatusDisbursed, domain.StatusSubmitted},
		{domain.StatusSigned, domain.StatusApproved},
		{domain.StatusAutoApproved, domain.StatusDisbursed}, // только через SIGNED
	}

	for _, tc := range invalidCases {
		from, to := tc[0], tc[1]
		err := sm.ValidateTransition(from, to)
		if err == nil {
			t.Errorf("переход из %s в %s должен быть запрещен, но ошибка не получена", from, to)
		}

		appErr, ok := err.(*domain.AppError)
		if !ok {
			t.Fatalf("ожидался *domain.AppError, получен: %T", err)
		}
		if appErr.HTTPStatus != 409 {
			t.Errorf("ожидался HTTP статус 409 Conflict, получен: %d", appErr.HTTPStatus)
		}
		if appErr.Code != domain.ErrCodeConflictState {
			t.Errorf("ожидался код ошибки CONFLICT_STATE, получен: %s", appErr.Code)
		}
	}
}

func TestStateMachine_TerminalStatuses(t *testing.T) {
	sm := app.NewStateMachine()

	terminal := []domain.ApplicationStatus{
		domain.StatusRejected,
		domain.StatusClientDeclined,
		domain.StatusDisbursed,
	}

	for _, s := range terminal {
		if !sm.IsTerminal(s) {
			t.Errorf("статус %s должен быть терминальным", s)
		}
		if len(sm.AllowedTransitions(s)) != 0 {
			t.Errorf("терминальный статус %s не должен иметь исходящих переходов", s)
		}
	}

	nonTerminal := []domain.ApplicationStatus{
		domain.StatusSubmitted,
		domain.StatusEnrichment,
		domain.StatusScoring,
		domain.StatusManualReview,
		domain.StatusSigned,
	}

	for _, s := range nonTerminal {
		if sm.IsTerminal(s) {
			t.Errorf("статус %s не должен быть терминальным", s)
		}
	}
}
