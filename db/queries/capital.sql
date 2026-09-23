-- name: InsertCapitalFlow :exec
INSERT INTO capital_flows (
    id, portfolio_id, direction, asset_symbol, quantity, value_usd, effective_at, account_id, note,
    created_by_user_id, version, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(direction), sqlc.arg(asset_symbol), sqlc.arg(quantity),
    sqlc.narg(value_usd), sqlc.arg(effective_at), sqlc.narg(account_id), sqlc.narg(note),
    sqlc.arg(created_by_user_id), sqlc.arg(version), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: GetCapitalFlow :one
SELECT id, portfolio_id, direction, asset_symbol, quantity, value_usd, effective_at, account_id, note,
       created_by_user_id, version, created_at, updated_at
FROM capital_flows
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: ListCapitalFlows :many
SELECT id, portfolio_id, direction, asset_symbol, quantity, value_usd, effective_at, account_id, note,
       created_by_user_id, version, created_at, updated_at
FROM capital_flows
WHERE portfolio_id = sqlc.arg(portfolio_id)
  AND effective_at >= sqlc.arg(from_at) AND effective_at < sqlc.arg(to_at)
ORDER BY effective_at DESC, id DESC;

-- name: UpdateCapitalFlow :execrows
UPDATE capital_flows
SET direction = sqlc.arg(direction),
    asset_symbol = sqlc.arg(asset_symbol),
    quantity = sqlc.arg(quantity),
    value_usd = sqlc.narg(value_usd),
    effective_at = sqlc.arg(effective_at),
    account_id = sqlc.narg(account_id),
    note = sqlc.narg(note),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version);

-- name: DeleteCapitalFlow :execrows
DELETE FROM capital_flows
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version);
