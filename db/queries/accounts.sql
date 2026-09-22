-- name: InsertAccount :exec
INSERT INTO accounts (
    id, portfolio_id, identity_id, external_ref, display_name, account_type, status,
    first_seen_at, last_seen_at, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(identity_id), sqlc.arg(external_ref),
    sqlc.arg(display_name), sqlc.arg(account_type), sqlc.arg(status), sqlc.arg(first_seen_at),
    sqlc.arg(last_seen_at), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: GetAccount :one
SELECT id, portfolio_id, identity_id, external_ref, display_name, account_type, status,
       first_seen_at, last_seen_at, created_at, updated_at
FROM accounts
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: ListAccountsByPortfolio :many
SELECT a.id, a.portfolio_id, a.identity_id, a.external_ref, a.display_name, a.account_type, a.status,
       a.first_seen_at, a.last_seen_at, a.created_at, a.updated_at
FROM accounts a
INNER JOIN identities i
    ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE a.portfolio_id = sqlc.arg(portfolio_id) AND i.status <> 'archived'
ORDER BY a.display_name ASC, a.id ASC
LIMIT ? OFFSET ?;

-- name: CountAccountsByPortfolio :one
SELECT COUNT(*)
FROM accounts a
INNER JOIN identities i
    ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE a.portfolio_id = sqlc.arg(portfolio_id) AND i.status <> 'archived';
