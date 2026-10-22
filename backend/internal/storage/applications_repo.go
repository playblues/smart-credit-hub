package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"smart-credit-hub/internal/domain"
)

// ApplicationRepository интерфейс хранилища кредитных заявок.
type ApplicationRepository interface {
	Create(ctx context.Context, tx pgx.Tx, app *domain.Application) error
	GetByID(ctx context.Context, id string) (*domain.Application, error)
	ListByUserID(ctx context.Context, userID string) ([]*domain.Application, error)
	ListAll(ctx context.Context, status *domain.ApplicationStatus, riskZone *domain.RiskZone) ([]*domain.Application, error)
	AddDocument(ctx context.Context, tx pgx.Tx, appID string, doc *domain.Document) error
	UpdateStatus(
		ctx context.Context,
		tx pgx.Tx,
		id string,
		oldStatus, newStatus domain.ApplicationStatus,
		actorRole string,
		comment *string,
		extraFields *domain.ApplicationUpdateFields,
	) error
}

type postgresApplicationRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresApplicationRepository создает экземпляр репозитория заявок для PostgreSQL.
func NewPostgresApplicationRepository(pool *pgxpool.Pool) ApplicationRepository {
	return &postgresApplicationRepository{pool: pool}
}

// DBExecutor интерфейс, позволяющий выполнять запросы как на пуле соединений, так и в транзакции.
type DBExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *postgresApplicationRepository) getExecutor(tx pgx.Tx) DBExecutor {
	if tx != nil {
		return tx
	}
	return r.pool
}

// Create сохраняет новую заявку и делает первую запись в аудит лог.
func (r *postgresApplicationRepository) Create(ctx context.Context, tx pgx.Tx, app *domain.Application) error {
	exec := r.getExecutor(tx)

	var counterOfferJSON, scoringResultJSON, companySnapshotJSON []byte
	var err error

	if app.CounterOffer != nil {
		counterOfferJSON, err = json.Marshal(app.CounterOffer)
		if err != nil {
			return fmt.Errorf("ошибка сериализации counter_offer: %w", err)
		}
	}
	if app.ScoringResult != nil {
		scoringResultJSON, err = json.Marshal(app.ScoringResult)
		if err != nil {
			return fmt.Errorf("ошибка сериализации scoring_result: %w", err)
		}
	}
	if app.CompanySnapshot != nil {
		companySnapshotJSON, err = json.Marshal(app.CompanySnapshot)
		if err != nil {
			return fmt.Errorf("ошибка сериализации company_snapshot: %w", err)
		}
	}

	query := `
		INSERT INTO applications (
			id, user_id, inn, status, product_type, purpose,
			requested_amount, requested_term_months, approved_amount, approved_term_months,
			interest_rate, counter_offer, scoring_result, company_snapshot,
			underwriter_comment, reject_reason_client, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14,
			$15, $16, $17, $18
		);
	`

	if app.CreatedAt.IsZero() {
		app.CreatedAt = time.Now()
	}
	if app.UpdatedAt.IsZero() {
		app.UpdatedAt = time.Now()
	}

	_, err = exec.Exec(ctx, query,
		app.ID, app.UserID, app.INN, string(app.Status), string(app.ProductType), app.Purpose,
		app.RequestedAmount, app.RequestedTermMonths, app.ApprovedAmount, app.ApprovedTermMonths,
		app.InterestRate, counterOfferJSON, scoringResultJSON, companySnapshotJSON,
		app.UnderwriterComment, app.RejectReasonClient, app.CreatedAt, app.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("ошибка вставки заявки: %w", err)
	}

	// Запись первого события в audit_logs
	auditQuery := `
		INSERT INTO audit_logs (application_id, from_status, to_status, actor_role, comment, created_at)
		VALUES ($1, $2, $3, $4, $5, $6);
	`
	initialComment := "Подача кредитной заявки"
	_, err = exec.Exec(ctx, auditQuery,
		app.ID, "", string(app.Status), "CLIENT", &initialComment, app.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("ошибка записи начального аудита: %w", err)
	}

	return nil
}

// GetByID возвращает полную карточку заявки с историей аудита и списком документов.
func (r *postgresApplicationRepository) GetByID(ctx context.Context, id string) (*domain.Application, error) {
	query := `
		SELECT
			id, user_id, inn, status, product_type, purpose,
			requested_amount, requested_term_months, approved_amount, approved_term_months,
			interest_rate, counter_offer, scoring_result, company_snapshot,
			underwriter_comment, reject_reason_client, created_at, updated_at
		FROM applications
		WHERE id = $1;
	`

	var app domain.Application
	var statusStr, productTypeStr string
	var counterOfferBytes, scoringResultBytes, companySnapshotBytes []byte

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&app.ID, &app.UserID, &app.INN, &statusStr, &productTypeStr, &app.Purpose,
		&app.RequestedAmount, &app.RequestedTermMonths, &app.ApprovedAmount, &app.ApprovedTermMonths,
		&app.InterestRate, &counterOfferBytes, &scoringResultBytes, &companySnapshotBytes,
		&app.UnderwriterComment, &app.RejectReasonClient, &app.CreatedAt, &app.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound(fmt.Sprintf("Заявка с ID '%s' не найдена", id), nil)
		}
		return nil, fmt.Errorf("ошибка запроса заявки: %w", err)
	}

	app.Status = domain.ApplicationStatus(statusStr)
	app.ProductType = domain.ProductType(productTypeStr)

	if len(counterOfferBytes) > 0 {
		var co domain.CounterOffer
		if err := json.Unmarshal(counterOfferBytes, &co); err == nil {
			app.CounterOffer = &co
		}
	}
	if len(scoringResultBytes) > 0 {
		var sr domain.ScoringResult
		if err := json.Unmarshal(scoringResultBytes, &sr); err == nil {
			app.ScoringResult = &sr
		}
	}
	if len(companySnapshotBytes) > 0 {
		var cs domain.CompanySnapshot
		if err := json.Unmarshal(companySnapshotBytes, &cs); err == nil {
			app.CompanySnapshot = &cs
		}
	}

	// Загрузка документов
	app.Documents = make([]domain.Document, 0)
	docsQuery := `
		SELECT id, name, document_type, file_path, size_bytes, uploaded_at
		FROM documents
		WHERE application_id = $1
		ORDER BY uploaded_at ASC;
	`
	rows, err := r.pool.Query(ctx, docsQuery, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var doc domain.Document
			var dtStr string
			if err := rows.Scan(&doc.ID, &doc.Name, &dtStr, &doc.FilePath, &doc.SizeBytes, &doc.UploadedAt); err == nil {
				doc.DocumentType = domain.DocumentType(dtStr)
				app.Documents = append(app.Documents, doc)
			}
		}
	}

	// Загрузка истории статусов из audit_logs
	app.StatusHistory = make([]domain.AuditLogEntry, 0)
	auditQuery := `
		SELECT from_status, to_status, actor_role, comment, created_at
		FROM audit_logs
		WHERE application_id = $1
		ORDER BY created_at ASC;
	`
	auditRows, err := r.pool.Query(ctx, auditQuery, id)
	if err == nil {
		defer auditRows.Close()
		for auditRows.Next() {
			var entry domain.AuditLogEntry
			var fromStr, toStr string
			if err := auditRows.Scan(&fromStr, &toStr, &entry.ActorRole, &entry.Comment, &entry.Timestamp); err == nil {
				entry.FromStatus = domain.ApplicationStatus(fromStr)
				entry.ToStatus = domain.ApplicationStatus(toStr)
				app.StatusHistory = append(app.StatusHistory, entry)
			}
		}
	}

	return &app, nil
}

// ListByUserID возвращает список заявок конкретного пользователя (клиента).
func (r *postgresApplicationRepository) ListByUserID(ctx context.Context, userID string) ([]*domain.Application, error) {
	query := `
		SELECT
			id, user_id, inn, status, product_type, purpose,
			requested_amount, requested_term_months, approved_amount, approved_term_months,
			interest_rate, counter_offer, scoring_result, company_snapshot,
			underwriter_comment, reject_reason_client, created_at, updated_at
		FROM applications
		WHERE user_id = $1
		ORDER BY created_at DESC;
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения списка заявок пользователя: %w", err)
	}
	defer rows.Close()

	applications := make([]*domain.Application, 0)
	for rows.Next() {
		var app domain.Application
		var statusStr, productTypeStr string
		var counterOfferBytes, scoringResultBytes, companySnapshotBytes []byte

		if err := rows.Scan(
			&app.ID, &app.UserID, &app.INN, &statusStr, &productTypeStr, &app.Purpose,
			&app.RequestedAmount, &app.RequestedTermMonths, &app.ApprovedAmount, &app.ApprovedTermMonths,
			&app.InterestRate, &counterOfferBytes, &scoringResultBytes, &companySnapshotBytes,
			&app.UnderwriterComment, &app.RejectReasonClient, &app.CreatedAt, &app.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("ошибка сканирования строки заявки: %w", err)
		}

		app.Status = domain.ApplicationStatus(statusStr)
		app.ProductType = domain.ProductType(productTypeStr)
		app.Documents = make([]domain.Document, 0)
		app.StatusHistory = make([]domain.AuditLogEntry, 0)

		if len(counterOfferBytes) > 0 {
			var co domain.CounterOffer
			if err := json.Unmarshal(counterOfferBytes, &co); err == nil {
				app.CounterOffer = &co
			}
		}
		if len(scoringResultBytes) > 0 {
			var sr domain.ScoringResult
			if err := json.Unmarshal(scoringResultBytes, &sr); err == nil {
				app.ScoringResult = &sr
			}
		}
		if len(companySnapshotBytes) > 0 {
			var cs domain.CompanySnapshot
			if err := json.Unmarshal(companySnapshotBytes, &cs); err == nil {
				app.CompanySnapshot = &cs
			}
		}

		applications = append(applications, &app)
	}

	return applications, nil
}

// ListAll возвращает список всех заявок с опциональной фильтрацией по статусу и риск-зоне (для андеррайтера).
func (r *postgresApplicationRepository) ListAll(ctx context.Context, status *domain.ApplicationStatus, riskZone *domain.RiskZone) ([]*domain.Application, error) {
	query := `
		SELECT
			id, user_id, inn, status, product_type, purpose,
			requested_amount, requested_term_months, approved_amount, approved_term_months,
			interest_rate, counter_offer, scoring_result, company_snapshot,
			underwriter_comment, reject_reason_client, created_at, updated_at
		FROM applications
		WHERE 1=1
	`
	var args []any
	argIndex := 1

	if status != nil && *status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, string(*status))
		argIndex++
	}

	if riskZone != nil && *riskZone != "" {
		query += fmt.Sprintf(" AND scoring_result->>'risk_zone' = $%d", argIndex)
		args = append(args, string(*riskZone))
		argIndex++
	}

	query += " ORDER BY created_at DESC;"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения списка всех заявок: %w", err)
	}
	defer rows.Close()

	applications := make([]*domain.Application, 0)
	for rows.Next() {
		var app domain.Application
		var statusStr, productTypeStr string
		var counterOfferBytes, scoringResultBytes, companySnapshotBytes []byte

		if err := rows.Scan(
			&app.ID, &app.UserID, &app.INN, &statusStr, &productTypeStr, &app.Purpose,
			&app.RequestedAmount, &app.RequestedTermMonths, &app.ApprovedAmount, &app.ApprovedTermMonths,
			&app.InterestRate, &counterOfferBytes, &scoringResultBytes, &companySnapshotBytes,
			&app.UnderwriterComment, &app.RejectReasonClient, &app.CreatedAt, &app.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("ошибка сканирования строки заявки: %w", err)
		}

		app.Status = domain.ApplicationStatus(statusStr)
		app.ProductType = domain.ProductType(productTypeStr)
		app.Documents = make([]domain.Document, 0)
		app.StatusHistory = make([]domain.AuditLogEntry, 0)

		if len(counterOfferBytes) > 0 {
			var co domain.CounterOffer
			if err := json.Unmarshal(counterOfferBytes, &co); err == nil {
				app.CounterOffer = &co
			}
		}
		if len(scoringResultBytes) > 0 {
			var sr domain.ScoringResult
			if err := json.Unmarshal(scoringResultBytes, &sr); err == nil {
				app.ScoringResult = &sr
			}
		}
		if len(companySnapshotBytes) > 0 {
			var cs domain.CompanySnapshot
			if err := json.Unmarshal(companySnapshotBytes, &cs); err == nil {
				app.CompanySnapshot = &cs
			}
		}

		applications = append(applications, &app)
	}

	return applications, nil
}

// AddDocument сохраняет запись о загруженном документе в БД.
func (r *postgresApplicationRepository) AddDocument(ctx context.Context, tx pgx.Tx, appID string, doc *domain.Document) error {
	exec := r.getExecutor(tx)

	query := `
		INSERT INTO documents (id, application_id, name, document_type, file_path, size_bytes, uploaded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7);
	`
	if doc.UploadedAt.IsZero() {
		doc.UploadedAt = time.Now()
	}

	_, err := exec.Exec(ctx, query, doc.ID, appID, doc.Name, string(doc.DocumentType), doc.FilePath, doc.SizeBytes, doc.UploadedAt)
	if err != nil {
		return fmt.Errorf("ошибка сохранения документа: %w", err)
	}

	return nil
}

// UpdateStatus атомарно переводит статус заявки с проверкой текущего статуса и записывает аудит.
func (r *postgresApplicationRepository) UpdateStatus(
	ctx context.Context,
	tx pgx.Tx,
	id string,
	oldStatus, newStatus domain.ApplicationStatus,
	actorRole string,
	comment *string,
	extra *domain.ApplicationUpdateFields,
) error {
	exec := r.getExecutor(tx)

	var counterOfferJSON, scoringResultJSON, companySnapshotJSON []byte
	var approvedAmount *float64
	var approvedTermMonths *int
	var interestRate *float64
	var underwriterComment *string
	var rejectReasonClient *string

	if extra != nil {
		approvedAmount = extra.ApprovedAmount
		approvedTermMonths = extra.ApprovedTermMonths
		interestRate = extra.InterestRate
		underwriterComment = extra.UnderwriterComment
		rejectReasonClient = extra.RejectReasonClient

		if extra.CounterOffer != nil {
			counterOfferJSON, _ = json.Marshal(extra.CounterOffer)
		}
		if extra.ScoringResult != nil {
			scoringResultJSON, _ = json.Marshal(extra.ScoringResult)
		}
		if extra.CompanySnapshot != nil {
			companySnapshotJSON, _ = json.Marshal(extra.CompanySnapshot)
		}
	}

	updateQuery := `
		UPDATE applications
		SET
			status = $1,
			approved_amount = COALESCE($2, approved_amount),
			approved_term_months = COALESCE($3, approved_term_months),
			interest_rate = COALESCE($4, interest_rate),
			counter_offer = CASE WHEN $5::jsonb IS NOT NULL THEN $5::jsonb ELSE counter_offer END,
			scoring_result = CASE WHEN $6::jsonb IS NOT NULL THEN $6::jsonb ELSE scoring_result END,
			company_snapshot = CASE WHEN $7::jsonb IS NOT NULL THEN $7::jsonb ELSE company_snapshot END,
			underwriter_comment = COALESCE($8, underwriter_comment),
			reject_reason_client = COALESCE($9, reject_reason_client),
			updated_at = NOW()
		WHERE id = $10 AND status = $11;
	`

	tag, err := exec.Exec(ctx, updateQuery,
		string(newStatus),
		approvedAmount, approvedTermMonths, interestRate,
		counterOfferJSON, scoringResultJSON, companySnapshotJSON,
		underwriterComment, rejectReasonClient,
		id, string(oldStatus),
	)
	if err != nil {
		return fmt.Errorf("ошибка обновления статуса заявки: %w", err)
	}

	if tag.RowsAffected() == 0 {
		var currentStatus string
		checkErr := exec.QueryRow(ctx, "SELECT status FROM applications WHERE id = $1", id).Scan(&currentStatus)
		if checkErr != nil {
			if errors.Is(checkErr, pgx.ErrNoRows) {
				return domain.ErrNotFound(fmt.Sprintf("Заявка '%s' не найдена", id), nil)
			}
			return fmt.Errorf("ошибка проверки статуса заявки: %w", checkErr)
		}
		return domain.ErrInvalidStateTransition(domain.ApplicationStatus(currentStatus), newStatus)
	}

	// Запись в аудит лог
	auditQuery := `
		INSERT INTO audit_logs (application_id, from_status, to_status, actor_role, comment, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW());
	`
	_, err = exec.Exec(ctx, auditQuery, id, string(oldStatus), string(newStatus), actorRole, comment)
	if err != nil {
		return fmt.Errorf("ошибка записи события в audit_logs: %w", err)
	}

	return nil
}

// InMemoryApplicationRepository реализация хранилища заявок в памяти для автономных тестов.
type InMemoryApplicationRepository struct {
	mu           sync.RWMutex
	applications map[string]*domain.Application
}

// NewInMemoryApplicationRepository создает in-memory репозиторий заявок.
func NewInMemoryApplicationRepository() *InMemoryApplicationRepository {
	return &InMemoryApplicationRepository{
		applications: make(map[string]*domain.Application),
	}
}

func (r *InMemoryApplicationRepository) Create(ctx context.Context, tx pgx.Tx, app *domain.Application) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cp := *app
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now()
	}
	if cp.UpdatedAt.IsZero() {
		cp.UpdatedAt = time.Now()
	}
	cp.Documents = make([]domain.Document, 0)

	comment := "Подача кредитной заявки"
	cp.StatusHistory = []domain.AuditLogEntry{
		{
			Timestamp:  cp.CreatedAt,
			FromStatus: "",
			ToStatus:   cp.Status,
			ActorRole:  "CLIENT",
			Comment:    &comment,
		},
	}

	r.applications[cp.ID] = &cp
	return nil
}

func (r *InMemoryApplicationRepository) GetByID(ctx context.Context, id string) (*domain.Application, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	app, exists := r.applications[id]
	if !exists {
		return nil, domain.ErrNotFound(fmt.Sprintf("Заявка с ID '%s' не найдена", id), nil)
	}

	cp := *app
	return &cp, nil
}

func (r *InMemoryApplicationRepository) ListByUserID(ctx context.Context, userID string) ([]*domain.Application, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*domain.Application
	for _, app := range r.applications {
		if app.UserID != nil && *app.UserID == userID {
			cp := *app
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (r *InMemoryApplicationRepository) ListAll(ctx context.Context, status *domain.ApplicationStatus, riskZone *domain.RiskZone) ([]*domain.Application, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*domain.Application
	for _, app := range r.applications {
		if status != nil && *status != "" && app.Status != *status {
			continue
		}
		if riskZone != nil && *riskZone != "" {
			if app.ScoringResult == nil || app.ScoringResult.RiskZone != *riskZone {
				continue
			}
		}
		cp := *app
		result = append(result, &cp)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result, nil
}

func (r *InMemoryApplicationRepository) AddDocument(ctx context.Context, tx pgx.Tx, appID string, doc *domain.Document) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	app, exists := r.applications[appID]
	if !exists {
		return domain.ErrNotFound(fmt.Sprintf("Заявка с ID '%s' не найдена", appID), nil)
	}

	if doc.UploadedAt.IsZero() {
		doc.UploadedAt = time.Now()
	}

	app.Documents = append(app.Documents, *doc)
	return nil
}

func (r *InMemoryApplicationRepository) UpdateStatus(
	ctx context.Context,
	tx pgx.Tx,
	id string,
	oldStatus, newStatus domain.ApplicationStatus,
	actorRole string,
	comment *string,
	extra *domain.ApplicationUpdateFields,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	app, exists := r.applications[id]
	if !exists {
		return domain.ErrNotFound(fmt.Sprintf("Заявка с ID '%s' не найдена", id), nil)
	}

	if app.Status != oldStatus {
		return domain.ErrInvalidStateTransition(app.Status, newStatus)
	}

	app.Status = newStatus
	app.UpdatedAt = time.Now()

	if extra != nil {
		if extra.ApprovedAmount != nil {
			app.ApprovedAmount = extra.ApprovedAmount
		}
		if extra.ApprovedTermMonths != nil {
			app.ApprovedTermMonths = extra.ApprovedTermMonths
		}
		if extra.InterestRate != nil {
			app.InterestRate = extra.InterestRate
		}
		if extra.CounterOffer != nil {
			app.CounterOffer = extra.CounterOffer
		}
		if extra.ScoringResult != nil {
			app.ScoringResult = extra.ScoringResult
		}
		if extra.CompanySnapshot != nil {
			app.CompanySnapshot = extra.CompanySnapshot
		}
		if extra.UnderwriterComment != nil {
			app.UnderwriterComment = extra.UnderwriterComment
		}
		if extra.RejectReasonClient != nil {
			app.RejectReasonClient = extra.RejectReasonClient
		}
	}

	app.StatusHistory = append(app.StatusHistory, domain.AuditLogEntry{
		Timestamp:  app.UpdatedAt,
		FromStatus: oldStatus,
		ToStatus:   newStatus,
		ActorRole:  actorRole,
		Comment:    comment,
	})

	return nil
}
