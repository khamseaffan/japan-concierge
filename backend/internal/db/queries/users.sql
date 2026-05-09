-- name: GetUser :one
SELECT id, email, display_name, locale, timezone, created_at, updated_at
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, display_name, locale, timezone, created_at, updated_at
FROM users
WHERE email = $1;
