-- Включение расширения для генерации UUID
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 1. Таблица пользователей
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    login VARCHAR(64) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(32) NOT NULL,
    inn VARCHAR(12),
    display_name VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Индекс по ИНН для быстрого поиска клиента
CREATE INDEX IF NOT EXISTS idx_users_inn ON users(inn);

-- 2. Таблица кредитных заявок
CREATE TABLE IF NOT EXISTS applications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id),
    inn VARCHAR(12) NOT NULL,
    status VARCHAR(32) NOT NULL,
    product_type VARCHAR(32) NOT NULL,
    purpose TEXT,
    requested_amount NUMERIC(15,2) NOT NULL,
    requested_term_months INT NOT NULL,
    approved_amount NUMERIC(15,2),
    approved_term_months INT,
    interest_rate NUMERIC(6,4),
    counter_offer JSONB,
    scoring_result JSONB,
    company_snapshot JSONB,
    underwriter_comment TEXT,
    reject_reason_client TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Индексы для фильтрации и поиска заявок
CREATE INDEX IF NOT EXISTS idx_applications_inn ON applications(inn);
CREATE INDEX IF NOT EXISTS idx_applications_status ON applications(status);
CREATE INDEX IF NOT EXISTS idx_applications_user_id ON applications(user_id);

-- 3. Таблица документов
CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    application_id UUID REFERENCES applications(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    document_type VARCHAR(64) NOT NULL,
    file_path VARCHAR(512) NOT NULL,
    size_bytes BIGINT NOT NULL,
    uploaded_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_documents_app_id ON documents(application_id);

-- 4. Таблица аудита изменения статусов (Audit Log)
CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL PRIMARY KEY,
    application_id UUID REFERENCES applications(id) ON DELETE CASCADE,
    from_status VARCHAR(32) NOT NULL,
    to_status VARCHAR(32) NOT NULL,
    actor_role VARCHAR(32) NOT NULL,
    comment TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_app_id ON audit_logs(application_id);
