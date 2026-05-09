-- +goose Up
-- +goose StatementBegin

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users (multi-tenant ready, hardcoded to id=1 in single-user mode)
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email TEXT UNIQUE,
    display_name TEXT,
    locale TEXT NOT NULL DEFAULT 'en',
    timezone TEXT NOT NULL DEFAULT 'America/New_York',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Visa types as a reference table, not a Postgres enum.
-- Adding a new visa = INSERT, not a schema migration.
CREATE TABLE visa_types (
    code TEXT PRIMARY KEY,
    display_name_en TEXT NOT NULL,
    display_name_ja TEXT NOT NULL,
    rule_file TEXT NOT NULL
);

INSERT INTO visa_types (code, display_name_en, display_name_ja, rule_file) VALUES
    ('jfind', 'J-FIND (Future Creation Activities)', '特定活動（未来創造人材）', 'jfind.yaml'),
    ('engineer', 'Engineer/Specialist in Humanities/International Services', '技術・人文知識・国際業務', 'engineer.yaml');

-- A user's visa instance
CREATE TABLE visas (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    visa_type_code TEXT NOT NULL REFERENCES visa_types(code),
    status TEXT NOT NULL CHECK (status IN (
        'planning', 'applied', 'approved', 'landed', 'active',
        'renewal_window', 'renewed', 'expired', 'changed_to_other'
    )),
    coe_number TEXT,
    coe_issued_at DATE,
    landed_at DATE,
    residence_card_number TEXT,
    residence_card_issued_at DATE,
    period_of_stay_months INT,
    expires_at DATE,
    sponsor_name TEXT,
    sponsor_address TEXT,
    job_title TEXT,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_visas_user_id ON visas(user_id);
CREATE INDEX idx_visas_user_active ON visas(user_id, status)
    WHERE status IN ('landed', 'active', 'renewal_window');

-- Life events: what happened, that triggers tasks
CREATE TABLE life_events (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    visa_id BIGINT REFERENCES visas(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    occurred_at DATE NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_life_events_user_id ON life_events(user_id);
CREATE INDEX idx_life_events_visa_id ON life_events(visa_id);
CREATE INDEX idx_life_events_type ON life_events(event_type);

-- Compliance tasks: what the user must do
CREATE TABLE compliance_tasks (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    visa_id BIGINT REFERENCES visas(id) ON DELETE CASCADE,
    rule_id TEXT NOT NULL,
    triggered_by_event_id BIGINT REFERENCES life_events(id) ON DELETE SET NULL,
    title_en TEXT NOT NULL,
    title_ja TEXT,
    description_en TEXT NOT NULL,
    description_ja TEXT,
    category TEXT NOT NULL CHECK (category IN (
        'pre_arrival', 'immigration', 'municipal', 'tax',
        'health_insurance', 'pension', 'banking', 'telecom',
        'employer', 'housing', 'general'
    )),
    severity TEXT NOT NULL DEFAULT 'mandatory'
        CHECK (severity IN ('mandatory', 'recommended', 'informational')),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'in_progress', 'done', 'overdue', 'not_applicable', 'skipped')),
    deadline_at DATE,
    completed_at TIMESTAMPTZ,
    legal_source_url TEXT,
    legal_source_text TEXT,
    location_hint TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Idempotency: re-firing the same event won't double-insert tasks for the same rule.
    UNIQUE (user_id, rule_id, triggered_by_event_id)
);

CREATE INDEX idx_tasks_user_id ON compliance_tasks(user_id);
CREATE INDEX idx_tasks_user_status ON compliance_tasks(user_id, status);
CREATE INDEX idx_tasks_user_deadline ON compliance_tasks(user_id, deadline_at)
    WHERE status = 'pending';

-- Task dependencies (DAG)
CREATE TABLE task_dependencies (
    task_id BIGINT NOT NULL REFERENCES compliance_tasks(id) ON DELETE CASCADE,
    depends_on_task_id BIGINT NOT NULL REFERENCES compliance_tasks(id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, depends_on_task_id),
    CHECK (task_id != depends_on_task_id)
);

-- Documents (Phase 2 fills these; schema ready now)
CREATE TABLE documents (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    document_type TEXT,
    storage_key TEXT NOT NULL,
    original_filename TEXT,
    mime_type TEXT,
    file_size_bytes BIGINT,
    issuing_authority TEXT,
    issued_at DATE,
    expires_at DATE,
    ocr_text TEXT,
    parsed_data JSONB,
    summary_en TEXT,
    summary_ja TEXT,
    classification_confidence REAL,
    parser_version TEXT,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    parsed_at TIMESTAMPTZ
);

CREATE INDEX idx_documents_user_id ON documents(user_id);
CREATE INDEX idx_documents_type ON documents(user_id, document_type);

-- Many-to-many: documents linked to tasks they fulfill
CREATE TABLE document_task_links (
    document_id BIGINT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    task_id BIGINT NOT NULL REFERENCES compliance_tasks(id) ON DELETE CASCADE,
    link_type TEXT NOT NULL
        CHECK (link_type IN ('proves_completion', 'required_input', 'output')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (document_id, task_id, link_type)
);

-- Addresses (current and historical)
CREATE TABLE addresses (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    full_address TEXT NOT NULL,
    postal_code TEXT,
    prefecture TEXT,
    city TEXT,
    ward TEXT,
    moved_in_at DATE NOT NULL,
    moved_out_at DATE,
    registered_at_ward_office BOOLEAN NOT NULL DEFAULT FALSE,
    ward_office_name TEXT,
    is_current BOOLEAN GENERATED ALWAYS AS (moved_out_at IS NULL) STORED,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_addresses_user_current ON addresses(user_id) WHERE moved_out_at IS NULL;

-- Employers (current and historical)
CREATE TABLE employers (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    address TEXT,
    role TEXT,
    employment_type TEXT CHECK (employment_type IN (
        'full_time_japan', 'part_time_japan', 'remote_overseas',
        'freelance_overseas', 'freelance_japan', 'none'
    )),
    started_at DATE NOT NULL,
    ended_at DATE,
    immigration_notified_at DATE,
    is_current BOOLEAN GENERATED ALWAYS AS (ended_at IS NULL) STORED,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_employers_user_current ON employers(user_id) WHERE ended_at IS NULL;

-- Reminders
CREATE TABLE reminders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    task_id BIGINT NOT NULL REFERENCES compliance_tasks(id) ON DELETE CASCADE,
    fire_at TIMESTAMPTZ NOT NULL,
    reminder_type TEXT NOT NULL
        CHECK (reminder_type IN ('advance_30d', 'advance_14d', 'advance_3d', 'day_of', 'overdue')),
    delivered_at TIMESTAMPTZ,
    delivery_channel TEXT CHECK (delivery_channel IN ('email', 'push', 'in_app'))
);

CREATE INDEX idx_reminders_pending ON reminders(fire_at) WHERE delivered_at IS NULL;
CREATE INDEX idx_reminders_user ON reminders(user_id);

-- Reusable updated_at trigger
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER visas_updated_at BEFORE UPDATE ON visas
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER tasks_updated_at BEFORE UPDATE ON compliance_tasks
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Seed single user for personal-use phase
INSERT INTO users (id, email, display_name, locale, timezone)
VALUES (1, 'me@local', 'Me', 'en', 'America/New_York');

SELECT setval('users_id_seq', (SELECT MAX(id) FROM users));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS reminders CASCADE;
DROP TABLE IF EXISTS employers CASCADE;
DROP TABLE IF EXISTS addresses CASCADE;
DROP TABLE IF EXISTS document_task_links CASCADE;
DROP TABLE IF EXISTS documents CASCADE;
DROP TABLE IF EXISTS task_dependencies CASCADE;
DROP TABLE IF EXISTS compliance_tasks CASCADE;
DROP TABLE IF EXISTS life_events CASCADE;
DROP TABLE IF EXISTS visas CASCADE;
DROP TABLE IF EXISTS visa_types CASCADE;
DROP TABLE IF EXISTS users CASCADE;
DROP FUNCTION IF EXISTS set_updated_at;
-- +goose StatementEnd
