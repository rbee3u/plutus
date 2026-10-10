-- name: InsertDualPosition :exec
INSERT INTO dual_positions (report_id, portfolio_id, platform_position_ref, product_ref, order_ref, base_asset_symbol, quote_asset_symbol, invest_asset_symbol, principal, direction, target_price, expiry_at, annualized_rate, rate_kind, status, subscribed_at)
VALUES (sqlc.arg(report_id), sqlc.arg(portfolio_id), sqlc.arg(platform_position_ref), sqlc.narg(product_ref), sqlc.narg(order_ref), sqlc.narg(base_asset_symbol), sqlc.narg(quote_asset_symbol), sqlc.arg(invest_asset_symbol), sqlc.arg(principal), sqlc.narg(direction), sqlc.narg(target_price), sqlc.narg(expiry_at), sqlc.narg(annualized_rate), sqlc.narg(rate_kind), sqlc.arg(status), sqlc.narg(subscribed_at));

-- name: ListDualPositionsByReport :many
SELECT * FROM dual_positions WHERE report_id = sqlc.arg(report_id)
ORDER BY expiry_at IS NULL, expiry_at, platform_position_ref;

-- name: ListDualPositionsByJob :many
SELECT sqlc.embed(p), cur.account_id, src.observed_at,
       a.identity_id, a.account_type, a.display_name AS account_name,
       i.platform, i.display_name AS identity_name
FROM account_reports cur
JOIN account_reports src ON src.portfolio_id = cur.portfolio_id AND src.id = cur.source_report_id
JOIN dual_positions p ON p.report_id = src.id
JOIN accounts a ON a.portfolio_id = cur.portfolio_id AND a.id = cur.account_id
JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE cur.portfolio_id = sqlc.arg(portfolio_id) AND cur.job_id = sqlc.arg(job_id)
ORDER BY cur.account_id, p.platform_position_ref;
