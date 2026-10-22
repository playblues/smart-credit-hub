package app

import (
	"smart-credit-hub/internal/domain"
)

// AllowedTransitionsMap определяет строгую карту допустимых переходов между статусами заявки
// в точном соответствии со спецификацией docs/state-machine.md.
var AllowedTransitionsMap = map[domain.ApplicationStatus]map[domain.ApplicationStatus]struct{}{
	domain.StatusSubmitted: {
		domain.StatusEnrichment: {},
	},
	domain.StatusEnrichment: {
		domain.StatusScoring: {},
	},
	domain.StatusScoring: {
		domain.StatusAutoApproved:   {},
		domain.StatusCounterOffered: {},
		domain.StatusManualReview:   {},
		domain.StatusRejected:       {},
	},
	domain.StatusCounterOffered: {
		domain.StatusApproved:       {},
		domain.StatusClientDeclined: {},
	},
	domain.StatusManualReview: {
		domain.StatusWaitingDocuments: {},
		domain.StatusApproved:         {},
		domain.StatusCounterOffered:   {},
		domain.StatusRejected:         {},
	},
	domain.StatusWaitingDocuments: {
		domain.StatusManualReview: {},
	},
	domain.StatusAutoApproved: {
		domain.StatusSigned: {},
	},
	domain.StatusApproved: {
		domain.StatusSigned: {},
	},
	domain.StatusSigned: {
		domain.StatusDisbursed: {},
	},
	// Терминальные статусы без исходящих переходов
	domain.StatusRejected:       {},
	domain.StatusClientDeclined: {},
	domain.StatusDisbursed:      {},
}

// StateMachine реализует правила жизненного цикла кредитной заявки.
type StateMachine struct{}

// NewStateMachine создает новый экземпляр StateMachine.
func NewStateMachine() *StateMachine {
	return &StateMachine{}
}

// CanTransition проверяет, разрешен ли переход из from в to.
func (sm *StateMachine) CanTransition(from, to domain.ApplicationStatus) bool {
	allowedNext, exists := AllowedTransitionsMap[from]
	if !exists {
		return false
	}
	_, allowed := allowedNext[to]
	return allowed
}

// ValidateTransition проверяет допустимость перехода.
// Если переход запрещен, возвращает ошибку домена со статусом HTTP 409 Conflict (code: "CONFLICT_STATE").
func (sm *StateMachine) ValidateTransition(from, to domain.ApplicationStatus) error {
	if sm.CanTransition(from, to) {
		return nil
	}
	return domain.ErrInvalidStateTransition(from, to)
}

// IsTerminal проверяет, является ли статус конечным (завершающим жизненный цикл).
func (sm *StateMachine) IsTerminal(status domain.ApplicationStatus) bool {
	return status == domain.StatusRejected ||
		status == domain.StatusClientDeclined ||
		status == domain.StatusDisbursed
}

// AllowedTransitions возвращает список всех разрешенных следующих статусов для заданного статуса.
func (sm *StateMachine) AllowedTransitions(from domain.ApplicationStatus) []domain.ApplicationStatus {
	allowedMap, exists := AllowedTransitionsMap[from]
	if !exists {
		return nil
	}

	result := make([]domain.ApplicationStatus, 0, len(allowedMap))
	for s := range allowedMap {
		result = append(result, s)
	}
	return result
}
