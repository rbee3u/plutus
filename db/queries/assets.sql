-- name: GetAssetByCode :one
SELECT id, code, name, kind, decimals, status, created_at, updated_at
FROM assets
WHERE code = sqlc.arg(code);

-- name: ListActiveAssets :many
SELECT id, code, name, kind, decimals, status, created_at, updated_at
FROM assets
WHERE status = 'active'
ORDER BY code;
