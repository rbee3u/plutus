-- name: InsertPortfolio :exec
INSERT INTO portfolios (
    id, user_id, name, name_key, timezone, collection_interval_seconds, is_initial, version, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(name), sqlc.arg(name_key), sqlc.arg(timezone), sqlc.arg(collection_interval_seconds),
    sqlc.arg(is_initial), sqlc.arg(version), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: ListPortfoliosByUser :many
SELECT id, user_id, name, name_key, timezone, collection_interval_seconds, is_initial, version, created_at, updated_at, deletion_requested_at
FROM portfolios
WHERE user_id = sqlc.arg(user_id)
ORDER BY created_at ASC, id ASC;

-- name: GetPortfolioByUserAndID :one
SELECT id, user_id, name, name_key, timezone, collection_interval_seconds, is_initial, version, created_at, updated_at, deletion_requested_at
FROM portfolios
WHERE user_id = sqlc.arg(user_id) AND id = sqlc.arg(id);

-- name: GetPortfolioByID :one
SELECT id, user_id, name, name_key, timezone, collection_interval_seconds, is_initial, version, created_at, updated_at, deletion_requested_at
FROM portfolios
WHERE id = sqlc.arg(id);

-- name: UpdatePortfolio :execrows
UPDATE portfolios
SET name = sqlc.arg(name),
    name_key = sqlc.arg(name_key),
    timezone = sqlc.arg(timezone),
    collection_interval_seconds = sqlc.arg(collection_interval_seconds),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND version = sqlc.arg(version) AND deletion_requested_at IS NULL;

-- name: DeletePortfolio :execrows
DELETE FROM portfolios
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND version = sqlc.arg(version);

-- name: ListPortfoliosForScheduling :many
SELECT p.id, p.collection_interval_seconds,
       (SELECT MAX(j.created_at) FROM collection_cycles j WHERE j.portfolio_id = p.id) AS last_cycle_at
FROM portfolios p
WHERE p.deletion_requested_at IS NULL AND EXISTS (
    SELECT 1 FROM identities i
    WHERE i.portfolio_id = p.id AND i.archived_at IS NULL AND i.connection_status = 'enabled'
)
ORDER BY p.created_at ASC, p.id ASC;

-- A consistent read avoids locking every scanned Portfolio row; the caller
-- then locks the chosen row by primary key.
-- name: GetNextDeletingPortfolioID :one
SELECT id FROM portfolios WHERE deletion_requested_at IS NOT NULL
ORDER BY deletion_requested_at ASC, id ASC LIMIT 1;

-- name: LockDeletingPortfolio :one
SELECT * FROM portfolios WHERE id = ? AND deletion_requested_at IS NOT NULL FOR UPDATE SKIP LOCKED;

-- name: RequestPortfolioDeletion :execrows
UPDATE portfolios SET deletion_requested_at = sqlc.narg(requested_at), version = version + 1, updated_at = sqlc.arg(at)
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND version = sqlc.arg(version)
  AND NOT is_initial AND deletion_requested_at IS NULL;

-- name: LockRetentionPortfolio :one
SELECT * FROM portfolios WHERE id = ? AND deletion_requested_at IS NULL FOR UPDATE;
