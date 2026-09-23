-- name: InsertAccountReport :exec
INSERT INTO account_reports (
    id, portfolio_id, job_id, account_id, status, observed_at, error_class, error_detail,
    equity, adjusted_equity, available_equity, initial_margin_ratio, maintenance_margin_ratio, unified_margin_ratio,
    value_usd, method, source_report_id, omitted_json, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(job_id), sqlc.arg(account_id), sqlc.arg(status),
    sqlc.narg(observed_at), sqlc.narg(error_class), sqlc.narg(error_detail),
    sqlc.narg(equity), sqlc.narg(adjusted_equity), sqlc.narg(available_equity), sqlc.narg(initial_margin_ratio),
    sqlc.narg(maintenance_margin_ratio), sqlc.narg(unified_margin_ratio),
    NULL, 'missing', NULL, NULL, sqlc.arg(created_at)
);

-- name: UpdateAccountReportValuation :exec
UPDATE account_reports
SET value_usd = sqlc.narg(value_usd),
    method = sqlc.arg(method),
    source_report_id = sqlc.narg(source_report_id),
    omitted_json = sqlc.narg(omitted_json)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: InsertBalance :exec
INSERT INTO balances (
    report_id, portfolio_id, listing_id, equity, available_to_withdraw, available_to_trade, liability, interest, unrealized_pnl
) VALUES (
    sqlc.arg(report_id), sqlc.arg(portfolio_id), sqlc.arg(listing_id), sqlc.arg(equity), sqlc.narg(available_to_withdraw),
    sqlc.narg(available_to_trade), sqlc.narg(liability), sqlc.narg(interest), sqlc.narg(unrealized_pnl)
);

-- name: InsertPosition :exec
INSERT INTO positions (
    report_id, portfolio_id, platform_position_ref, instrument_id, side, quantity, quantity_unit,
    entry_price, mark_price, liquidation_price, unrealized_pnl, leverage, margin
) VALUES (
    sqlc.arg(report_id), sqlc.arg(portfolio_id), sqlc.arg(platform_position_ref), sqlc.arg(instrument_id), sqlc.arg(side),
    sqlc.arg(quantity), sqlc.arg(quantity_unit), sqlc.narg(entry_price), sqlc.narg(mark_price), sqlc.narg(liquidation_price),
    sqlc.narg(unrealized_pnl), sqlc.narg(leverage), sqlc.narg(margin)
);

-- name: ListReportsByJob :many
SELECT r.id, r.portfolio_id, r.job_id, r.account_id, r.status, r.observed_at, r.error_class, r.error_detail,
       r.equity, r.adjusted_equity, r.available_equity, r.initial_margin_ratio, r.maintenance_margin_ratio, r.unified_margin_ratio,
       r.value_usd, r.method, r.source_report_id, r.omitted_json, r.created_at,
       a.identity_id, a.account_type, a.display_name AS account_name, a.selected, i.platform, i.display_name AS identity_name,
       i.status AS identity_status, i.connection_status,
       src.observed_at AS source_observed_at, src.equity AS source_equity, src.adjusted_equity AS source_adjusted_equity,
       src.available_equity AS source_available_equity, src.initial_margin_ratio AS source_initial_margin_ratio,
       src.maintenance_margin_ratio AS source_maintenance_margin_ratio, src.unified_margin_ratio AS source_unified_margin_ratio
FROM account_reports r
INNER JOIN accounts a ON a.portfolio_id = r.portfolio_id AND a.id = r.account_id
INNER JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
LEFT JOIN account_reports src ON src.portfolio_id = r.portfolio_id AND src.id = r.source_report_id
WHERE r.portfolio_id = sqlc.arg(portfolio_id) AND r.job_id = sqlc.arg(job_id)
ORDER BY i.platform ASC, i.created_at ASC, a.account_type ASC;

-- name: GetReport :one
SELECT r.id, r.portfolio_id, r.job_id, r.account_id, r.status, r.observed_at, r.error_class, r.error_detail,
       r.equity, r.adjusted_equity, r.available_equity, r.initial_margin_ratio, r.maintenance_margin_ratio, r.unified_margin_ratio,
       r.value_usd, r.method, r.source_report_id, r.omitted_json, r.created_at,
       a.identity_id, a.account_type, a.display_name AS account_name, a.selected, i.platform, i.display_name AS identity_name,
       i.status AS identity_status, i.connection_status,
       src.observed_at AS source_observed_at, src.equity AS source_equity, src.adjusted_equity AS source_adjusted_equity,
       src.available_equity AS source_available_equity, src.initial_margin_ratio AS source_initial_margin_ratio,
       src.maintenance_margin_ratio AS source_maintenance_margin_ratio, src.unified_margin_ratio AS source_unified_margin_ratio
FROM account_reports r
INNER JOIN accounts a ON a.portfolio_id = r.portfolio_id AND a.id = r.account_id
INNER JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
LEFT JOIN account_reports src ON src.portfolio_id = r.portfolio_id AND src.id = r.source_report_id
WHERE r.portfolio_id = sqlc.arg(portfolio_id) AND r.id = sqlc.arg(id);

-- name: GetReportForAccountInJob :one
SELECT id FROM account_reports
WHERE portfolio_id = sqlc.arg(portfolio_id) AND job_id = sqlc.arg(job_id) AND account_id = sqlc.arg(account_id);

-- name: GetLatestOkReportForAccount :one
SELECT id, portfolio_id, job_id, account_id, status, observed_at, equity, adjusted_equity, available_equity,
       initial_margin_ratio, maintenance_margin_ratio, unified_margin_ratio, created_at
FROM account_reports
WHERE portfolio_id = sqlc.arg(portfolio_id) AND account_id = sqlc.arg(account_id) AND status = 'ok'
  AND created_at < sqlc.arg(before)
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ListBalancesByReport :many
SELECT b.report_id, b.listing_id, b.equity, b.available_to_withdraw, b.available_to_trade, b.liability, b.interest, b.unrealized_pnl,
       l.platform, l.symbol, l.asset_symbol
FROM balances b
INNER JOIN asset_listings l ON l.id = b.listing_id
WHERE b.report_id = sqlc.arg(report_id)
ORDER BY l.asset_symbol ASC, l.symbol ASC;

-- name: ListPositionsByReport :many
SELECT p.report_id, p.platform_position_ref, p.instrument_id, p.side, p.quantity, p.quantity_unit,
       p.entry_price, p.mark_price, p.liquidation_price, p.unrealized_pnl, p.leverage, p.margin,
       n.platform, n.symbol, n.kind, n.base_symbol, n.quote_symbol, n.settlement_symbol, n.expiry_at, n.contract_size
FROM positions p
INNER JOIN instruments n ON n.id = p.instrument_id
WHERE p.report_id = sqlc.arg(report_id)
ORDER BY n.symbol ASC, p.side ASC, p.platform_position_ref ASC;

-- name: ListBalancesByJob :many
SELECT cur.account_id, cur.source_report_id AS report_id, cur.observed_at AS current_observed_at, src.observed_at,
       b.listing_id, b.equity, b.available_to_withdraw, b.available_to_trade, b.liability, b.interest, b.unrealized_pnl,
       l.platform, l.symbol, l.asset_symbol,
       a.identity_id, a.account_type, a.display_name AS account_name, i.display_name AS identity_name
FROM account_reports cur
INNER JOIN account_reports src ON src.portfolio_id = cur.portfolio_id AND src.id = cur.source_report_id
INNER JOIN balances b ON b.report_id = src.id
INNER JOIN asset_listings l ON l.id = b.listing_id
INNER JOIN accounts a ON a.portfolio_id = cur.portfolio_id AND a.id = cur.account_id
INNER JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE cur.portfolio_id = sqlc.arg(portfolio_id) AND cur.job_id = sqlc.arg(job_id)
ORDER BY l.asset_symbol ASC, i.platform ASC, i.created_at ASC, a.account_type ASC;

-- name: ListPositionsByJob :many
SELECT cur.account_id, cur.source_report_id AS report_id, src.observed_at,
       p.platform_position_ref, p.instrument_id, p.side, p.quantity, p.quantity_unit,
       p.entry_price, p.mark_price, p.liquidation_price, p.unrealized_pnl, p.leverage, p.margin,
       n.platform, n.symbol, n.kind, n.base_symbol, n.quote_symbol, n.settlement_symbol, n.expiry_at, n.contract_size,
       a.identity_id, a.account_type, a.display_name AS account_name, i.display_name AS identity_name
FROM account_reports cur
INNER JOIN account_reports src ON src.portfolio_id = cur.portfolio_id AND src.id = cur.source_report_id
INNER JOIN positions p ON p.report_id = src.id
INNER JOIN instruments n ON n.id = p.instrument_id
INNER JOIN accounts a ON a.portfolio_id = cur.portfolio_id AND a.id = cur.account_id
INNER JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE cur.portfolio_id = sqlc.arg(portfolio_id) AND cur.job_id = sqlc.arg(job_id)
ORDER BY n.symbol ASC, i.platform ASC, i.created_at ASC, a.account_type ASC, p.side ASC;
