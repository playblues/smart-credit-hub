package storage

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Transactor интерфейс для выполнения бизнес-логики в рамках транзакции.
type Transactor interface {
	WithinTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// PostgresStorage объединяет пул соединений и методы управления БД.
type PostgresStorage struct {
	Pool *pgxpool.Pool
}

// NewPostgresPool создает и конфигурирует пул соединений к PostgreSQL в соответствии с ТЗ.
func NewPostgresPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга DSN: %w", err)
	}

	// Настройки пула соединений из ТЗ
	poolConfig.MaxConns = 25
	poolConfig.MinConns = 5
	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.MaxConnLifetime = 1 * time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать пул соединений: %w", err)
	}

	// Проверка доступности базы данных
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ошибка пинга PostgreSQL при инициализации: %w", err)
	}

	slog.Info("Пул соединений с PostgreSQL успешно инициализирован",
		slog.Int("max_conns", int(poolConfig.MaxConns)),
		slog.Int("min_conns", int(poolConfig.MinConns)),
		slog.Duration("max_conn_idle_time", poolConfig.MaxConnIdleTime),
		slog.Duration("max_conn_lifetime", poolConfig.MaxConnLifetime),
	)

	return pool, nil
}

// RunMigrations находит все SQL-файлы в embed.FS, сортирует по имени и последовательно применяет их.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationFS embed.FS) error {
	// Создание служебной таблицы версий миграций
	createTableQuery := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`
	if _, err := pool.Exec(ctx, createTableQuery); err != nil {
		return fmt.Errorf("ошибка создания таблицы schema_migrations: %w", err)
	}

	// Чтение списка файлов миграций
	entries, err := fs.ReadDir(migrationFS, ".")
	if err != nil {
		return fmt.Errorf("ошибка чтения каталога миграций: %w", err)
	}

	var migrationFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			migrationFiles = append(migrationFiles, entry.Name())
		}
	}

	sort.Strings(migrationFiles)

	for _, fileName := range migrationFiles {
		// Проверяем, была ли уже применена миграция
		var exists bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1);`
		err := pool.QueryRow(ctx, checkQuery, fileName).Scan(&exists)
		if err != nil {
			return fmt.Errorf("ошибка проверки статуса миграции %s: %w", fileName, err)
		}

		if exists {
			continue
		}

		// Читаем содержимое SQL файла
		content, err := migrationFS.ReadFile(fileName)
		if err != nil {
			return fmt.Errorf("ошибка чтения файла миграции %s: %w", fileName, err)
		}

		// Выполняем миграцию в транзакции
		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("ошибка открытия транзакции для миграции %s: %w", fileName, err)
		}

		if _, err := tx.Exec(ctx, string(content)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("ошибка выполнения SQL миграции %s: %w", fileName, err)
		}

		// Фиксируем версию в таблице schema_migrations
		insertVersionQuery := `INSERT INTO schema_migrations (version) VALUES ($1);`
		if _, err := tx.Exec(ctx, insertVersionQuery, fileName); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("ошибка записи версии миграции %s: %w", fileName, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("ошибка коммита транзакции миграции %s: %w", fileName, err)
		}

		slog.Info("Успешно применена миграция базы данных", slog.String("version", fileName))
	}

	return nil
}

// WithinTransaction выполняет переданную функцию внутри транзакции PostgreSQL.
func WithinTransaction(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ошибка запуска транзакции: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("ошибка коммита транзакции: %w", err)
	}

	return nil
}
