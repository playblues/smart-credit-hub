package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"smart-credit-hub/internal/api/handlers"
	"smart-credit-hub/internal/api/middleware"
	"smart-credit-hub/internal/app"
	"smart-credit-hub/internal/auth"
	"smart-credit-hub/internal/config"
	"smart-credit-hub/internal/domain"
	"smart-credit-hub/internal/mock"
	"smart-credit-hub/internal/scoring"
	"smart-credit-hub/internal/storage"
)

// Dependencies объединяет все зависимости для сборки роутера приложения.
type Dependencies struct {
	Config               *config.Config
	AuthService          auth.Service
	MockService          mock.Service
	AppRepo              storage.ApplicationRepository
	Pipeline             *app.PipelineService
	ScoringModel         *scoring.ModelConfig
	ClientActionsHandler *handlers.ClientActionsHandler
	BankHandler          *handlers.BankHandler
	Pinger               handlers.Pinger
}

// NewRouter собирает и конфигурирует Chi-роутер приложения со всеми middleware и хэндлерами.
func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()

	// Базовые middleware
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.CORS(deps.Config.CORSAllowedOrigins))

	// Инициализация репозитория по умолчанию для тестов
	appRepo := deps.AppRepo
	if appRepo == nil {
		appRepo = storage.NewInMemoryApplicationRepository()
	}

	uploadDir := "uploads"
	if deps.Config != nil && deps.Config.UploadDir != "" {
		uploadDir = deps.Config.UploadDir
	}

	// Инициализация хэндлеров
	healthH := handlers.NewHealthHandler(deps.Pinger)
	authH := handlers.NewAuthHandler(deps.AuthService)
	mockH := handlers.NewMockHandler(deps.MockService)
	appsH := handlers.NewApplicationsHandler(appRepo, deps.Pipeline)

	clientActionsH := deps.ClientActionsHandler
	if clientActionsH == nil {
		clientActionsH = handlers.NewClientActionsHandler(appRepo, uploadDir)
	}

	bankH := deps.BankHandler
	if bankH == nil {
		bankH = handlers.NewBankHandler(appRepo, deps.ScoringModel)
	}

	// Liveness проверка (доступна на корневом уровне)
	r.Get("/healthz", healthH.Check)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Публичные эндпоинты аутентификации
		r.Route("/auth", func(r chi.Router) {
			r.Post("/login", authH.Login)
		})

		// Мок-эндпоинты внешних систем (ФНС и БКИ)
		r.Route("/mock", func(r chi.Router) {
			r.Get("/fns/{inn}", mockH.GetFNS)
			r.Get("/bki/{inn}", mockH.GetBKI)
		})

		// Защищенные эндпоинты заявок (требуют JWT авторизации)
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(deps.AuthService))

			// 1. Клиентские заявки
			r.Route("/applications", func(r chi.Router) {
				// Создание и просмотр списка - только клиенты
				r.With(middleware.RequireRole(domain.RoleClient)).Post("/", appsH.CreateApplication)
				r.With(middleware.RequireRole(domain.RoleClient)).Get("/", appsH.ListApplications)

				// Просмотр заявки по ID: клиент видит только свою, андеррайтер - любую
				r.Get("/{id}", appsH.GetApplicationByID)

				// Действия клиента по заявке (принятие/отклонение контр-оффера, документы, подписание)
				r.With(middleware.RequireRole(domain.RoleClient)).Group(func(r chi.Router) {
					r.Post("/{id}/counter-offer/accept", clientActionsH.AcceptCounterOffer)
					r.Post("/{id}/counter-offer/reject", clientActionsH.RejectCounterOffer)
					r.Post("/{id}/documents", clientActionsH.UploadDocument)
					r.Post("/{id}/sign", clientActionsH.Sign)
				})
			})

			// 2. АРМ Андеррайтера (доступно только роли UNDERWRITER)
			r.Route("/bank", func(r chi.Router) {
				r.Use(middleware.RequireRole(domain.RoleUnderwriter))

				r.Get("/applications", bankH.ListApplications)
				r.Get("/applications/{id}", bankH.GetApplicationByID)
				r.Post("/applications/{id}/decision", bankH.Decision)
			})
		})
	})

	return r
}
