-- name: ListRebuildReports :many
SELECT r.id, r.portfolio_id, r.job_id, r.account_id, r.scope_version, r.margin_mode, r.status, r.observed_at, r.error_class, r.error_detail,
       r.equity, r.adjusted_equity, r.available_equity, r.initial_margin_ratio, r.maintenance_margin_ratio, r.unified_margin_ratio,
       r.value_usd, r.method, r.source_report_id, r.omitted_json, r.created_at, r.manual_state_json, r.manual_modified_at,
       a.identity_id, a.account_type, a.display_name AS account_name, a.selected, i.platform, i.display_name AS identity_name,
       i.status AS identity_status, i.connection_status, i.account_structure,
       src.margin_mode AS source_margin_mode, src.observed_at AS source_observed_at, src.equity AS source_equity, src.adjusted_equity AS source_adjusted_equity,
       src.available_equity AS source_available_equity, src.initial_margin_ratio AS source_initial_margin_ratio,
       src.maintenance_margin_ratio AS source_maintenance_margin_ratio, src.unified_margin_ratio AS source_unified_margin_ratio
FROM account_reports r
INNER JOIN accounts a ON a.portfolio_id = r.portfolio_id AND a.id = r.account_id
INNER JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
LEFT JOIN account_reports src ON src.portfolio_id = r.portfolio_id AND src.id = r.source_report_id
WHERE r.portfolio_id = sqlc.arg(portfolio_id) AND r.job_id IN (sqlc.slice('job_ids'))
ORDER BY r.id;

-- name: ListRebuildSources :many
SELECT failed.id AS failed_report_id, src.id, src.portfolio_id, src.job_id, src.account_id,
       src.scope_version, src.margin_mode, src.status, src.observed_at, src.equity,
       src.adjusted_equity, src.available_equity, src.initial_margin_ratio,
       src.maintenance_margin_ratio, src.unified_margin_ratio, src.created_at
FROM account_reports failed
INNER JOIN account_reports src ON src.id = (
    SELECT candidate.id FROM account_reports candidate
    WHERE candidate.portfolio_id = failed.portfolio_id
      AND candidate.account_id = failed.account_id
      AND candidate.scope_version = failed.scope_version AND candidate.status = 'ok'
      AND candidate.created_at < failed.created_at
    ORDER BY candidate.created_at DESC, candidate.id DESC
    LIMIT 1
)
WHERE failed.portfolio_id = sqlc.arg(portfolio_id)
  AND failed.id IN (sqlc.slice('failed_ids'));

-- name: ListRebuildBalances :many
SELECT b.report_id, b.listing_id, b.equity, b.available_to_withdraw, b.available_to_trade, b.liability, b.interest, b.unrealized_pnl,
       l.platform, l.symbol, l.asset_symbol
FROM balances b
INNER JOIN asset_listings l ON l.id = b.listing_id
WHERE b.portfolio_id = sqlc.arg(portfolio_id) AND b.report_id IN (sqlc.slice('report_ids'))
ORDER BY b.report_id, l.asset_symbol ASC, l.symbol ASC;

-- name: ListRebuildPrices :many
SELECT platform, asset_symbol, observed_at, price, quote_symbol
FROM prices
WHERE asset_symbol IN (sqlc.slice('asset_symbols'))
  AND observed_at >= sqlc.arg(from_at) AND observed_at <= sqlc.arg(to_at)
ORDER BY platform, asset_symbol, observed_at;

-- name: ListRebuildPointTimes :many
SELECT job_id, evaluated_at FROM valuation_points
WHERE portfolio_id = sqlc.arg(portfolio_id)
ORDER BY evaluated_at, job_id;
