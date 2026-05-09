-- name: GetActiveVisaForUser :one
-- Returns the user's most relevant in-progress or active visa.
-- Order prefers active/landed status over planning/applied.
SELECT
    id, user_id, visa_type_code, status,
    coe_number, coe_issued_at, landed_at,
    residence_card_number, residence_card_issued_at,
    period_of_stay_months, expires_at,
    sponsor_name, sponsor_address, job_title, notes,
    created_at, updated_at
FROM visas
WHERE user_id = $1
ORDER BY
    CASE status
        WHEN 'active' THEN 1
        WHEN 'landed' THEN 2
        WHEN 'renewal_window' THEN 3
        WHEN 'approved' THEN 4
        WHEN 'applied' THEN 5
        WHEN 'planning' THEN 6
        ELSE 99
    END,
    created_at DESC
LIMIT 1;

-- name: GetVisaByID :one
SELECT
    id, user_id, visa_type_code, status,
    coe_number, coe_issued_at, landed_at,
    residence_card_number, residence_card_issued_at,
    period_of_stay_months, expires_at,
    sponsor_name, sponsor_address, job_title, notes,
    created_at, updated_at
FROM visas
WHERE id = $1 AND user_id = $2;

-- name: ListVisasForUser :many
SELECT
    id, user_id, visa_type_code, status,
    coe_number, coe_issued_at, landed_at,
    residence_card_number, residence_card_issued_at,
    period_of_stay_months, expires_at,
    sponsor_name, sponsor_address, job_title, notes,
    created_at, updated_at
FROM visas
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: CreateVisa :one
INSERT INTO visas (
    user_id, visa_type_code, status,
    coe_number, coe_issued_at, landed_at,
    residence_card_number, residence_card_issued_at,
    period_of_stay_months, expires_at,
    sponsor_name, sponsor_address, job_title, notes
) VALUES (
    $1, $2, $3,
    $4, $5, $6,
    $7, $8,
    $9, $10,
    $11, $12, $13, $14
)
RETURNING id, user_id, visa_type_code, status,
    coe_number, coe_issued_at, landed_at,
    residence_card_number, residence_card_issued_at,
    period_of_stay_months, expires_at,
    sponsor_name, sponsor_address, job_title, notes,
    created_at, updated_at;
