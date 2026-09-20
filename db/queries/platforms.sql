-- name: GetPlatformByCode :one
SELECT id, code, name, status, created_at, updated_at
FROM platforms
WHERE code = sqlc.arg(code);

-- name: ListPlatforms :many
SELECT id, code, name, status, created_at, updated_at
FROM platforms
ORDER BY code;
