-- name: CreateLifeEvent :one
INSERT INTO life_events (user_id, visa_id, event_type, occurred_at, payload)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, user_id, visa_id, event_type, occurred_at, payload, created_at;

-- name: GetLifeEvent :one
SELECT id, user_id, visa_id, event_type, occurred_at, payload, created_at
FROM life_events
WHERE id = $1 AND user_id = $2;

-- name: ListLifeEventsForUser :many
SELECT id, user_id, visa_id, event_type, occurred_at, payload, created_at
FROM life_events
WHERE user_id = $1
ORDER BY occurred_at DESC, created_at DESC
LIMIT $2 OFFSET $3;
