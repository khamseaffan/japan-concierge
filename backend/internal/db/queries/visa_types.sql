-- name: ListVisaTypes :many
SELECT code, display_name_en, display_name_ja, rule_file
FROM visa_types
ORDER BY code;

-- name: GetVisaType :one
SELECT code, display_name_en, display_name_ja, rule_file
FROM visa_types
WHERE code = $1;
