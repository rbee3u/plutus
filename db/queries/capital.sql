-- name: InsertCapitalFlow :exec
INSERT INTO capital_flows (
    id, portfolio_id, direction, canonical_asset_symbol, quantity, value_usd, pricing_method, pricing_platform, price_observation_id, pricing_attempted_at, calculation_version, effective_at, account_id, note,
    version, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(direction), sqlc.arg(canonical_asset_symbol), sqlc.arg(quantity),
    sqlc.narg(value_usd), sqlc.arg(pricing_method), sqlc.narg(pricing_platform), sqlc.narg(price_observation_id), sqlc.arg(pricing_attempted_at), sqlc.arg(calculation_version), sqlc.arg(effective_at), sqlc.narg(account_id), sqlc.narg(note),
    sqlc.arg(version), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: GetCapitalFlow :one
SELECT * FROM capital_flows
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: ListCapitalFlows :many
SELECT * FROM capital_flows
WHERE portfolio_id = sqlc.arg(portfolio_id)
  AND effective_at >= sqlc.arg(from_at) AND effective_at < sqlc.arg(to_at)
ORDER BY effective_at DESC, id DESC;

-- name: UpdateCapitalFlow :execrows
UPDATE capital_flows
SET direction = sqlc.arg(direction),
    canonical_asset_symbol = sqlc.arg(canonical_asset_symbol),
    quantity = sqlc.arg(quantity),
    value_usd = sqlc.narg(value_usd),
    pricing_method = sqlc.arg(pricing_method),
    pricing_platform = sqlc.narg(pricing_platform),
    price_observation_id = sqlc.narg(price_observation_id),
    pricing_attempted_at = sqlc.arg(pricing_attempted_at),
    calculation_version = sqlc.arg(calculation_version),
    effective_at = sqlc.arg(effective_at),
    account_id = sqlc.narg(account_id),
    note = sqlc.narg(note),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version);

-- name: DeleteCapitalFlow :execrows
DELETE FROM capital_flows
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version);

-- name: ListCapitalFlowPage :many
SELECT * FROM capital_flows
WHERE portfolio_id = sqlc.arg(portfolio_id)
  AND effective_at >= sqlc.arg(from_at) AND effective_at < sqlc.arg(to_at)
ORDER BY effective_at DESC, id DESC LIMIT ? OFFSET ?;

-- name: CountCapitalFlows :one
SELECT COUNT(*) FROM capital_flows
WHERE portfolio_id = sqlc.arg(portfolio_id)
  AND effective_at >= sqlc.arg(from_at) AND effective_at < sqlc.arg(to_at);
