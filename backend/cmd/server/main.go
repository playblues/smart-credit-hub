package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"smart-credit-hub/internal/api"
	"smart-credit-hub/internal/app"
	"smart-credit-hub/internal/auth"
	"smart-credit-hub/internal/config"
	"smart-credit-hub/internal/mock"
	"smart-credit-hub/internal/scoring"
	"smart-credit-hub/internal/storage"
	"smart-credit-hub/migrations"
)

func main() {
	// 1. Загрузка конфигурации приложения
	cfg := config.Load()

	// 2. Инициализация структурированного логгера log/slog
	initLogger(cfg.Environment)

	slog.Info("Запуск кредитного конвейера Alfa Credit Pipeline",
		slog.String("port", cfg.Port),
		slog.String("env", cfg.Environment),
		slog.String("cases_path", cfg.CasesFilePath),
	)

	// 3. Подключение к PostgreSQL с retry-логикой (до 5 попыток с интервалом 2 сек)
	var pool *pgxpool.Pool
	var dbErr error
	maxRetries := 5
	retryInterval := 2 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		slog.Info("Попытка подключения к базе данных PostgreSQL",
			slog.Int("attempt", attempt),
			slog.Int("max_attempts", maxRetries),
		)

		connCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		pool, dbErr = storage.NewPostgresPool(connCtx, cfg.DBDSN)
		cancel()

		if dbErr == nil {
			slog.Info("Соединение с PostgreSQL успешно установлено и проверено")
			break
		}

		slog.Warn("Не удалось подключиться к PostgreSQL",
			slog.Int("attempt", attempt),
			slog.String("error", dbErr.Error()),
		)

		if attempt < maxRetries {
			time.Sleep(retryInterval)
		}
	}

	if dbErr != nil {
		slog.Error("Критическая ошибка: не удалось подключиться к PostgreSQL после всех попыток", "error", dbErr)
		os.Exit(1)
	}
	defer pool.Close()

	// 4. Применение встроенных миграций схемы БД через embed.FS
	migrationCtx, cancelMigration := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelMigration()

	if err := storage.RunMigrations(migrationCtx, pool, migrations.FS); err != nil {
		slog.Error("Критическая ошибка выполнения миграций базы данных", "error", err)
		os.Exit(1)
	}

	// 5. Инициализация репозиториев, скорингового ядра и мок-сервиса
	userRepo := storage.NewPostgresUserRepository(pool)
	appRepo := storage.NewPostgresApplicationRepository(pool)

	mockService, err := mock.NewService(cfg.CasesFilePath)
	if err != nil {
		slog.Error("Критическая ошибка инициализации мок-сервиса", "error", err)
		os.Exit(1)
	}

	scoringModel, err := scoring.LoadModel("")
	if err != nil {
		slog.Error("Критическая ошибка загрузки скоринговой модели", "error", err)
		os.Exit(1)
	}
	scoringEngine := scoring.NewEngine(scoringModel)

	// Фоновый кредитный конвейер
	pipeline := app.NewPipelineService(appRepo, mockService, scoringEngine, pool)

	// 6. Инициализация сервиса аутентификации и токенов JWT
	tokenManager := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTExpirationHours)
	authService := auth.NewService(tokenManager, userRepo)

	// 7. Построение HTTP-роутера со всеми слоями, middleware и проверкой БД в healthcheck
	router := api.NewRouter(api.Dependencies{
		Config:      cfg,
		AuthService: authService,
		MockService: mockService,
		AppRepo:     appRepo,
		Pipeline:    pipeline,
		Pinger:      pool,
	})

	// 8. Конфигурация HTTP-сервера с защитой от медленных соединений
	server := &http.Server{
		Addr:              cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)

	// 9. Запуск сервера в фоновой горутине
	go func() {
		slog.Info("HTTP сервер слушает входящие соединения", slog.String("addr", cfg.Port))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// 10. Ожидание сигнала на завершение (Graceful Shutdown)
	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		slog.Error("Непредвиденная ошибка HTTP-сервера", "error", err)
		os.Exit(1)

	case sig := <-shutdownSignals:
		slog.Info("Получен сигнал завершения работы, начало Graceful Shutdown", "signal", sig.String())

		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			slog.Error("Ошибка при принудительной остановке HTTP сервера", "error", err)
			_ = server.Close()
			os.Exit(1)
		}

		slog.Info("Сервер успешно остановлен, соединения закрыты")
	}
}

// initLogger настраивает log/slog в зависимости от окружения.
func initLogger(env string) {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	if env == "development" || env == "local" {
		opts.Level = slog.LevelDebug
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}
