package domain

import "time"

// Role определяет роль пользователя в кредитном конвейере.
type Role string

const (
	// RoleClient - клиент (представитель малого бизнеса).
	RoleClient Role = "CLIENT"
	// RoleUnderwriter - сотрудник банка (андеррайтер / кредитный инспектор).
	RoleUnderwriter Role = "UNDERWRITER"
)

// IsValid проверяет валидность роли.
func (r Role) IsValid() bool {
	return r == RoleClient || r == RoleUnderwriter
}

// User представляет пользователя системы.
type User struct {
	ID           string     `json:"id"`
	Login        string     `json:"login"`
	PasswordHash string     `json:"-"`
	Password     string     `json:"-"`
	Role         Role       `json:"role"`
	INN          *string    `json:"inn,omitempty"`
	DisplayName  string     `json:"display_name"`
	CreatedAt    *time.Time `json:"created_at,omitempty"`
}

// CheckPassword проверяет совпадение переданного пароля с паролем пользователя.
func (u *User) CheckPassword(plain string) bool {
	if u.PasswordHash != "" && u.PasswordHash == plain {
		return true
	}
	if u.Password != "" && u.Password == plain {
		return true
	}
	return false
}

// LoginRequest DTO запроса на аутентификацию (POST /api/v1/auth/login).
type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// LoginResponse DTO успешного ответа на аутентификацию.
type LoginResponse struct {
	Token       string  `json:"token"`
	Role        Role    `json:"role"`
	INN         *string `json:"inn"`
	DisplayName string  `json:"display_name"`
}

// FNSCompany DTO ответа мок-сервиса ФНС (GET /api/v1/mock/fns/{inn}).
type FNSCompany struct {
	INN               string  `json:"inn"`
	Name              string  `json:"name"`
	OKVED             string  `json:"okved"`
	AnnualRevenue     float64 `json:"annual_revenue"`
	BusinessAgeMonths int     `json:"business_age_months"`
	IsLiquidating     bool    `json:"is_liquidating"`
}

// BKICreditHistory DTO ответа мок-сервиса БКИ (GET /api/v1/mock/bki/{inn}).
type BKICreditHistory struct {
	INN                string `json:"inn"`
	ActiveLoansCount   int    `json:"active_loans_count"`
	MaxOverdueDays     int    `json:"max_overdue_days"`
	HadPastDelinquency bool   `json:"had_past_delinquency"`
}

// HealthResponse DTO проверки жизнеспособности сервиса (GET /healthz).
type HealthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database,omitempty"`
}

// TestCaseItem структура элемента из scoring/testdata/cases.json.
type TestCaseItem struct {
	CaseID      string           `json:"case_id"`
	Description string           `json:"description"`
	Company     FNSCompany       `json:"company"`
	BKI         BKICreditHistory `json:"bki"`
}

// ApplicationStatus константы статусов заявки согласно docs/openapi.yaml и state-machine.md
type ApplicationStatus string

const (
	StatusSubmitted        ApplicationStatus = "SUBMITTED"
	StatusEnrichment       ApplicationStatus = "ENRICHMENT"
	StatusScoring          ApplicationStatus = "SCORING"
	StatusAutoApproved     ApplicationStatus = "AUTO_APPROVED"
	StatusCounterOffered   ApplicationStatus = "COUNTER_OFFERED"
	StatusClientDeclined   ApplicationStatus = "CLIENT_DECLINED"
	StatusManualReview     ApplicationStatus = "MANUAL_REVIEW"
	StatusWaitingDocuments ApplicationStatus = "WAITING_DOCUMENTS"
	StatusApproved         ApplicationStatus = "APPROVED"
	StatusRejected         ApplicationStatus = "REJECTED"
	StatusSigned           ApplicationStatus = "SIGNED"
	StatusDisbursed        ApplicationStatus = "DISBURSED"
)

// ProductType кредитного продукта
type ProductType string

const (
	ProductWorkingCapital ProductType = "WORKING_CAPITAL"
	ProductOverdraft      ProductType = "OVERDRAFT"
)

// RiskZone зоны риска скоринга
type RiskZone string

const (
	RiskZoneGreen  RiskZone = "GREEN"
	RiskZoneYellow RiskZone = "YELLOW"
	RiskZoneGrey   RiskZone = "GREY"
	RiskZoneRed    RiskZone = "RED"
)

// DocumentType тип прикрепленного документа
type DocumentType string

const (
	DocFinancialStatement DocumentType = "FINANCIAL_STATEMENT"
	DocTaxDeclaration     DocumentType = "TAX_DECLARATION"
	DocBankStatement      DocumentType = "BANK_STATEMENT"
	DocLeaseContract      DocumentType = "LEASE_CONTRACT"
	DocOther              DocumentType = "OTHER"
)

// CounterOffer встречное предложение банка
type CounterOffer struct {
	Amount     float64 `json:"amount"`
	TermMonths int     `json:"term_months"`
	Rate       float64 `json:"rate"`
	PTI        float64 `json:"pti"`
	Reason     string  `json:"reason"`
}

// ScoringResult результат скоринговой модели
type ScoringResult struct {
	ZScore        float64  `json:"z_score"`
	PD            float64  `json:"pd"`
	Score         int      `json:"score"`
	RiskZone      RiskZone `json:"risk_zone"`
	StopFactor    *string  `json:"stop_factor"`
	FactorsClient []string `json:"factors_client"`
	FactorsBank   []string `json:"factors_bank"`
	ModelVersion  string   `json:"model_version"`
}

// CompanySnapshot снапшот данных ФНС и БКИ на момент подачи заявки
type CompanySnapshot struct {
	INN                   string  `json:"inn"`
	Name                  string  `json:"name"`
	AnnualRevenue         float64 `json:"annual_revenue"`
	BusinessAgeMonths     int     `json:"business_age_months"`
	IsLiquidating         bool    `json:"is_liquidating"`
	BKIActiveLoans        int     `json:"bki_active_loans"`
	BKIMaxOverdueDays     int     `json:"bki_max_overdue_days"`
	BKIHadPastDelinquency bool    `json:"bki_had_past_delinquency"`
}

// Document описание загруженного документа
type Document struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	DocumentType DocumentType `json:"document_type"`
	FilePath     string       `json:"-"`
	UploadedAt   time.Time    `json:"uploaded_at"`
	SizeBytes    int64        `json:"size_bytes"`
}

// AuditLogEntry запись в журнале аудита статусов
type AuditLogEntry struct {
	Timestamp  time.Time         `json:"timestamp"`
	FromStatus ApplicationStatus `json:"from_status"`
	ToStatus   ApplicationStatus `json:"to_status"`
	ActorRole  string            `json:"actor_role"`
	Comment    *string           `json:"comment"`
}

// CreateApplicationRequest DTO запроса на создание заявки (POST /api/v1/applications).
type CreateApplicationRequest struct {
	INN                 string      `json:"inn"`
	ProductType         ProductType `json:"product_type"`
	RequestedAmount     float64     `json:"requested_amount"`
	RequestedTermMonths int         `json:"requested_term_months"`
	Purpose             *string     `json:"purpose,omitempty"`
}

// ApplicationUpdateFields поля обновления заявки при переходе статуса.
type ApplicationUpdateFields struct {
	ApprovedAmount     *float64
	ApprovedTermMonths *int
	InterestRate       *float64
	CounterOffer       *CounterOffer
	ScoringResult      *ScoringResult
	CompanySnapshot    *CompanySnapshot
	UnderwriterComment *string
	RejectReasonClient *string
}

// Application кредитная заявка
type Application struct {
	ID                  string            `json:"id"`
	UserID              *string           `json:"user_id,omitempty"`
	INN                 string            `json:"inn"`
	Status              ApplicationStatus `json:"status"`
	ProductType         ProductType       `json:"product_type"`
	Purpose             *string           `json:"purpose"`
	RequestedAmount     float64           `json:"requested_amount"`
	RequestedTermMonths int               `json:"requested_term_months"`
	ApprovedAmount      *float64          `json:"approved_amount"`
	ApprovedTermMonths  *int              `json:"approved_term_months"`
	InterestRate        *float64          `json:"interest_rate"`
	CounterOffer        *CounterOffer     `json:"counter_offer"`
	ScoringResult       *ScoringResult    `json:"scoring_result"`
	CompanySnapshot     *CompanySnapshot  `json:"company_snapshot"`
	UnderwriterComment  *string           `json:"underwriter_comment"`
	RejectReasonClient  *string           `json:"reject_reason_client"`
	Documents           []Document        `json:"documents"`
	StatusHistory       []AuditLogEntry   `json:"status_history"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}
