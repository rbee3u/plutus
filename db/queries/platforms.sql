-- name: GetPlatformByCode :one
SELECT id, code, name, status, created_at, updated_at
FROM platforms
WHERE code = sqlc.arg(code);

-- name: GetPlatformByID :one
SELECT id, code, name, status, created_at, updated_at
FROM platforms
WHERE id = sqlc.arg(id);

-- name: ListPlatforms :many
SELECT id, code, name, status, created_at, updated_at
FROM platforms
ORDER BY code;
