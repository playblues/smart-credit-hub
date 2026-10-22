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

	"smart-credit-hub/internal/api"
	"smart-credit-hub/internal/auth"
	"smart-credit-hub/internal/config"
	"smart-credit-hub/internal/mock"
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

	// 3. Инициализация мок-сервиса внешних данных (ФНС и БКИ)
	mockService, err := mock.NewService(cfg.CasesFilePath)
	if err != nil {
		slog.Error("Критическая ошибка инициализации мок-сервиса", "error", err)
		os.Exit(1)
	}

	// 4. Инициализация сервиса аутентификации и токенов JWT
	tokenManager := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTExpirationHours)
	authService := auth.NewService(tokenManager)

	// 5. Построение HTTP-роутера со всеми слоями и middleware
	router := api.NewRouter(api.Dependencies{
		Config:      cfg,
		AuthService: authService,
		MockService: mockService,
	})

	// 6. Конфигурация HTTP-сервера с защитой от медленных соединений
	server := &http.Server{
		Addr:              cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       60 * time.Second,
	}

	// Канал для перехвата ошибок сервера при запуске
	serverErrors := make(chan error, 1)

	// 7. Запуск сервера в фоновой горутине
	go func() {
		slog.Info("HTTP сервер слушает входящие соединения", slog.String("addr", cfg.Port))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// 8. Ожидание сигнала на завершение (Graceful Shutdown)
	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		slog.Error("Непредвиденная ошибка HTTP-сервера", "error", err)
		os.Exit(1)

	case sig := <-shutdownSignals:
		slog.Info("Получен сигнал завершения работы, начало Graceful Shutdown", "signal", sig.String())

		// Контекст ожидания завершения активных соединений с таймаутом
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			slog.Error("Ошибка при принудительной остановке сервера", "error", err)
			_ = server.Close()
			os.Exit(1)
		}

		slog.Info("Сервер успешно остановлен, все ресурсы освобождены")
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
