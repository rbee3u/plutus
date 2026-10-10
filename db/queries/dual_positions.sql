-- name: InsertDualPosition :exec
INSERT INTO dual_positions (report_id, portfolio_id, position_key, product_ref, order_ref, base_asset_symbol, quote_asset_symbol, principal_asset_symbol, principal, direction, target_price, expiry_at, annualized_rate, rate_kind, status, subscribed_at)
VALUES (sqlc.arg(report_id), sqlc.arg(portfolio_id), sqlc.arg(position_key), sqlc.narg(product_ref), sqlc.narg(order_ref), sqlc.narg(base_asset_symbol), sqlc.narg(quote_asset_symbol), sqlc.arg(principal_asset_symbol), sqlc.arg(principal), sqlc.narg(direction), sqlc.narg(target_price), sqlc.narg(expiry_at), sqlc.narg(annualized_rate), sqlc.narg(rate_kind), sqlc.arg(status), sqlc.narg(subscribed_at));

-- name: ListDualPositionsByReport :many
SELECT * FROM dual_positions WHERE portfolio_id = sqlc.arg(portfolio_id) AND report_id = sqlc.arg(report_id)
ORDER BY expiry_at IS NULL, expiry_at, position_key;

-- name: ListDualPositionsByCycle :many
SELECT sqlc.embed(p), cur.account_id, COALESCE(src.source_observed_at, src.collected_at) AS observed_at,
       a.identity_id, a.account_type, a.account_type AS account_name,
       i.platform, i.display_name AS identity_name
FROM account_reports cur
JOIN account_valuations v ON v.portfolio_id = cur.portfolio_id AND v.cycle_id = cur.cycle_id AND v.account_id = cur.account_id
JOIN account_reports src ON src.portfolio_id = cur.portfolio_id AND src.id = v.source_report_id
JOIN dual_positions p ON p.portfolio_id = src.portfolio_id AND p.report_id = src.id
JOIN accounts a ON a.portfolio_id = cur.portfolio_id AND a.id = cur.account_id
JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE cur.portfolio_id = sqlc.arg(portfolio_id) AND cur.cycle_id = sqlc.arg(cycle_id)
ORDER BY cur.account_id, p.position_key;
