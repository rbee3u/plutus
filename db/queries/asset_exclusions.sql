-- name: ListCurrentAssetExclusions :many
SELECT asset_id, ignored, reason, version, updated_at
FROM portfolio_asset_exclusions WHERE portfolio_id = ?;

-- name: LockExclusionPortfolio :one
SELECT id FROM portfolios WHERE id = ? FOR UPDATE;

-- name: GetAssetExclusion :one
SELECT ignored, reason, version, updated_at
FROM portfolio_asset_exclusions WHERE portfolio_id = ? AND asset_id = ?;

-- name: UpsertAssetExclusion :exec
INSERT INTO portfolio_asset_exclusions
    (portfolio_id, asset_id, ignored, reason, version, actor_user_id, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE ignored = VALUES(ignored), reason = VALUES(reason),
    version = VALUES(version), actor_user_id = VALUES(actor_user_id), updated_at = VALUES(updated_at);

-- name: InsertAssetExclusionEvent :exec
INSERT INTO portfolio_asset_exclusion_events
    (portfolio_id, asset_id, version, ignored, reason, actor_user_id, effective_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListAssetExclusionsBefore :many
SELECT asset_id, ignored
FROM portfolio_asset_exclusion_events
WHERE portfolio_id = ? AND effective_at < ?
ORDER BY asset_id, version DESC;

-- name: ListAssetExclusionHistory :many
SELECT asset_id, effective_at, ignored
FROM portfolio_asset_exclusion_events WHERE portfolio_id = ?
ORDER BY asset_id, version DESC;

-- name: ListCalculatedAssetsAtJobs :many
SELECT DISTINCT asset.id, asset.platform, asset.symbol
FROM account_reports r
JOIN balances b ON b.report_id = r.source_report_id
JOIN assets asset ON asset.id = b.asset_id
WHERE r.portfolio_id = sqlc.arg(portfolio_id)
    AND r.job_id IN (sqlc.arg(start_job_id), sqlc.arg(end_job_id))
    AND r.method = 'calculated' AND b.equity <> 0
ORDER BY asset.platform, asset.symbol;
