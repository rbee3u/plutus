-- name: InsertAccount :exec
INSERT INTO accounts (
    id, portfolio_id, identity_id, account_type, selected
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(identity_id), sqlc.arg(account_type),
    sqlc.arg(selected)
);

-- name: ListAccountsByPortfolio :many
SELECT a.id, a.portfolio_id, a.identity_id, a.account_type,  a.selected, i.platform, i.display_name AS identity_name, i.account_structure, i.archived_at AS identity_archived_at
FROM accounts a
INNER JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE a.portfolio_id = sqlc.arg(portfolio_id)
ORDER BY a.id;

-- name: ListAccountsByIdentity :many
SELECT portfolio_id, id, identity_id, account_type, selected
FROM accounts
WHERE portfolio_id = sqlc.arg(portfolio_id) AND identity_id = sqlc.arg(identity_id)
ORDER BY id;

-- name: GetAccount :one
SELECT a.id, a.portfolio_id, a.identity_id, a.account_type,  a.selected, i.platform, i.display_name AS identity_name, i.account_structure, i.archived_at AS identity_archived_at
FROM accounts a
INNER JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE a.portfolio_id = sqlc.arg(portfolio_id) AND a.id = sqlc.arg(id);

-- name: UpdateAccount :execrows
UPDATE accounts
SET selected = sqlc.arg(selected)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: ListBoundaryAccounts :many
SELECT a.id, a.portfolio_id, a.identity_id, a.account_type,  i.platform, i.display_name AS identity_name,
       i.connection_status, i.account_structure
FROM accounts a
INNER JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE a.portfolio_id = sqlc.arg(portfolio_id) AND a.selected = TRUE AND i.archived_at IS NULL
ORDER BY a.id;
