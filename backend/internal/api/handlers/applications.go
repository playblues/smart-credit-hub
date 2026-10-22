package handlers

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"smart-credit-hub/internal/api/middleware"
	"smart-credit-hub/internal/app"
	"smart-credit-hub/internal/domain"
	"smart-credit-hub/internal/storage"
)

// ApplicationsHandler инкапсулирует HTTP-обработчики работы с кредитными заявками.
type ApplicationsHandler struct {
	appRepo  storage.ApplicationRepository
	pipeline *app.PipelineService
}

// NewApplicationsHandler создает новый экземпляр ApplicationsHandler.
func NewApplicationsHandler(appRepo storage.ApplicationRepository, pipeline *app.PipelineService) *ApplicationsHandler {
	return &ApplicationsHandler{
		appRepo:  appRepo,
		pipeline: pipeline,
	}
}

// CreateApplication обрабатывает POST /api/v1/applications.
// Создает кредитную заявку в статусе SUBMITTED и запускает конвейер в фоне.
func (h *ApplicationsHandler) CreateApplication(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok || claims == nil {
		domain.RespondWithError(
			w,
			http.StatusUnauthorized,
			domain.ErrCodeUnauthorized,
			"Пользователь не аутентифицирован",
			nil,
		)
		return
	}

	var req domain.CreateApplicationRequest
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

	req.INN = strings.TrimSpace(req.INN)
	if req.INN == "" || len(req.INN) < 10 || len(req.INN) > 12 {
		domain.RespondWithError(
			w,
			http.StatusBadRequest,
			domain.ErrCodeBadRequest,
			"ИНН организации обязателен и должен содержать от 10 до 12 цифр",
			nil,
		)
		return
	}

	// Клиент может подавать заявку только от лица своей организации
	if claims.Role == domain.RoleClient && claims.INN != nil && *claims.INN != "" && *claims.INN != req.INN {
		domain.RespondWithError(
			w,
			http.StatusForbidden,
			domain.ErrCodeForbidden,
			"Подача заявки возможна только по ИНН организации текущего пользователя",
			nil,
		)
		return
	}

	if req.ProductType != domain.ProductWorkingCapital && req.ProductType != domain.ProductOverdraft {
		domain.RespondWithError(
			w,
			http.StatusBadRequest,
			domain.ErrCodeBadRequest,
			"Недопустимый тип кредитного продукта (ожидается WORKING_CAPITAL или OVERDRAFT)",
			nil,
		)
		return
	}

	if req.RequestedAmount <= 0 {
		domain.RespondWithError(
			w,
			http.StatusBadRequest,
			domain.ErrCodeBadRequest,
			"Запрашиваемая сумма кредита должна быть строго больше нуля",
			nil,
		)
		return
	}

	if req.RequestedTermMonths <= 0 {
		domain.RespondWithError(
			w,
			http.StatusBadRequest,
			domain.ErrCodeBadRequest,
			"Запрашиваемый срок кредита должен быть строго больше нуля",
			nil,
		)
		return
	}

	now := time.Now()
	newApp := domain.Application{
		ID:                  newUUID(),
		UserID:              &claims.UserID,
		INN:                 req.INN,
		Status:              domain.StatusSubmitted,
		ProductType:         req.ProductType,
		Purpose:             req.Purpose,
		RequestedAmount:     req.RequestedAmount,
		RequestedTermMonths: req.RequestedTermMonths,
		Documents:           make([]domain.Document, 0),
		StatusHistory: []domain.AuditLogEntry{
			{
				Timestamp:  now,
				FromStatus: "",
				ToStatus:   domain.StatusSubmitted,
				ActorRole:  "CLIENT",
				Comment:    domain.StringPtr("Подача кредитной заявки"),
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.appRepo.Create(r.Context(), nil, &newApp); err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	// Запуск асинхронной фоновой обработки конвейера
	if h.pipeline != nil {
		h.pipeline.StartProcessing(newApp.ID)
	}

	domain.RespondWithJSON(w, http.StatusCreated, &newApp)
}

// ListApplications обрабатывает GET /api/v1/applications.
// Возвращает список всех заявок текущего клиента.
func (h *ApplicationsHandler) ListApplications(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok || claims == nil {
		domain.RespondWithError(
			w,
			http.StatusUnauthorized,
			domain.ErrCodeUnauthorized,
			"Пользователь не аутентифицирован",
			nil,
		)
		return
	}

	apps, err := h.appRepo.ListByUserID(r.Context(), claims.UserID)
	if err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	if apps == nil {
		apps = make([]*domain.Application, 0)
	}

	domain.RespondWithJSON(w, http.StatusOK, apps)
}

// GetApplicationByID обрабатывает GET /api/v1/applications/{id}.
// Проверяет разграничение доступа: клиент может открыть только свою заявку, андеррайтер - любую.
func (h *ApplicationsHandler) GetApplicationByID(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.GetClaims(r.Context())
	if !ok || claims == nil {
		domain.RespondWithError(
			w,
			http.StatusUnauthorized,
			domain.ErrCodeUnauthorized,
			"Пользователь не аутентифицирован",
			nil,
		)
		return
	}

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

	app, err := h.appRepo.GetByID(r.Context(), id)
	if err != nil {
		domain.RespondWithAppError(w, err)
		return
	}

	// Разграничение доступа: клиент может видеть только свою заявку
	if claims.Role == domain.RoleClient {
		if app.UserID == nil || *app.UserID != claims.UserID {
			domain.RespondWithError(
				w,
				http.StatusForbidden,
				domain.ErrCodeForbidden,
				"Доступ к чужой кредитной заявке запрещен",
				nil,
			)
			return
		}
	}

	domain.RespondWithJSON(w, http.StatusOK, app)
}

func newUUID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}
