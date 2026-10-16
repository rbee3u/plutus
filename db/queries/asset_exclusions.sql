-- name: ListCurrentAssetExclusions :many
SELECT e.asset_id, e.excluded, e.reason, e.version, e.effective_at
FROM (
    SELECT v.portfolio_id, v.asset_id, MAX(v.version) AS version
    FROM asset_exclusion_versions v WHERE v.portfolio_id = sqlc.arg(portfolio_id)
    GROUP BY v.portfolio_id, v.asset_id
) latest
JOIN asset_exclusion_versions e ON e.portfolio_id = latest.portfolio_id AND e.asset_id = latest.asset_id AND e.version = latest.version;

-- name: LockPortfolio :one
SELECT id FROM portfolios WHERE id = ? AND deletion_requested_at IS NULL FOR UPDATE;

-- name: GetAssetExclusion :one
SELECT excluded, reason, version, effective_at
FROM asset_exclusion_versions WHERE portfolio_id = ? AND asset_id = ?
ORDER BY version DESC LIMIT 1;

-- name: InsertAssetExclusionVersion :exec
INSERT INTO asset_exclusion_versions
    (portfolio_id, asset_id, version, excluded, reason, effective_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: ListAssetExclusionsBefore :many
SELECT asset_id, excluded, version, effective_at
FROM asset_exclusion_versions
WHERE portfolio_id = ? AND effective_at < ?
ORDER BY asset_id, version DESC;

-- Keep one opening version per asset and only changes within the window.
-- Effective times need not follow version order; MAX selects the adopted rule.
-- name: ListAssetExclusionsForWindow :many
SELECT e.asset_id, e.excluded, e.version, e.effective_at
FROM (
    SELECT v.portfolio_id, v.asset_id, MAX(v.version) AS version
    FROM asset_exclusion_versions v
    WHERE v.portfolio_id = sqlc.arg(portfolio_id) AND v.effective_at < sqlc.arg(from_at)
    GROUP BY v.portfolio_id, v.asset_id
) opening
JOIN asset_exclusion_versions e ON e.portfolio_id = opening.portfolio_id AND e.asset_id = opening.asset_id AND e.version = opening.version
UNION ALL
SELECT delta.asset_id, delta.excluded, delta.version, delta.effective_at
FROM asset_exclusion_versions delta
WHERE delta.portfolio_id = sqlc.arg(portfolio_id) AND delta.effective_at >= sqlc.arg(from_at) AND delta.effective_at < sqlc.arg(to_at)
ORDER BY effective_at, version;

-- name: ListCurrentAssetExclusionsByAssets :many
SELECT e.asset_id, e.excluded
FROM assets a
JOIN asset_exclusion_versions e ON e.portfolio_id = a.portfolio_id AND e.asset_id = a.id
    AND e.version = (SELECT v.version FROM asset_exclusion_versions v WHERE v.portfolio_id = a.portfolio_id AND v.asset_id = a.id ORDER BY v.portfolio_id DESC, v.asset_id DESC, v.version DESC LIMIT 1)
WHERE a.portfolio_id = sqlc.arg(portfolio_id) AND a.id IN (sqlc.slice('asset_ids'));

-- One backward primary-key walk per asset and endpoint reads only the asset's
-- changes after that endpoint; see ListLatestCalculatedAssetChanges for the
-- ORDER BY constraint.
-- name: ListCalculatedAssetsAtCycles :many
SELECT DISTINCT a.id, a.platform, a.symbol
FROM valuation_points p
JOIN assets a ON a.portfolio_id = p.portfolio_id
WHERE p.portfolio_id = sqlc.arg(portfolio_id) AND p.cycle_id IN (sqlc.arg(start_cycle_id), sqlc.arg(end_cycle_id))
  AND a.id IN (sqlc.slice('asset_ids'))
  AND COALESCE((
    SELECT prior.treatment <> 'absent' FROM calculated_asset_changes prior
    WHERE prior.portfolio_id = a.portfolio_id AND prior.asset_id = a.id
      AND (prior.evaluated_at < p.evaluated_at OR (prior.evaluated_at = p.evaluated_at AND prior.cycle_id <= p.cycle_id))
    ORDER BY prior.portfolio_id DESC, prior.asset_id DESC, prior.evaluated_at DESC, prior.cycle_id DESC LIMIT 1
  ), FALSE)
ORDER BY a.platform, a.symbol;

-- name: GetPortfolioHistoryBoundary :one
SELECT history_before FROM portfolios WHERE id = sqlc.arg(portfolio_id);

-- Constant asset IDs let the primary key bound both asset and time. An
-- asset-driven correlated join filters time after walking the asset's history.
-- name: ListCalculatedAssetChangesBetween :many
SELECT c.asset_id, c.evaluated_at, c.cycle_id, c.treatment, a.platform, a.symbol
FROM calculated_asset_changes c FORCE INDEX (PRIMARY)
STRAIGHT_JOIN assets a ON a.portfolio_id = c.portfolio_id AND a.id = c.asset_id
WHERE c.portfolio_id = sqlc.arg(portfolio_id) AND c.asset_id IN (sqlc.slice('asset_ids'))
  AND c.evaluated_at >= sqlc.arg(from_at) AND c.evaluated_at <= sqlc.arg(to_at)
ORDER BY c.evaluated_at, c.cycle_id;
