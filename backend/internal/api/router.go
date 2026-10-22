package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"smart-credit-hub/internal/api/handlers"
	"smart-credit-hub/internal/api/middleware"
	"smart-credit-hub/internal/auth"
	"smart-credit-hub/internal/config"
	"smart-credit-hub/internal/mock"
)

// Dependencies объединяет все зависимости для сборки роутера приложения.
type Dependencies struct {
	Config      *config.Config
	AuthService auth.Service
	MockService mock.Service
	Pinger      handlers.Pinger
}

// NewRouter собирает и конфигурирует Chi-роутер приложения со всеми middleware и хэндлерами.
func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()

	// Базовые middleware
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.CORS(deps.Config.CORSAllowedOrigins))

	// Инициализация хэндлеров
	healthH := handlers.NewHealthHandler(deps.Pinger)
	authH := handlers.NewAuthHandler(deps.AuthService)
	mockH := handlers.NewMockHandler(deps.MockService)

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
	})

	return r
}
