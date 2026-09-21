-- name: InsertPortfolio :exec
INSERT INTO portfolios (
    id, user_id, name, timezone, display_asset_id, status, undeletable, version, created_at, updated_at, archived_at
) VALUES (
    sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(name), sqlc.arg(timezone), sqlc.arg(display_asset_id),
    sqlc.arg(status), sqlc.arg(undeletable), sqlc.arg(version), sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.arg(archived_at)
);

-- name: ListPortfoliosByUser :many
SELECT
    p.id, p.user_id, p.name, p.timezone, p.display_asset_id, p.status, p.undeletable, p.version,
    p.created_at, p.updated_at, p.archived_at, a.code AS display_asset_code
FROM portfolios p
JOIN assets a ON a.id = p.display_asset_id
WHERE p.user_id = sqlc.arg(user_id)
  AND p.status = 'active'
ORDER BY p.created_at ASC, p.id ASC;

-- name: GetPortfolioByUserAndID :one
SELECT
    p.id, p.user_id, p.name, p.timezone, p.display_asset_id, p.status, p.undeletable, p.version,
    p.created_at, p.updated_at, p.archived_at, a.code AS display_asset_code
FROM portfolios p
JOIN assets a ON a.id = p.display_asset_id
WHERE p.user_id = sqlc.arg(user_id) AND p.id = sqlc.arg(id);

-- name: GetPortfolioByID :one
SELECT
    p.id, p.user_id, p.name, p.timezone, p.display_asset_id, p.status, p.undeletable, p.version,
    p.created_at, p.updated_at, p.archived_at, a.code AS display_asset_code,
    p.active_projection_generation_id
FROM portfolios p
JOIN assets a ON a.id = p.display_asset_id
WHERE p.id = sqlc.arg(id);

-- name: UpdatePortfolio :execrows
UPDATE portfolios
SET name = sqlc.arg(name),
    timezone = sqlc.arg(timezone),
    display_asset_id = sqlc.arg(display_asset_id),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND version = sqlc.arg(version);

-- name: DeletePortfolio :execrows
DELETE FROM portfolios
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND version = sqlc.arg(version);
