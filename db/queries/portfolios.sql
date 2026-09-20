-- name: InsertPortfolio :exec
INSERT INTO portfolios (
    id, user_id, name, timezone, display_asset_id, status, version, created_at, updated_at, archived_at
) VALUES (
    sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(name), sqlc.arg(timezone), sqlc.arg(display_asset_id),
    sqlc.arg(status), sqlc.arg(version), sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.arg(archived_at)
);
