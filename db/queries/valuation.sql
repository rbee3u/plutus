-- name: InsertValuationPoint :exec
INSERT INTO valuation_points (
    portfolio_id, cycle_id, evaluated_at, total_usd, account_count, stale_account_count, incomplete_account_count, calculation_version
) VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetFirstValuationPoint :one
SELECT * FROM valuation_points
WHERE evaluated_at >= COALESCE((SELECT history_before FROM portfolios WHERE id = sqlc.arg(portfolio_id)), CAST('1000-01-01' AS DATETIME(6))) AND portfolio_id = sqlc.arg(portfolio_id)
ORDER BY evaluated_at ASC, cycle_id ASC
LIMIT 1;

-- name: GetLatestValuationPoint :one
SELECT * FROM valuation_points
WHERE portfolio_id = sqlc.arg(portfolio_id)
ORDER BY evaluated_at DESC, cycle_id DESC
LIMIT 1;

-- name: GetValuationPoint :one
SELECT * FROM valuation_points
WHERE portfolio_id = sqlc.arg(portfolio_id) AND cycle_id = sqlc.arg(cycle_id);

-- name: GetValuationPointAtOrBefore :one
SELECT * FROM valuation_points
WHERE sqlc.narg(at) >= COALESCE((SELECT history_before FROM portfolios WHERE id = sqlc.arg(portfolio_id)), CAST('1000-01-01' AS DATETIME(6))) AND portfolio_id = sqlc.arg(portfolio_id) AND evaluated_at <= sqlc.narg(at)
ORDER BY evaluated_at DESC, cycle_id DESC
LIMIT 1;

-- Prepared endpoint reads must seek the timeline even for distant history.
-- name: GetValuationPointBefore :one
SELECT * FROM valuation_points FORCE INDEX (idx_valuation_points_timeline)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND (evaluated_at < sqlc.arg(at) OR (evaluated_at = sqlc.arg(at) AND cycle_id < sqlc.arg(cycle_id)))
ORDER BY evaluated_at DESC, cycle_id DESC
LIMIT 1;

-- name: ListValuationPointsBetween :many
SELECT * FROM valuation_points
WHERE portfolio_id = sqlc.arg(portfolio_id) AND evaluated_at >= sqlc.arg(from_at) AND evaluated_at <= sqlc.arg(to_at)
ORDER BY evaluated_at ASC, cycle_id ASC;

-- name: GetValuationPointByCycle :one
SELECT * FROM valuation_points
WHERE portfolio_id = sqlc.arg(portfolio_id) AND cycle_id = sqlc.arg(cycle_id);

-- name: ListValuationPointSummariesBetween :many
SELECT portfolio_id, cycle_id, evaluated_at, total_usd, account_count, stale_account_count, incomplete_account_count,
       calculation_version, detail_state
FROM valuation_points WHERE evaluated_at >= COALESCE((SELECT history_before FROM portfolios WHERE id = sqlc.arg(portfolio_id)), CAST('1000-01-01' AS DATETIME(6))) AND portfolio_id = sqlc.arg(portfolio_id) AND evaluated_at >= sqlc.arg(from_at) AND evaluated_at <= sqlc.arg(to_at)
ORDER BY evaluated_at ASC, cycle_id ASC LIMIT ?;

-- name: ListValuationPointHeadersBetween :many
SELECT cycle_id, evaluated_at FROM valuation_points WHERE evaluated_at >= COALESCE((SELECT history_before FROM portfolios WHERE id = sqlc.arg(portfolio_id)), CAST('1000-01-01' AS DATETIME(6))) AND portfolio_id = sqlc.arg(portfolio_id) AND evaluated_at >= sqlc.arg(from_at) AND evaluated_at <= sqlc.arg(to_at)
ORDER BY evaluated_at ASC, cycle_id ASC;

-- name: ListValuationPointSummariesByIDs :many
SELECT portfolio_id, cycle_id, evaluated_at, total_usd, account_count, stale_account_count, incomplete_account_count,
       calculation_version, detail_state
FROM valuation_points WHERE portfolio_id = sqlc.arg(portfolio_id) AND cycle_id IN (sqlc.slice('ids'))
ORDER BY evaluated_at ASC, cycle_id ASC;

-- name: ListValuationPointPage :many
SELECT portfolio_id, cycle_id, evaluated_at, total_usd, account_count, stale_account_count, incomplete_account_count,
       calculation_version, detail_state
FROM valuation_points WHERE evaluated_at >= COALESCE((SELECT history_before FROM portfolios WHERE id = sqlc.arg(portfolio_id)), CAST('1000-01-01' AS DATETIME(6))) AND portfolio_id = sqlc.arg(portfolio_id) AND evaluated_at >= sqlc.arg(from_at) AND evaluated_at <= sqlc.arg(to_at)
ORDER BY evaluated_at DESC, cycle_id DESC LIMIT ? OFFSET ?;

-- name: CountValuationPointsBetween :one
SELECT COUNT(*) FROM valuation_points WHERE evaluated_at >= COALESCE((SELECT history_before FROM portfolios WHERE id = sqlc.arg(portfolio_id)), CAST('1000-01-01' AS DATETIME(6))) AND portfolio_id = sqlc.arg(portfolio_id) AND evaluated_at >= sqlc.arg(from_at) AND evaluated_at <= sqlc.arg(to_at);

-- name: InsertAccountValuation :exec
INSERT INTO account_valuations (portfolio_id, cycle_id, account_id, source_report_id, valuation_method, value_usd)
VALUES (sqlc.arg(portfolio_id), sqlc.arg(cycle_id), sqlc.arg(account_id), sqlc.narg(source_report_id), sqlc.arg(valuation_method), sqlc.narg(value_usd));

-- name: InsertBoundaryEvent :exec
INSERT INTO boundary_events (portfolio_id, cycle_id, account_id, direction, valuation_method, value_usd, valuation_complete)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListBalanceValuationsByCycles :many
SELECT v.*, b.equity, a.symbol AS asset_symbol
FROM balance_valuations v
JOIN balances b ON b.portfolio_id = v.portfolio_id AND b.report_id = v.source_report_id AND b.asset_id = v.asset_id
JOIN assets a ON a.portfolio_id = v.portfolio_id AND a.id = v.asset_id
WHERE v.portfolio_id = sqlc.arg(portfolio_id) AND v.cycle_id IN (sqlc.slice('cycle_ids'))
ORDER BY v.cycle_id, v.account_id, a.symbol, v.asset_id;

-- name: ListBoundaryEventsByCycles :many
SELECT e.*, a.identity_id, a.account_type, i.account_structure, i.display_name AS identity_name, i.platform
FROM boundary_events e
JOIN accounts a ON a.portfolio_id = e.portfolio_id AND a.id = e.account_id
JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE e.portfolio_id = sqlc.arg(portfolio_id) AND e.cycle_id IN (sqlc.slice('cycle_ids'))
ORDER BY e.cycle_id, e.account_id;

-- name: ListBoundaryValuesBetween :many
SELECT e.cycle_id, e.direction, e.value_usd
FROM boundary_events e
JOIN valuation_points p ON p.portfolio_id = e.portfolio_id AND p.cycle_id = e.cycle_id
WHERE e.portfolio_id = sqlc.arg(portfolio_id) AND p.evaluated_at >= sqlc.arg(from_at) AND p.evaluated_at <= sqlc.arg(to_at);

-- One earlier anchor supplies the first predecessor without scanning old
-- history once per requested point. Equal timestamps remain ordered by cycle ID.
-- The covering timeline prevents a portfolio-wide primary-key scan after imports.
-- name: ListPointComparisonTimeline :many
SELECT p.cycle_id, p.evaluated_at, p.calculation_version FROM valuation_points p FORCE INDEX (idx_valuation_points_timeline)
WHERE p.portfolio_id = sqlc.arg(portfolio_id)
  AND p.evaluated_at >= sqlc.arg(from_at) AND p.evaluated_at <= sqlc.arg(to_at)
UNION ALL
SELECT anchor.cycle_id, anchor.evaluated_at, anchor.calculation_version FROM (
    SELECT earlier.cycle_id, earlier.evaluated_at, earlier.calculation_version FROM valuation_points earlier FORCE INDEX (idx_valuation_points_timeline)
    WHERE earlier.portfolio_id = sqlc.arg(portfolio_id) AND earlier.evaluated_at < sqlc.arg(from_at)
    ORDER BY earlier.evaluated_at DESC, earlier.cycle_id DESC LIMIT 1
) anchor
ORDER BY evaluated_at, cycle_id;

-- Account reports establish identity membership even before its first manual
-- report. Quantities follow the adopted source, never a newer pending input.
-- name: ListManualComparisonInputsByCycles :many
SELECT r.cycle_id, r.account_id, a.identity_id, a.account_type,
       src.manual_revision_number, b.asset_id, b.quantity
FROM account_reports r
JOIN accounts a ON a.portfolio_id = r.portfolio_id AND a.id = r.account_id
LEFT JOIN account_valuations v ON v.portfolio_id = r.portfolio_id AND v.cycle_id = r.cycle_id AND v.account_id = r.account_id AND a.account_type = 'manual'
LEFT JOIN account_reports src ON src.portfolio_id = v.portfolio_id AND src.id = v.source_report_id
LEFT JOIN manual_revision_balances b ON b.portfolio_id = src.portfolio_id AND b.account_id = src.account_id AND b.revision_number = src.manual_revision_number
WHERE r.portfolio_id = sqlc.arg(portfolio_id) AND r.cycle_id IN (sqlc.slice('cycle_ids'));

-- name: InsertManualBalanceChange :exec
INSERT INTO manual_balance_changes (portfolio_id, cycle_id, account_id) VALUES (?, ?, ?);

-- name: ListManualBalanceChangesByCycles :many
SELECT m.cycle_id, m.account_id, a.account_type FROM manual_balance_changes m
JOIN accounts a ON a.portfolio_id = m.portfolio_id AND a.id = m.account_id
WHERE m.portfolio_id = sqlc.arg(portfolio_id) AND m.cycle_id IN (sqlc.slice('cycle_ids'))
ORDER BY m.cycle_id, m.account_id;

-- name: InsertCalculatedAssetChange :exec
INSERT INTO calculated_asset_changes (portfolio_id, asset_id, evaluated_at, cycle_id, treatment, canonical_asset_symbol)
VALUES (?, ?, ?, ?, ?, ?);

-- Correlated latest-change lookups name the full primary-key prefix in ORDER
-- BY; otherwise MySQL sorts each asset's whole history instead of reading its
-- newest row backward. Outer-row bounds never seek, so a bounded lookup walks
-- back over the asset's later changes.
-- name: ListLatestCalculatedAssetChanges :many
SELECT s.* FROM (
    SELECT a.portfolio_id, a.id AS asset_id, (
        SELECT latest.cycle_id FROM calculated_asset_changes latest
        WHERE latest.portfolio_id = a.portfolio_id AND latest.asset_id = a.id
        ORDER BY latest.portfolio_id DESC, latest.asset_id DESC, latest.evaluated_at DESC, latest.cycle_id DESC LIMIT 1
    ) AS cycle_id
    FROM assets a WHERE a.portfolio_id = sqlc.arg(portfolio_id)
) latest
JOIN calculated_asset_changes s ON s.portfolio_id = latest.portfolio_id AND s.asset_id = latest.asset_id AND s.cycle_id = latest.cycle_id;

-- A mapping change is an included asset whose previous change was included
-- under another canonical asset. Only a mapping update changes an asset's
-- canonical asset, and it advances the mapping version, so the join starts from
-- remapped assets; otherwise every change in a range read walks its asset's
-- later history.
-- name: ListAssetMappingChangesByCycles :many
SELECT c.cycle_id, c.asset_id, a.symbol
FROM assets a
STRAIGHT_JOIN calculated_asset_changes c ON c.portfolio_id = a.portfolio_id AND c.asset_id = a.id
WHERE a.portfolio_id = sqlc.arg(portfolio_id) AND a.mapping_version > 1
  AND c.cycle_id IN (sqlc.slice('cycle_ids')) AND c.treatment = 'included'
  AND (
    SELECT prior.canonical_asset_symbol FROM calculated_asset_changes prior
    WHERE prior.portfolio_id = c.portfolio_id AND prior.asset_id = c.asset_id
      AND (prior.evaluated_at < c.evaluated_at OR (prior.evaluated_at = c.evaluated_at AND prior.cycle_id < c.cycle_id))
    ORDER BY prior.portfolio_id DESC, prior.asset_id DESC, prior.evaluated_at DESC, prior.cycle_id DESC LIMIT 1
  ) <> c.canonical_asset_symbol
ORDER BY c.cycle_id, c.asset_id;

-- name: ListPointCalculatedAssets :many
SELECT DISTINCT b.asset_id, a.symbol, v.canonical_asset_symbol, v.pricing_method = 'excluded' AS excluded
FROM account_valuations av
JOIN balance_valuations v ON v.portfolio_id = av.portfolio_id AND v.cycle_id = av.cycle_id AND v.account_id = av.account_id
JOIN balances b ON b.portfolio_id = v.portfolio_id AND b.report_id = v.source_report_id AND b.asset_id = v.asset_id
JOIN assets a ON a.portfolio_id = b.portfolio_id AND a.id = b.asset_id
WHERE av.portfolio_id = sqlc.arg(portfolio_id) AND av.cycle_id = sqlc.arg(cycle_id)
  AND av.valuation_method = 'calculated' AND b.equity <> 0
ORDER BY b.asset_id;

-- name: GetRetainedSnapshotBefore :one
SELECT * FROM valuation_points WHERE evaluated_at >= COALESCE((SELECT history_before FROM portfolios WHERE id = sqlc.arg(portfolio_id)), CAST('1000-01-01' AS DATETIME(6))) AND portfolio_id = sqlc.arg(portfolio_id) AND evaluated_at < sqlc.arg(evaluated_at) AND detail_state IN ('full','daily')
ORDER BY evaluated_at DESC, cycle_id DESC LIMIT 1;

-- Read only points that can contribute comparison evidence. The version window
-- includes one predecessor, so A -> B -> A and equal-time cycle ordering survive.
-- name: ListComparisonPointHeadersBetween :many
SELECT p.cycle_id, p.evaluated_at
FROM manual_balance_changes m
STRAIGHT_JOIN valuation_points p ON p.portfolio_id = m.portfolio_id AND p.cycle_id = m.cycle_id
WHERE m.portfolio_id = sqlc.arg(portfolio_id) AND p.evaluated_at >= sqlc.arg(from_at) AND p.evaluated_at <= sqlc.arg(to_at)
UNION
SELECT p.cycle_id, p.evaluated_at
FROM boundary_events e
STRAIGHT_JOIN valuation_points p ON p.portfolio_id = e.portfolio_id AND p.cycle_id = e.cycle_id
WHERE e.portfolio_id = sqlc.arg(portfolio_id) AND p.evaluated_at >= sqlc.arg(from_at) AND p.evaluated_at <= sqlc.arg(to_at)
UNION
SELECT p.cycle_id, p.evaluated_at
FROM assets a
STRAIGHT_JOIN calculated_asset_changes c ON c.portfolio_id = a.portfolio_id AND c.asset_id = a.id
STRAIGHT_JOIN valuation_points p ON p.portfolio_id = c.portfolio_id AND p.cycle_id = c.cycle_id
WHERE a.portfolio_id = sqlc.arg(portfolio_id) AND a.mapping_version > 1 AND c.treatment = 'included'
  AND p.evaluated_at >= sqlc.arg(from_at) AND p.evaluated_at <= sqlc.arg(to_at)
UNION
SELECT versions.cycle_id, versions.evaluated_at FROM (
    SELECT timeline.cycle_id, timeline.evaluated_at, timeline.calculation_version,
           LAG(timeline.calculation_version) OVER (ORDER BY timeline.evaluated_at, timeline.cycle_id) AS previous_version
    FROM (
        SELECT current.cycle_id, current.evaluated_at, current.calculation_version FROM valuation_points current FORCE INDEX (idx_valuation_points_timeline)
        WHERE current.portfolio_id = sqlc.arg(portfolio_id) AND current.evaluated_at >= sqlc.arg(from_at) AND current.evaluated_at <= sqlc.arg(to_at)
        UNION ALL
        SELECT anchor.cycle_id, anchor.evaluated_at, anchor.calculation_version FROM (
            SELECT earlier.cycle_id, earlier.evaluated_at, earlier.calculation_version FROM valuation_points earlier FORCE INDEX (idx_valuation_points_timeline)
            WHERE earlier.portfolio_id = sqlc.arg(portfolio_id) AND earlier.evaluated_at < sqlc.arg(from_at)
            ORDER BY earlier.evaluated_at DESC, earlier.cycle_id DESC LIMIT 1
        ) anchor
    ) timeline
) versions WHERE versions.calculation_version <> versions.previous_version
ORDER BY evaluated_at, cycle_id;

-- name: ListBoundaryPointHeadersBetween :many
SELECT DISTINCT p.cycle_id, p.evaluated_at
FROM boundary_events e
STRAIGHT_JOIN valuation_points p ON p.portfolio_id = e.portfolio_id AND p.cycle_id = e.cycle_id
WHERE p.evaluated_at >= COALESCE((SELECT history_before FROM portfolios WHERE id = sqlc.arg(portfolio_id)), CAST('1000-01-01' AS DATETIME(6))) AND e.portfolio_id = sqlc.arg(portfolio_id) AND p.evaluated_at >= sqlc.arg(from_at) AND p.evaluated_at <= sqlc.arg(to_at)
ORDER BY p.evaluated_at, p.cycle_id;

-- name: GetHistoryValuationPointBefore :one
SELECT * FROM valuation_points FORCE INDEX (idx_valuation_points_timeline)
WHERE sqlc.narg(at) >= COALESCE((SELECT history_before FROM portfolios WHERE id = sqlc.arg(portfolio_id)), CAST('1000-01-01' AS DATETIME(6))) AND portfolio_id = sqlc.arg(portfolio_id) AND (evaluated_at < sqlc.narg(at) OR (evaluated_at = sqlc.narg(at) AND cycle_id < sqlc.arg(cycle_id)))
ORDER BY evaluated_at DESC, cycle_id DESC
LIMIT 1;


-- name: IsPointInRetainedHistory :one
SELECT EXISTS(SELECT 1 FROM valuation_points p JOIN portfolios owner ON owner.id = p.portfolio_id
WHERE p.portfolio_id = sqlc.arg(portfolio_id) AND p.cycle_id = sqlc.arg(cycle_id)
AND (owner.history_before IS NULL OR p.evaluated_at >= owner.history_before)) AS available;

-- name: ListAccountBalanceValuations :many
SELECT v.*, b.equity, a.symbol AS asset_symbol
FROM balance_valuations v
JOIN balances b ON b.portfolio_id = v.portfolio_id AND b.report_id = v.source_report_id AND b.asset_id = v.asset_id
JOIN assets a ON a.portfolio_id = v.portfolio_id AND a.id = v.asset_id
WHERE v.portfolio_id = sqlc.arg(portfolio_id) AND v.cycle_id = sqlc.arg(cycle_id) AND v.account_id = sqlc.arg(account_id)
ORDER BY v.cycle_id, v.account_id, a.symbol, v.asset_id;

-- name: ListValuationPointHeaderPage :many
SELECT cycle_id, evaluated_at FROM valuation_points
WHERE evaluated_at >= COALESCE((SELECT history_before FROM portfolios WHERE portfolios.id = sqlc.arg(portfolio_id)), CAST('1000-01-01' AS DATETIME(6)))
  AND portfolio_id = sqlc.arg(portfolio_id) AND evaluated_at >= sqlc.arg(from_at) AND evaluated_at <= sqlc.arg(to_at)
  AND (sqlc.arg(has_cursor) = 0 OR evaluated_at > sqlc.arg(cursor_at) OR (evaluated_at = sqlc.arg(cursor_at) AND cycle_id > sqlc.arg(cursor_id)))
ORDER BY evaluated_at, cycle_id LIMIT 1000;

-- name: ListBoundaryEventPage :many
SELECT e.*, p.evaluated_at, a.identity_id, a.account_type, i.account_structure, i.display_name AS identity_name, i.platform
FROM valuation_points p
JOIN boundary_events e ON e.portfolio_id = p.portfolio_id AND e.cycle_id = p.cycle_id
JOIN accounts a ON a.portfolio_id = e.portfolio_id AND a.id = e.account_id
JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE p.evaluated_at >= COALESCE((SELECT history_before FROM portfolios WHERE portfolios.id = sqlc.arg(portfolio_id)), CAST('1000-01-01' AS DATETIME(6)))
  AND p.portfolio_id = sqlc.arg(portfolio_id) AND p.evaluated_at >= sqlc.arg(from_at) AND p.evaluated_at <= sqlc.arg(to_at)
ORDER BY p.evaluated_at DESC, p.cycle_id DESC,
         FIND_IN_SET(i.platform, sqlc.arg(platform_order)) = 0,
         FIND_IN_SET(i.platform, sqlc.arg(platform_order)), i.platform,
         CAST(LOWER(i.display_name) AS BINARY), i.id,
         FIND_IN_SET(CONCAT(i.account_structure, ':', a.account_type), sqlc.arg(account_order)) = 0,
         FIND_IN_SET(CONCAT(i.account_structure, ':', a.account_type), sqlc.arg(account_order)), a.account_type, a.id
LIMIT ? OFFSET ?;

-- name: CountBoundaryEvents :one
SELECT COUNT(*) FROM valuation_points p
JOIN boundary_events e ON e.portfolio_id = p.portfolio_id AND e.cycle_id = p.cycle_id
WHERE p.evaluated_at >= COALESCE((SELECT history_before FROM portfolios WHERE portfolios.id = sqlc.arg(portfolio_id)), CAST('1000-01-01' AS DATETIME(6)))
  AND p.portfolio_id = sqlc.arg(portfolio_id) AND p.evaluated_at >= sqlc.arg(from_at) AND p.evaluated_at <= sqlc.arg(to_at);
