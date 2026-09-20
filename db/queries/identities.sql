-- name: InsertIdentity :exec
INSERT INTO identities (
    id, portfolio_id, platform_id, parent_identity_id, external_ref, display_name,
    account_mode, kind, status, version, created_at, updated_at, archived_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(platform_id), sqlc.narg(parent_identity_id),
    sqlc.arg(external_ref), sqlc.arg(display_name), sqlc.arg(account_mode), sqlc.arg(kind),
    sqlc.arg(status), sqlc.arg(version), sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.arg(archived_at)
);

-- name: ListIdentitiesByPortfolio :many
SELECT id, portfolio_id, platform_id, parent_identity_id, external_ref, display_name,
       account_mode, kind, status, version, created_at, updated_at, archived_at
FROM identities
WHERE portfolio_id = sqlc.arg(portfolio_id)
ORDER BY created_at ASC, id ASC;

-- name: GetIdentity :one
SELECT id, portfolio_id, platform_id, parent_identity_id, external_ref, display_name,
       account_mode, kind, status, version, created_at, updated_at, archived_at
FROM identities
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: UpdateIdentity :execrows
UPDATE identities
SET display_name = sqlc.arg(display_name),
    status = sqlc.arg(status),
    parent_identity_id = sqlc.narg(parent_identity_id),
    external_ref = sqlc.arg(external_ref),
    account_mode = sqlc.arg(account_mode),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version);

-- name: ArchiveIdentity :execrows
UPDATE identities
SET status = 'archived',
    archived_at = sqlc.arg(archived_at),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version) AND status <> 'archived';
