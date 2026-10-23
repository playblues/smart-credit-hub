package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"smart-credit-hub/internal/api/middleware"
	"smart-credit-hub/internal/app"
	"smart-credit-hub/internal/domain"
	"smart-credit-hub/internal/storage"
)

// ClientActionsHandler обрабатывает действия клиента по заявке.
type ClientActionsHandler struct {
	appRepo       storage.ApplicationRepository
	stateMachine  *app.StateMachine
	uploadDir     string
	disburseDelay time.Duration
}

// NewClientActionsHandler создает новый экземпляр ClientActionsHandler.
func NewClientActionsHandler(appRepo storage.ApplicationRepository, uploadDir string) *ClientActionsHandler {
	if uploadDir == "" {
		uploadDir = "uploads"
	}
	return &ClientActionsHandler{
		appRepo:       appRepo,
		stateMachine:  app.NewStateMachine(),
		uploadDir:     uploadDir,
		disburseDelay: 2 * time.Second,
	}
}

// SetDisburseDelay переопределяет задержку автоматического зачисления средств (для тестов).
func (h *ClientActionsHandler) SetDisburseDelay(d time.Duration) {
	h.disburseDelay = d
}

// checkClientOwnership проверяет принадлежность заявки текущему авторизованному клиенту.
func (h *ClientActionsHandler) checkClientOwnership(r *http.Request, app *domain.Application) error {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok || claims == nil {
		return domain.ErrUnauthorized(nil)
	}

	if claims.Role == domain.RoleClient {
		if app.UserID == nil || *app.UserID != claims.UserID {
			return domain.ErrForbidden(domain.StringPtr("Доступ к чужой кредитной заявке запрещен"))
		}
	}

	return nil
}

// AcceptCounterOffer обрабатывает POST /api/v1/applications/{id}/counter-offer/accept.
// Клиент принимает встречное предложение (COUNTER_OFFERED -> APPROVED).
func (h *ClientActionsHandler) AcceptCounterOffer(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	application, err := h.appRepo.GetByID(r.Context(), id)
	if err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	if err := h.checkClientOwnership(r, application); err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	if application.Status != domain.StatusCounterOffered {
		domain.RespondWithError(
			w,
			http.StatusConflict,
			domain.ErrCodeConflictState,
			"Принятие контр-оффера допустимо только для заявок в статусе COUNTER_OFFERED",
			nil,
		)
		return
	}

	if application.CounterOffer == nil {
		domain.RespondWithError(
			w,
			http.StatusConflict,
			domain.ErrCodeConflictState,
			"В заявке отсутствуют параметры встречного предложения (контр-оффера)",
			nil,
		)
		return
	}

	if err := h.stateMachine.ValidateTransition(domain.StatusCounterOffered, domain.StatusApproved); err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	co := application.CounterOffer
	comment := "Клиент принял встречное предложение банка"
	extra := &domain.ApplicationUpdateFields{
		ApprovedAmount:     &co.Amount,
		ApprovedTermMonths: &co.TermMonths,
		InterestRate:       &co.Rate,
	}

	err = h.appRepo.UpdateStatus(
		r.Context(),
		nil,
		id,
		domain.StatusCounterOffered,
		domain.StatusApproved,
		"CLIENT",
		&comment,
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

// RejectCounterOffer обрабатывает POST /api/v1/applications/{id}/counter-offer/reject.
// Клиент отклоняет встречное предложение (COUNTER_OFFERED -> CLIENT_DECLINED).
func (h *ClientActionsHandler) RejectCounterOffer(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	application, err := h.appRepo.GetByID(r.Context(), id)
	if err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	if err := h.checkClientOwnership(r, application); err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	if application.Status != domain.StatusCounterOffered {
		domain.RespondWithError(
			w,
			http.StatusConflict,
			domain.ErrCodeConflictState,
			"Отклонение контр-оффера допустимо только для заявок в статусе COUNTER_OFFERED",
			nil,
		)
		return
	}

	if err := h.stateMachine.ValidateTransition(domain.StatusCounterOffered, domain.StatusClientDeclined); err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	comment := "Клиент отклонил встречное предложение банка"
	err = h.appRepo.UpdateStatus(
		r.Context(),
		nil,
		id,
		domain.StatusCounterOffered,
		domain.StatusClientDeclined,
		"CLIENT",
		&comment,
		nil,
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

// UploadDocument обрабатывает POST /api/v1/applications/{id}/documents (multipart/form-data).
// Загружает документ (до 10 МБ, PDF/PNG/JPEG) и возвращает заявку в MANUAL_REVIEW.
func (h *ClientActionsHandler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	application, err := h.appRepo.GetByID(r.Context(), id)
	if err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	if err := h.checkClientOwnership(r, application); err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	// Загрузка документов допустима только из статуса WAITING_DOCUMENTS
	if application.Status != domain.StatusWaitingDocuments {
		domain.RespondWithError(
			w,
			http.StatusConflict,
			domain.ErrCodeConflictState,
			"Загрузка документов допустима только для заявок в статусе WAITING_DOCUMENTS",
			nil,
		)
		return
	}

	// Ограничение размера тела запроса строго до 10 МБ
	const maxUploadSize = 10 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		domain.RespondWithError(
			w,
			http.StatusBadRequest,
			domain.ErrCodeBadRequest,
			"Размер файла превышает допустимый лимит 10 МБ или тело запроса некорректно",
			domain.StringPtr(err.Error()),
		)
		return
	}

	docTypeStr := strings.TrimSpace(r.FormValue("document_type"))
	docType := domain.DocumentType(docTypeStr)
	if !docType.IsValid() {
		domain.RespondWithError(
			w,
			http.StatusBadRequest,
			domain.ErrCodeBadRequest,
			"Некорректный тип документа document_type (ожидается FINANCIAL_STATEMENT, TAX_DECLARATION, BANK_STATEMENT, LEASE_CONTRACT, OTHER)",
			nil,
		)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		domain.RespondWithError(
			w,
			http.StatusBadRequest,
			domain.ErrCodeBadRequest,
			"Поле file обязательно в multipart/form-data",
			domain.StringPtr(err.Error()),
		)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".pdf" && ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		domain.RespondWithError(
			w,
			http.StatusBadRequest,
			domain.ErrCodeBadRequest,
			"Недопустимый формат файла. Разрешены только PDF, PNG и JPEG",
			nil,
		)
		return
	}

	if err := os.MkdirAll(h.uploadDir, 0755); err != nil {
		domain.RespondWithError(
			w,
			http.StatusInternalServerError,
			domain.ErrCodeInternalError,
			"Не удалось создать директорию для хранения файлов",
			domain.StringPtr(err.Error()),
		)
		return
	}

	docID := newUUID()
	savedFileName := fmt.Sprintf("%s%s", docID, ext)
	savedFilePath := filepath.Join(h.uploadDir, savedFileName)

	dst, err := os.Create(savedFilePath)
	if err != nil {
		domain.RespondWithError(
			w,
			http.StatusInternalServerError,
			domain.ErrCodeInternalError,
			"Ошибка сохранения файла на сервере",
			domain.StringPtr(err.Error()),
		)
		return
	}
	defer dst.Close()

	size, err := io.Copy(dst, file)
	if err != nil {
		domain.RespondWithError(
			w,
			http.StatusInternalServerError,
			domain.ErrCodeInternalError,
			"Ошибка записи содержимого файла",
			domain.StringPtr(err.Error()),
		)
		return
	}

	doc := domain.Document{
		ID:           docID,
		Name:         header.Filename,
		DocumentType: docType,
		FilePath:     savedFilePath,
		UploadedAt:   time.Now(),
		SizeBytes:    size,
	}

	if err := h.appRepo.AddDocument(r.Context(), nil, id, &doc); err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	// Автоматический перевод заявки WAITING_DOCUMENTS -> MANUAL_REVIEW
	auditComment := fmt.Sprintf("Клиент прикрепил документ: %s (%s)", header.Filename, docType)
	err = h.appRepo.UpdateStatus(
		r.Context(),
		nil,
		id,
		domain.StatusWaitingDocuments,
		domain.StatusManualReview,
		"CLIENT",
		&auditComment,
		nil,
	)
	if err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	domain.RespondWithJSON(w, http.StatusCreated, doc)
}

// Sign обрабатывает POST /api/v1/applications/{id}/sign.
// Подписание договора клиентом (AUTO_APPROVED | APPROVED -> SIGNED) с последующим фоновым зачислением (-> DISBURSED).
func (h *ClientActionsHandler) Sign(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	application, err := h.appRepo.GetByID(r.Context(), id)
	if err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	if err := h.checkClientOwnership(r, application); err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	// Подписание допустимо только из AUTO_APPROVED или APPROVED
	if application.Status != domain.StatusAutoApproved && application.Status != domain.StatusApproved {
		domain.RespondWithError(
			w,
			http.StatusConflict,
			domain.ErrCodeConflictState,
			"Подписание договора допустимо только для одобренных заявок (AUTO_APPROVED или APPROVED)",
			nil,
		)
		return
	}

	if err := h.stateMachine.ValidateTransition(application.Status, domain.StatusSigned); err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	signComment := "Кредитный договор успешно подписан клиентом электронной подписью"
	err = h.appRepo.UpdateStatus(
		r.Context(),
		nil,
		id,
		application.Status,
		domain.StatusSigned,
		"CLIENT",
		&signComment,
		nil,
	)
	if err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	// Фоновый процесс выдачи кредитных средств (~2 секунды: SIGNED -> DISBURSED)
	go func(appID string, delay time.Duration) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("КРИТИЧЕСКАЯ ПАНИКА в процессе выдачи средств",
					slog.String("application_id", appID),
					slog.Any("panic", r),
					slog.String("stack", string(debug.Stack())),
				)
			}
		}()

		if delay > 0 {
			time.Sleep(delay)
		}

		disburseComment := "Кредитные средства успешно зачислены на расчетный счет клиента"
		err := h.appRepo.UpdateStatus(
			context.Background(),
			nil,
			appID,
			domain.StatusSigned,
			domain.StatusDisbursed,
			"SYSTEM",
			&disburseComment,
			nil,
		)
		if err != nil {
			slog.Error("Ошибка перевода заявки в статус DISBURSED",
				slog.String("application_id", appID),
				slog.String("error", err.Error()),
			)
		}
	}(id, h.disburseDelay)

	updatedApp, err := h.appRepo.GetByID(r.Context(), id)
	if err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	domain.RespondWithJSON(w, http.StatusOK, updatedApp)
}
