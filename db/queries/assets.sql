-- name: GetAssetByCode :one
SELECT id, code, name, kind, decimals, status, created_at, updated_at
FROM assets
WHERE code = sqlc.arg(code);

-- name: GetAssetByID :one
SELECT id, code, name, kind, decimals, status, created_at, updated_at
FROM assets
WHERE id = sqlc.arg(id);

-- name: ListActiveAssets :many
SELECT id, code, name, kind, decimals, status, created_at, updated_at
FROM assets
WHERE status = 'active'
ORDER BY code;

-- name: SearchAssets :many
SELECT id, code, name, kind, decimals, status, created_at, updated_at
FROM assets
WHERE status IN ('active', 'provisional')
  AND (sqlc.arg(query) = '' OR INSTR(LOWER(code), LOWER(sqlc.arg(query))) > 0 OR INSTR(LOWER(name), LOWER(sqlc.arg(query))) > 0)
ORDER BY code ASC, id ASC
LIMIT ? OFFSET ?;

-- name: CountSearchAssets :one
SELECT COUNT(*)
FROM assets
WHERE status IN ('active', 'provisional')
  AND (sqlc.arg(query) = '' OR INSTR(LOWER(code), LOWER(sqlc.arg(query))) > 0 OR INSTR(LOWER(name), LOWER(sqlc.arg(query))) > 0);
