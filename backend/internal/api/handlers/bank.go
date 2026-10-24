package handlers

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"smart-credit-hub/internal/app"
	"smart-credit-hub/internal/domain"
	"smart-credit-hub/internal/scoring"
	"smart-credit-hub/internal/storage"
)

// BankHandler инкапсулирует обработчики АРМ Андеррайтера.
type BankHandler struct {
	appRepo      storage.ApplicationRepository
	stateMachine *app.StateMachine
	scoringModel *scoring.ModelConfig
}

// NewBankHandler создает новый экземпляр BankHandler.
func NewBankHandler(appRepo storage.ApplicationRepository, scoringModel *scoring.ModelConfig) *BankHandler {
	return &BankHandler{
		appRepo:      appRepo,
		stateMachine: app.NewStateMachine(),
		scoringModel: scoringModel,
	}
}

// ListApplications обрабатывает GET /api/v1/bank/applications.
// Возвращает очередь заявок для андеррайтера с опциональной фильтрацией по status и risk_zone.
func (h *BankHandler) ListApplications(w http.ResponseWriter, r *http.Request) {
	statusParam := strings.TrimSpace(r.URL.Query().Get("status"))
	riskZoneParam := strings.TrimSpace(r.URL.Query().Get("risk_zone"))

	var statusFilter *domain.ApplicationStatus
	if statusParam != "" {
		st := domain.ApplicationStatus(statusParam)
		statusFilter = &st
	}

	var riskZoneFilter *domain.RiskZone
	if riskZoneParam != "" {
		rz := domain.RiskZone(riskZoneParam)
		riskZoneFilter = &rz
	}

	apps, err := h.appRepo.ListAll(r.Context(), statusFilter, riskZoneFilter)
	if err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	if apps == nil {
		apps = make([]*domain.Application, 0)
	}

	domain.RespondWithJSON(w, http.StatusOK, apps)
}

// GetApplicationByID обрабатывает GET /api/v1/bank/applications/{id}.
// Возвращает полную карточку заявки для андеррайтера.
func (h *BankHandler) GetApplicationByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		domain.RespondWithError(
			w,
			http.StatusBadRequest,
			domain.ErrCodeBadRequest,
			"Идентификатор заявки не указан",
			nil,
		)
		return
	}

	application, err := h.appRepo.GetByID(r.Context(), id)
	if err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	domain.RespondWithJSON(w, http.StatusOK, application)
}

// Decision обрабатывает POST /api/v1/bank/applications/{id}/decision.
// Выносит решение андеррайтера по заявке в статусе MANUAL_REVIEW.
func (h *BankHandler) Decision(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		domain.RespondWithError(
			w,
			http.StatusBadRequest,
			domain.ErrCodeBadRequest,
			"Идентификатор заявки не указан",
			nil,
		)
		return
	}

	var req domain.UnderwriterDecisionRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		domain.RespondWithError(
			w,
			http.StatusBadRequest,
			domain.ErrCodeBadRequest,
			"Некорректный JSON в теле запроса",
			domain.StringPtr(err.Error()),
		)
		return
	}

	// Загружаем текущее состояние заявки
	application, err := h.appRepo.GetByID(r.Context(), id)
	if err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	// Решение андеррайтера допустимо ТОЛЬКО из статуса MANUAL_REVIEW
	if application.Status != domain.StatusManualReview {
		domain.RespondWithError(
			w,
			http.StatusConflict,
			domain.ErrCodeConflictState,
			"Решение андеррайтера применимо только к заявкам в статусе MANUAL_REVIEW",
			nil,
		)
		return
	}

	var targetStatus domain.ApplicationStatus
	extra := &domain.ApplicationUpdateFields{}

	baseRate := 0.185
	if h.scoringModel != nil {
		if prod, ok := h.scoringModel.Products[string(application.ProductType)]; ok {
			baseRate = prod.BaseRate
		}
	}

	switch req.Action {
	case domain.ActionApprove:
		targetStatus = domain.StatusApproved
		extra.ApprovedAmount = &application.RequestedAmount
		extra.ApprovedTermMonths = &application.RequestedTermMonths
		extra.InterestRate = &baseRate
		if req.Comment != nil {
			extra.UnderwriterComment = req.Comment
		}

	case domain.ActionReject:
		targetStatus = domain.StatusRejected
		if req.Comment == nil || strings.TrimSpace(*req.Comment) == "" {
			domain.RespondWithError(
				w,
				http.StatusBadRequest,
				domain.ErrCodeBadRequest,
				"Поле comment обязательно при отклонении заявки (REJECT)",
				nil,
			)
			return
		}
		extra.RejectReasonClient = req.Comment
		extra.UnderwriterComment = req.Comment

	case domain.ActionRequestDocuments:
		targetStatus = domain.StatusWaitingDocuments
		if req.Comment == nil || strings.TrimSpace(*req.Comment) == "" {
			domain.RespondWithError(
				w,
				http.StatusBadRequest,
				domain.ErrCodeBadRequest,
				"Поле comment обязательно при запросе документов (REQUEST_DOCUMENTS)",
				nil,
			)
			return
		}
		extra.UnderwriterComment = req.Comment

	case domain.ActionCounterOffer:
		targetStatus = domain.StatusCounterOffered
		if req.CounterOfferAmount == nil && req.CounterOffer != nil && req.CounterOffer.Amount > 0 {
			req.CounterOfferAmount = &req.CounterOffer.Amount
		}
		if req.CounterOfferTermMonths == nil && req.CounterOffer != nil && req.CounterOffer.TermMonths > 0 {
			req.CounterOfferTermMonths = &req.CounterOffer.TermMonths
		}
		if req.CounterOfferAmount == nil || *req.CounterOfferAmount <= 0 {
			domain.RespondWithError(
				w,
				http.StatusBadRequest,
				domain.ErrCodeBadRequest,
				"Поле counter_offer_amount обязательно и должно быть больше нуля",
				nil,
			)
			return
		}
		if req.CounterOfferTermMonths == nil || *req.CounterOfferTermMonths <= 0 {
			domain.RespondWithError(
				w,
				http.StatusBadRequest,
				domain.ErrCodeBadRequest,
				"Поле counter_offer_term_months обязательно и должно быть больше нуля",
				nil,
			)
			return
		}

		revenue := 0.0
		if application.CompanySnapshot != nil {
			revenue = application.CompanySnapshot.AnnualRevenue
		}

		rate := baseRate
		pti := scoring.PTI(*req.CounterOfferAmount, rate, *req.CounterOfferTermMonths, revenue)
		ptiRounded := math.Round(pti*10000.0) / 10000.0

		reason := "Индивидуальные условия, предложенные андеррайтером"
		if req.Comment != nil && strings.TrimSpace(*req.Comment) != "" {
			reason = strings.TrimSpace(*req.Comment)
		}

		extra.CounterOffer = &domain.CounterOffer{
			Amount:     *req.CounterOfferAmount,
			TermMonths: *req.CounterOfferTermMonths,
			Rate:       rate,
			PTI:        ptiRounded,
			Reason:     reason,
		}
		extra.UnderwriterComment = req.Comment

	default:
		domain.RespondWithError(
			w,
			http.StatusBadRequest,
			domain.ErrCodeBadRequest,
			"Недопустимое действие action (ожидается: APPROVE, REJECT, REQUEST_DOCUMENTS, COUNTER_OFFER)",
			nil,
		)
		return
	}

	// Валидация перехода через State Machine
	if err := h.stateMachine.ValidateTransition(application.Status, targetStatus); err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	auditComment := req.Comment
	if auditComment == nil {
		defComment := string(req.Action)
		auditComment = &defComment
	}

	err = h.appRepo.UpdateStatus(
		r.Context(),
		nil,
		id,
		application.Status,
		targetStatus,
		"UNDERWRITER",
		auditComment,
		extra,
	)
	if err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	updatedApp, err := h.appRepo.GetByID(r.Context(), id)
	if err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	domain.RespondWithJSON(w, http.StatusOK, updatedApp)
}
