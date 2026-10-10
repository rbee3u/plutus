-- name: ListCurrentAssetExclusions :many
SELECT e.asset_id, e.excluded, e.reason, e.version, e.effective_at
FROM asset_exclusion_versions e
WHERE e.portfolio_id = sqlc.arg(portfolio_id)
  AND e.version = (SELECT MAX(v.version) FROM asset_exclusion_versions v WHERE v.portfolio_id = e.portfolio_id AND v.asset_id = e.asset_id);

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
SELECT asset_id, excluded
FROM asset_exclusion_versions
WHERE portfolio_id = ? AND effective_at < ?
ORDER BY asset_id, version DESC;

-- One backward primary-key walk per Asset and endpoint reads only the Asset's
-- changes after that endpoint; see ListLatestCalculatedAssetChanges for the
-- ORDER BY constraint.
-- name: ListCalculatedAssetsAtCycles :many
SELECT DISTINCT a.id, a.platform, a.symbol
FROM valuation_points p
JOIN assets a ON a.portfolio_id = p.portfolio_id
WHERE p.portfolio_id = sqlc.arg(portfolio_id) AND p.cycle_id IN (sqlc.arg(start_cycle_id), sqlc.arg(end_cycle_id))
  AND COALESCE((
    SELECT prior.treatment <> 'absent' FROM calculated_asset_changes prior
    WHERE prior.portfolio_id = a.portfolio_id AND prior.asset_id = a.id
      AND (prior.evaluated_at < p.evaluated_at OR (prior.evaluated_at = p.evaluated_at AND prior.cycle_id <= p.cycle_id))
    ORDER BY prior.portfolio_id DESC, prior.asset_id DESC, prior.evaluated_at DESC, prior.cycle_id DESC LIMIT 1
  ), FALSE)
ORDER BY a.platform, a.symbol;
