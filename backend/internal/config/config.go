package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config хранит настройки конфигурации приложения.
type Config struct {
	// Port - порт для запуска HTTP-сервера (по умолчанию: "8080").
	Port string

	// DBDSN - строка подключения к PostgreSQL.
	DBDSN string

	// JWTSecret - секретный ключ для подписи и верификации JWT.
	JWTSecret string

	// JWTExpirationHours - время жизни токена в часах.
	JWTExpirationHours int

	// CasesFilePath - путь к файлу с тестовыми данными (cases.json).
	CasesFilePath string

	// CORSAllowedOrigins - список разрешенных origins для CORS.
	CORSAllowedOrigins []string

	// UploadDir - директория для сохранения загружаемых документов (по умолчанию: "uploads").
	UploadDir string

	// Environment - среда окружения (development, production, test).
	Environment string

	// ReadHeaderTimeout - таймаут на чтение заголовков запроса.
	ReadHeaderTimeout time.Duration

	// ReadTimeout - таймаут на чтение тела запроса.
	ReadTimeout time.Duration

	// WriteTimeout - таймаут на запись ответа.
	WriteTimeout time.Duration

	// ShutdownTimeout - время ожидания корректного завершения соединений (Graceful Shutdown).
	ShutdownTimeout time.Duration
}

// Load загружает конфигурацию из переменных окружения с безопасными дефолтными значениями.
func Load() *Config {
	port := getEnv("PORT", "8080")
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	dbDSN := getEnv("DB_DSN", "postgres://alfa_user:alfa_password@localhost:5432/alfa_credit_db?sslmode=disable")
	jwtSecret := getEnv("JWT_SECRET", "smart-credit-hub-super-secret-key-demo-2026")

	jwtExpHours, err := strconv.Atoi(getEnv("JWT_EXPIRATION_HOURS", "24"))
	if err != nil || jwtExpHours <= 0 {
		jwtExpHours = 24
	}

	casesPath := FindCasesFilePath(getEnv("CASES_FILE_PATH", ""))

	corsOriginsStr := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173")
	corsOrigins := strings.Split(corsOriginsStr, ",")
	for i := range corsOrigins {
		corsOrigins[i] = strings.TrimSpace(corsOrigins[i])
	}

	uploadDir := getEnv("UPLOAD_DIR", "uploads")
	env := getEnv("APP_ENV", "development")

	return &Config{
		Port:               port,
		DBDSN:              dbDSN,
		JWTSecret:          jwtSecret,
		JWTExpirationHours: jwtExpHours,
		CasesFilePath:      casesPath,
		CORSAllowedOrigins: corsOrigins,
		UploadDir:          uploadDir,
		Environment:        env,
		ReadHeaderTimeout:  5 * time.Second,
		ReadTimeout:        15 * time.Second,
		WriteTimeout:       15 * time.Second,
		ShutdownTimeout:    5 * time.Second,
	}
}

// getEnv возвращает значение переменной окружения или fallback, если переменная не задана.
func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return fallback
}

// FindCasesFilePath ищет файл cases.json в стандартных локациях проекта.
func FindCasesFilePath(override string) string {
	if override != "" {
		if _, err := os.Stat(override); err == nil {
			return override
		}
	}

	candidates := []string{
		"scoring/testdata/cases.json",
		"../scoring/testdata/cases.json",
		"../../scoring/testdata/cases.json",
		"../../../scoring/testdata/cases.json",
		"../../../../scoring/testdata/cases.json",
	}

	for _, cand := range candidates {
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}

	return "scoring/testdata/cases.json"
}
