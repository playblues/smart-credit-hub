package domain

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
	ID          string  `json:"id"`
	Login       string  `json:"login"`
	Password    string  `json:"-"`
	Role        Role    `json:"role"`
	INN         *string `json:"inn,omitempty"`
	DisplayName string  `json:"display_name"`
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
	Status string `json:"status"`
}

// TestCaseItem структура элемента из scoring/testdata/cases.json.
type TestCaseItem struct {
	CaseID      string            `json:"case_id"`
	Description string            `json:"description"`
	Company     FNSCompany        `json:"company"`
	BKI         BKICreditHistory  `json:"bki"`
}
