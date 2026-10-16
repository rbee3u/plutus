-- name: InsertAccountReport :exec
INSERT INTO account_reports (
    id, portfolio_id, cycle_id, account_id, status, collected_at, source_observed_at, recorded_at, margin_mode, error_class, error_detail, reported_equity_usd, adjusted_equity_usd, available_equity_usd, initial_margin_requirement_usd, maintenance_margin_requirement_usd, reported_initial_margin_ratio, reported_maintenance_margin_ratio, reported_margin_ratio, spot_margin_unrealized_pnl_usd, manual_revision_number
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(cycle_id), sqlc.arg(account_id), sqlc.arg(status), sqlc.narg(collected_at), sqlc.narg(source_observed_at), sqlc.arg(recorded_at), sqlc.narg(margin_mode), sqlc.narg(error_class), sqlc.narg(error_detail), sqlc.narg(reported_equity_usd), sqlc.narg(adjusted_equity_usd), sqlc.narg(available_equity_usd), sqlc.narg(initial_margin_requirement_usd), sqlc.narg(maintenance_margin_requirement_usd), sqlc.narg(reported_initial_margin_ratio), sqlc.narg(reported_maintenance_margin_ratio), sqlc.narg(reported_margin_ratio), sqlc.narg(spot_margin_unrealized_pnl_usd), sqlc.narg(manual_revision_number)
);

-- name: InsertBalance :exec
INSERT INTO balances (
    report_id, portfolio_id, asset_id, equity, available_to_withdraw, available_to_trade, liability, interest, unrealized_pnl
) VALUES (
    sqlc.arg(report_id), sqlc.arg(portfolio_id), sqlc.arg(asset_id), sqlc.arg(equity), sqlc.narg(available_to_withdraw),
    sqlc.narg(available_to_trade), sqlc.narg(liability), sqlc.narg(interest), sqlc.narg(unrealized_pnl)
);

-- name: InsertPosition :exec
INSERT INTO futures_positions (
    report_id, portfolio_id, position_key, futures_contract_id, side, source_quantity, source_quantity_unit,
    entry_price, mark_price, liquidation_price, unrealized_pnl, leverage, margin,
    position_mode, margin_setting, margin_asset_symbol, margin_basis
) VALUES (
    sqlc.arg(report_id), sqlc.arg(portfolio_id), sqlc.arg(position_key), sqlc.arg(futures_contract_id), sqlc.arg(side),
    sqlc.arg(source_quantity), sqlc.arg(source_quantity_unit), sqlc.narg(entry_price), sqlc.narg(mark_price), sqlc.narg(liquidation_price),
    sqlc.narg(unrealized_pnl), sqlc.narg(leverage), sqlc.narg(margin),
    sqlc.arg(position_mode), sqlc.narg(margin_setting), sqlc.narg(margin_asset_symbol), sqlc.narg(margin_basis)
);

-- name: ListReportsByCycle :many
SELECT r.id, r.portfolio_id, r.cycle_id, r.account_id, r.margin_mode, r.status, r.collected_at, r.source_observed_at, r.error_class, r.error_detail,
       r.reported_equity_usd, r.adjusted_equity_usd, r.available_equity_usd, r.initial_margin_requirement_usd, r.maintenance_margin_requirement_usd, r.spot_margin_unrealized_pnl_usd, r.reported_initial_margin_ratio, r.reported_maintenance_margin_ratio, r.reported_margin_ratio,
       v.value_usd, v.valuation_method, v.source_report_id, r.recorded_at, r.manual_revision_number, mr.changed_at AS manual_changed_at,
       a.identity_id, a.account_type, a.selected, i.platform, i.display_name AS identity_name,
       i.archived_at AS identity_archived_at, i.connection_status, i.account_structure,
       src.margin_mode AS source_margin_mode, COALESCE(src.source_observed_at, src.collected_at) AS effective_source_observed_at, src.reported_equity_usd AS source_equity, src.adjusted_equity_usd AS source_adjusted_equity,
       src.available_equity_usd AS source_available_equity, src.initial_margin_requirement_usd AS source_initial_margin_requirement_usd, src.maintenance_margin_requirement_usd AS source_maintenance_margin_requirement_usd, src.spot_margin_unrealized_pnl_usd AS source_spot_margin_unrealized_pnl_usd, src.reported_initial_margin_ratio AS source_initial_margin_ratio,
       src.reported_maintenance_margin_ratio AS source_maintenance_margin_ratio, src.reported_margin_ratio AS source_margin_ratio
FROM account_reports r
INNER JOIN accounts a ON a.portfolio_id = r.portfolio_id AND a.id = r.account_id
INNER JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
LEFT JOIN manual_revisions mr ON mr.portfolio_id = r.portfolio_id AND mr.account_id = r.account_id AND mr.revision_number = r.manual_revision_number
LEFT JOIN account_valuations v ON v.portfolio_id = r.portfolio_id AND v.cycle_id = r.cycle_id AND v.account_id = r.account_id
LEFT JOIN account_reports src ON src.portfolio_id = r.portfolio_id AND src.id = v.source_report_id
WHERE r.portfolio_id = sqlc.arg(portfolio_id) AND r.cycle_id = sqlc.arg(cycle_id)
ORDER BY r.id;

-- name: GetReport :one
SELECT r.id, r.portfolio_id, r.cycle_id, r.account_id, r.margin_mode, r.status, r.collected_at, r.source_observed_at, r.error_class, r.error_detail,
       r.reported_equity_usd, r.adjusted_equity_usd, r.available_equity_usd, r.initial_margin_requirement_usd, r.maintenance_margin_requirement_usd, r.spot_margin_unrealized_pnl_usd, r.reported_initial_margin_ratio, r.reported_maintenance_margin_ratio, r.reported_margin_ratio,
       v.value_usd, v.valuation_method, v.source_report_id, r.recorded_at, r.manual_revision_number, mr.changed_at AS manual_changed_at,
       a.identity_id, a.account_type, a.selected, i.platform, i.display_name AS identity_name,
       i.archived_at AS identity_archived_at, i.connection_status, i.account_structure,
       src.margin_mode AS source_margin_mode, COALESCE(src.source_observed_at, src.collected_at) AS effective_source_observed_at, src.reported_equity_usd AS source_equity, src.adjusted_equity_usd AS source_adjusted_equity,
       src.available_equity_usd AS source_available_equity, src.initial_margin_requirement_usd AS source_initial_margin_requirement_usd, src.maintenance_margin_requirement_usd AS source_maintenance_margin_requirement_usd, src.spot_margin_unrealized_pnl_usd AS source_spot_margin_unrealized_pnl_usd, src.reported_initial_margin_ratio AS source_initial_margin_ratio,
       src.reported_maintenance_margin_ratio AS source_maintenance_margin_ratio, src.reported_margin_ratio AS source_margin_ratio
FROM account_reports r
INNER JOIN accounts a ON a.portfolio_id = r.portfolio_id AND a.id = r.account_id
INNER JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
LEFT JOIN manual_revisions mr ON mr.portfolio_id = r.portfolio_id AND mr.account_id = r.account_id AND mr.revision_number = r.manual_revision_number
LEFT JOIN account_valuations v ON v.portfolio_id = r.portfolio_id AND v.cycle_id = r.cycle_id AND v.account_id = r.account_id
LEFT JOIN account_reports src ON src.portfolio_id = r.portfolio_id AND src.id = v.source_report_id
WHERE r.portfolio_id = sqlc.arg(portfolio_id) AND r.id = sqlc.arg(id);

-- name: GetLatestSucceededReportForAccount :one
SELECT * FROM account_reports
WHERE portfolio_id = sqlc.arg(portfolio_id) AND account_id = sqlc.arg(account_id) AND status = 'succeeded'
  AND recorded_at < sqlc.arg(before)
ORDER BY recorded_at DESC, id DESC LIMIT 1;

-- name: ListBalancesByReport :many
SELECT b.report_id, b.asset_id, b.equity, b.available_to_withdraw, b.available_to_trade, b.liability, b.interest, b.unrealized_pnl,
       asset.platform, asset.symbol AS asset_symbol, asset.canonical_asset_symbol AS canonical_asset_symbol
FROM balances b
INNER JOIN assets asset ON asset.portfolio_id = b.portfolio_id AND asset.id = b.asset_id
WHERE b.portfolio_id = sqlc.arg(portfolio_id) AND b.report_id = sqlc.arg(report_id)
ORDER BY asset.symbol ASC, asset.canonical_asset_symbol ASC;

-- Seed history initialization runs inside the transaction that inserted every
-- row, before statistics describe them; the join order stays fixed.
-- name: ListBalancesByReports :many
SELECT b.report_id, b.asset_id, b.equity, b.available_to_withdraw, b.available_to_trade, b.liability, b.interest, b.unrealized_pnl,
       asset.platform, asset.symbol AS asset_symbol, asset.canonical_asset_symbol AS canonical_asset_symbol
FROM balances b
STRAIGHT_JOIN assets asset ON asset.portfolio_id = b.portfolio_id AND asset.id = b.asset_id
WHERE b.portfolio_id = sqlc.arg(portfolio_id) AND b.report_id IN (sqlc.slice('report_ids'))
ORDER BY b.report_id, asset.symbol ASC, asset.canonical_asset_symbol ASC;

-- name: ListPositionsByReport :many
SELECT p.report_id, p.position_key, p.futures_contract_id, p.side, p.source_quantity, p.source_quantity_unit,
       p.entry_price, p.mark_price, p.liquidation_price, p.unrealized_pnl, p.leverage, p.margin, p.position_mode, p.margin_setting, p.margin_asset_symbol, p.margin_basis,
       n.platform, n.symbol AS futures_symbol, n.kind, n.base_asset_symbol, n.quote_asset_symbol, n.settlement_asset_symbol, n.expiry_at, n.contract_size
FROM futures_positions p
INNER JOIN futures_contracts n ON n.portfolio_id = p.portfolio_id AND n.id = p.futures_contract_id
WHERE p.portfolio_id = sqlc.arg(portfolio_id) AND p.report_id = sqlc.arg(report_id)
ORDER BY p.position_key;

-- name: ListBalancesByCycle :many
SELECT cur.account_id, v.source_report_id AS report_id, COALESCE(cur.source_observed_at, cur.collected_at) AS current_observed_at, COALESCE(src.source_observed_at, src.collected_at) AS observed_at,
       b.asset_id, b.equity,
       asset.platform, asset.symbol AS asset_symbol, asset.canonical_asset_symbol AS canonical_asset_symbol,
       a.identity_id, a.account_type, i.display_name AS identity_name
FROM account_reports cur
JOIN account_valuations v ON v.portfolio_id = cur.portfolio_id AND v.cycle_id = cur.cycle_id AND v.account_id = cur.account_id
INNER JOIN account_reports src ON src.portfolio_id = cur.portfolio_id AND src.id = v.source_report_id
INNER JOIN balances b ON b.portfolio_id = src.portfolio_id AND b.report_id = src.id
JOIN balance_valuations bv ON bv.portfolio_id = v.portfolio_id AND bv.cycle_id = v.cycle_id AND bv.account_id = v.account_id AND bv.asset_id = b.asset_id
INNER JOIN assets asset ON asset.portfolio_id = b.portfolio_id AND asset.id = b.asset_id
INNER JOIN accounts a ON a.portfolio_id = cur.portfolio_id AND a.id = cur.account_id
INNER JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE cur.portfolio_id = sqlc.arg(portfolio_id) AND cur.cycle_id = sqlc.arg(cycle_id)
AND (sqlc.arg(platform) = '' OR i.platform = sqlc.arg(platform))
AND (sqlc.narg(identity_id) IS NULL OR i.id = sqlc.narg(identity_id))
AND (a.account_type = 'manual' OR sqlc.arg(symbol) = '' OR LOCATE(sqlc.arg(symbol), LOWER(asset.symbol)) > 0 OR LOCATE(sqlc.arg(symbol), LOWER(bv.canonical_asset_symbol)) > 0)
ORDER BY cur.account_id, b.asset_id;

-- name: ListPositionsByCycle :many
SELECT cur.account_id, v.source_report_id AS report_id, COALESCE(src.source_observed_at, src.collected_at) AS observed_at,
       p.position_key, p.futures_contract_id, p.side, p.source_quantity, p.source_quantity_unit,
       p.entry_price, p.mark_price, p.liquidation_price, p.unrealized_pnl, p.leverage, p.margin, p.position_mode, p.margin_setting, p.margin_asset_symbol, p.margin_basis,
       n.platform, n.symbol AS futures_symbol, n.kind, n.base_asset_symbol, n.quote_asset_symbol, n.settlement_asset_symbol, n.expiry_at, n.contract_size,
       a.identity_id, a.account_type, i.display_name AS identity_name
FROM account_reports cur
JOIN account_valuations v ON v.portfolio_id = cur.portfolio_id AND v.cycle_id = cur.cycle_id AND v.account_id = cur.account_id
INNER JOIN account_reports src ON src.portfolio_id = cur.portfolio_id AND src.id = v.source_report_id
INNER JOIN futures_positions p ON p.portfolio_id = src.portfolio_id AND p.report_id = src.id
INNER JOIN futures_contracts n ON n.portfolio_id = p.portfolio_id AND n.id = p.futures_contract_id
INNER JOIN accounts a ON a.portfolio_id = cur.portfolio_id AND a.id = cur.account_id
INNER JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE cur.portfolio_id = sqlc.arg(portfolio_id) AND cur.cycle_id = sqlc.arg(cycle_id)
ORDER BY cur.account_id, p.position_key;

-- name: ListReportsByCycles :many
SELECT r.id, r.portfolio_id, r.cycle_id, r.account_id, r.margin_mode, r.status, r.collected_at, r.source_observed_at, r.error_class, r.error_detail,
       r.reported_equity_usd, r.adjusted_equity_usd, r.available_equity_usd, r.initial_margin_requirement_usd, r.maintenance_margin_requirement_usd, r.spot_margin_unrealized_pnl_usd, r.reported_initial_margin_ratio, r.reported_maintenance_margin_ratio, r.reported_margin_ratio,
       v.value_usd, v.valuation_method, v.source_report_id, r.recorded_at, r.manual_revision_number, mr.changed_at AS manual_changed_at,
       a.identity_id, a.account_type, a.selected, i.platform, i.display_name AS identity_name,
       i.archived_at AS identity_archived_at, i.connection_status, i.account_structure,
       src.margin_mode AS source_margin_mode, COALESCE(src.source_observed_at, src.collected_at) AS effective_source_observed_at, src.reported_equity_usd AS source_equity, src.adjusted_equity_usd AS source_adjusted_equity,
       src.available_equity_usd AS source_available_equity, src.initial_margin_requirement_usd AS source_initial_margin_requirement_usd, src.maintenance_margin_requirement_usd AS source_maintenance_margin_requirement_usd, src.spot_margin_unrealized_pnl_usd AS source_spot_margin_unrealized_pnl_usd, src.reported_initial_margin_ratio AS source_initial_margin_ratio,
       src.reported_maintenance_margin_ratio AS source_maintenance_margin_ratio, src.reported_margin_ratio AS source_margin_ratio
FROM account_reports r
INNER JOIN accounts a ON a.portfolio_id = r.portfolio_id AND a.id = r.account_id
INNER JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
LEFT JOIN manual_revisions mr ON mr.portfolio_id = r.portfolio_id AND mr.account_id = r.account_id AND mr.revision_number = r.manual_revision_number
LEFT JOIN account_valuations v ON v.portfolio_id = r.portfolio_id AND v.cycle_id = r.cycle_id AND v.account_id = r.account_id
LEFT JOIN account_reports src ON src.portfolio_id = r.portfolio_id AND src.id = v.source_report_id
WHERE r.portfolio_id = sqlc.arg(portfolio_id) AND r.cycle_id IN (sqlc.slice('cycle_ids'))
ORDER BY r.id;

-- name: GetAccountReportAtCycle :one
SELECT r.id, r.portfolio_id, r.cycle_id, r.account_id, r.margin_mode, r.status, r.collected_at, r.source_observed_at, r.error_class, r.error_detail,
       r.reported_equity_usd, r.adjusted_equity_usd, r.available_equity_usd, r.initial_margin_requirement_usd, r.maintenance_margin_requirement_usd, r.spot_margin_unrealized_pnl_usd, r.reported_initial_margin_ratio, r.reported_maintenance_margin_ratio, r.reported_margin_ratio,
       v.value_usd, v.valuation_method, v.source_report_id, r.recorded_at, r.manual_revision_number, mr.changed_at AS manual_changed_at,
       a.identity_id, a.account_type, a.selected, i.platform, i.display_name AS identity_name,
       i.archived_at AS identity_archived_at, i.connection_status, i.account_structure,
       src.margin_mode AS source_margin_mode, COALESCE(src.source_observed_at, src.collected_at) AS effective_source_observed_at, src.reported_equity_usd AS source_equity, src.adjusted_equity_usd AS source_adjusted_equity,
       src.available_equity_usd AS source_available_equity, src.initial_margin_requirement_usd AS source_initial_margin_requirement_usd, src.maintenance_margin_requirement_usd AS source_maintenance_margin_requirement_usd, src.spot_margin_unrealized_pnl_usd AS source_spot_margin_unrealized_pnl_usd, src.reported_initial_margin_ratio AS source_initial_margin_ratio,
       src.reported_maintenance_margin_ratio AS source_maintenance_margin_ratio, src.reported_margin_ratio AS source_margin_ratio
FROM account_reports r
INNER JOIN accounts a ON a.portfolio_id = r.portfolio_id AND a.id = r.account_id
INNER JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
LEFT JOIN manual_revisions mr ON mr.portfolio_id = r.portfolio_id AND mr.account_id = r.account_id AND mr.revision_number = r.manual_revision_number
LEFT JOIN account_valuations v ON v.portfolio_id = r.portfolio_id AND v.cycle_id = r.cycle_id AND v.account_id = r.account_id
LEFT JOIN account_reports src ON src.portfolio_id = r.portfolio_id AND src.id = v.source_report_id
WHERE r.portfolio_id = sqlc.arg(portfolio_id) AND r.cycle_id = sqlc.arg(cycle_id)
AND r.account_id = sqlc.arg(account_id);

-- Only the displayed page needs availability, liability, interest, and PnL.
-- name: ListBalanceDetailsByReportsAndAssets :many
SELECT * FROM balances
WHERE portfolio_id = sqlc.arg(portfolio_id)
  AND report_id IN (sqlc.slice('report_ids')) AND asset_id IN (sqlc.slice('asset_ids'));
