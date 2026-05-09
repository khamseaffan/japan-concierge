-- name: CreateComplianceTask :one
-- Inserts a generated task. Uses ON CONFLICT DO NOTHING because the schema
-- enforces idempotency via UNIQUE (user_id, rule_id, triggered_by_event_id).
-- If the task already exists, this returns no rows. The caller decides whether
-- that's a no-op or an error.
INSERT INTO compliance_tasks (
    user_id, visa_id, rule_id, triggered_by_event_id,
    title_en, title_ja, description_en, description_ja,
    category, severity, status,
    deadline_at, legal_source_url, legal_source_text, location_hint,
    metadata
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7, $8,
    $9, $10, $11,
    $12, $13, $14, $15,
    $16
)
ON CONFLICT (user_id, rule_id, triggered_by_event_id) DO NOTHING
RETURNING id, user_id, visa_id, rule_id, triggered_by_event_id,
    title_en, title_ja, description_en, description_ja,
    category, severity, status,
    deadline_at, completed_at,
    legal_source_url, legal_source_text, location_hint,
    metadata, created_at, updated_at;

-- name: GetComplianceTask :one
SELECT id, user_id, visa_id, rule_id, triggered_by_event_id,
    title_en, title_ja, description_en, description_ja,
    category, severity, status,
    deadline_at, completed_at,
    legal_source_url, legal_source_text, location_hint,
    metadata, created_at, updated_at
FROM compliance_tasks
WHERE id = $1 AND user_id = $2;

-- name: ListPendingTasksForUser :many
SELECT id, user_id, visa_id, rule_id, triggered_by_event_id,
    title_en, title_ja, description_en, description_ja,
    category, severity, status,
    deadline_at, completed_at,
    legal_source_url, legal_source_text, location_hint,
    metadata, created_at, updated_at
FROM compliance_tasks
WHERE user_id = $1 AND status IN ('pending', 'in_progress', 'overdue')
ORDER BY
    CASE WHEN deadline_at IS NULL THEN 1 ELSE 0 END,
    deadline_at ASC,
    created_at ASC;

-- name: ListAllTasksForUser :many
SELECT id, user_id, visa_id, rule_id, triggered_by_event_id,
    title_en, title_ja, description_en, description_ja,
    category, severity, status,
    deadline_at, completed_at,
    legal_source_url, legal_source_text, location_hint,
    metadata, created_at, updated_at
FROM compliance_tasks
WHERE user_id = $1
ORDER BY
    CASE WHEN deadline_at IS NULL THEN 1 ELSE 0 END,
    deadline_at ASC,
    created_at ASC;

-- name: ListTasksTriggeredByEvent :many
-- Returns tasks created by a specific life event. Used by the service layer
-- to return the freshly-generated tasks from RecordLifeEvent, so the HTTP
-- response can echo what was created without a follow-up query.
SELECT id, user_id, visa_id, rule_id, triggered_by_event_id,
    title_en, title_ja, description_en, description_ja,
    category, severity, status,
    deadline_at, completed_at,
    legal_source_url, legal_source_text, location_hint,
    metadata, created_at, updated_at
FROM compliance_tasks
WHERE user_id = $1 AND triggered_by_event_id = $2
ORDER BY
    CASE WHEN deadline_at IS NULL THEN 1 ELSE 0 END,
    deadline_at ASC,
    created_at ASC;

-- name: MarkTaskDone :one
UPDATE compliance_tasks
SET status = 'done', completed_at = NOW()
WHERE id = $1 AND user_id = $2 AND status != 'done'
RETURNING id, user_id, visa_id, rule_id, triggered_by_event_id,
    title_en, title_ja, description_en, description_ja,
    category, severity, status,
    deadline_at, completed_at,
    legal_source_url, legal_source_text, location_hint,
    metadata, created_at, updated_at;
