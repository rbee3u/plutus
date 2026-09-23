-- name: InsertPortfolio :exec
INSERT INTO portfolios (
    id, user_id, name, timezone, collection_interval_seconds, undeletable, version, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(name), sqlc.arg(timezone), sqlc.arg(collection_interval_seconds),
    sqlc.arg(undeletable), sqlc.arg(version), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: ListPortfoliosByUser :many
SELECT id, user_id, name, timezone, collection_interval_seconds, undeletable, version, created_at, updated_at
FROM portfolios
WHERE user_id = sqlc.arg(user_id)
ORDER BY created_at ASC, id ASC;

-- name: GetPortfolioByUserAndID :one
SELECT id, user_id, name, timezone, collection_interval_seconds, undeletable, version, created_at, updated_at
FROM portfolios
WHERE user_id = sqlc.arg(user_id) AND id = sqlc.arg(id);

-- name: GetPortfolioByID :one
SELECT id, user_id, name, timezone, collection_interval_seconds, undeletable, version, created_at, updated_at
FROM portfolios
WHERE id = sqlc.arg(id);

-- name: UpdatePortfolio :execrows
UPDATE portfolios
SET name = sqlc.arg(name),
    timezone = sqlc.arg(timezone),
    collection_interval_seconds = sqlc.arg(collection_interval_seconds),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND version = sqlc.arg(version);

-- name: DeletePortfolio :execrows
DELETE FROM portfolios
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND version = sqlc.arg(version);

-- name: ListPortfoliosForScheduling :many
SELECT p.id, p.collection_interval_seconds,
       (SELECT MAX(j.created_at) FROM jobs j WHERE j.portfolio_id = p.id AND j.kind = 'collection_cycle') AS last_cycle_at
FROM portfolios p
WHERE EXISTS (
    SELECT 1 FROM identities i
    WHERE i.portfolio_id = p.id AND i.status = 'enabled' AND i.connection_status = 'enabled'
)
ORDER BY p.created_at ASC, p.id ASC;
