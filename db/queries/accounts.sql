-- name: InsertAccount :exec
INSERT INTO accounts (
    id, portfolio_id, identity_id, account_type, display_name, selected, version, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(identity_id), sqlc.arg(account_type),
    sqlc.arg(display_name), sqlc.arg(selected), sqlc.arg(version), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: ListAccountsByPortfolio :many
SELECT a.id, a.portfolio_id, a.identity_id, a.account_type, a.display_name, a.selected, a.version,
       a.created_at, a.updated_at, i.platform, i.display_name AS identity_name, i.status AS identity_status
FROM accounts a
INNER JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE a.portfolio_id = sqlc.arg(portfolio_id)
ORDER BY i.platform ASC, i.created_at ASC, a.account_type ASC;

-- name: ListAccountsByIdentity :many
SELECT id, portfolio_id, identity_id, account_type, display_name, selected, version, created_at, updated_at
FROM accounts
WHERE portfolio_id = sqlc.arg(portfolio_id) AND identity_id = sqlc.arg(identity_id)
ORDER BY account_type ASC;

-- name: GetAccount :one
SELECT a.id, a.portfolio_id, a.identity_id, a.account_type, a.display_name, a.selected, a.version,
       a.created_at, a.updated_at, i.platform, i.display_name AS identity_name, i.status AS identity_status
FROM accounts a
INNER JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE a.portfolio_id = sqlc.arg(portfolio_id) AND a.id = sqlc.arg(id);

-- name: UpdateAccount :execrows
UPDATE accounts
SET display_name = sqlc.arg(display_name),
    selected = sqlc.arg(selected),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: ListBoundaryAccounts :many
SELECT a.id, a.portfolio_id, a.identity_id, a.account_type, a.display_name, i.platform, i.display_name AS identity_name,
       i.connection_status
FROM accounts a
INNER JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE a.portfolio_id = sqlc.arg(portfolio_id) AND a.selected = TRUE AND i.status = 'enabled'
ORDER BY i.platform ASC, i.created_at ASC, a.account_type ASC;
