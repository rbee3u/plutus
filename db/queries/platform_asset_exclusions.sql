-- name: ListCurrentPlatformAssetExclusions :many
SELECT platform_asset_id, ignored, reason, version, updated_at
FROM portfolio_platform_asset_exclusions WHERE portfolio_id = ?;

-- name: LockExclusionPortfolio :one
SELECT id FROM portfolios WHERE id = ? FOR UPDATE;

-- name: GetPlatformAssetExclusion :one
SELECT ignored, reason, version, updated_at
FROM portfolio_platform_asset_exclusions WHERE portfolio_id = ? AND platform_asset_id = ?;

-- name: UpsertPlatformAssetExclusion :exec
INSERT INTO portfolio_platform_asset_exclusions
    (portfolio_id, platform_asset_id, ignored, reason, version, actor_user_id, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE ignored = VALUES(ignored), reason = VALUES(reason),
    version = VALUES(version), actor_user_id = VALUES(actor_user_id), updated_at = VALUES(updated_at);

-- name: InsertPlatformAssetExclusionEvent :exec
INSERT INTO portfolio_platform_asset_exclusion_events
    (portfolio_id, platform_asset_id, version, ignored, reason, actor_user_id, effective_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListPlatformAssetExclusionsBefore :many
SELECT platform_asset_id, ignored
FROM portfolio_platform_asset_exclusion_events
WHERE portfolio_id = ? AND effective_at < ?
ORDER BY platform_asset_id, version DESC;

-- name: ListPlatformAssetExclusionHistory :many
SELECT platform_asset_id, effective_at, ignored
FROM portfolio_platform_asset_exclusion_events WHERE portfolio_id = ?
ORDER BY platform_asset_id, version DESC;

-- name: ListCalculatedPlatformAssetsAtJobs :many
SELECT DISTINCT l.id, l.platform, l.symbol
FROM account_reports r
JOIN balances b ON b.report_id = r.source_report_id
JOIN platform_assets l ON l.id = b.platform_asset_id
WHERE r.portfolio_id = sqlc.arg(portfolio_id)
    AND r.job_id IN (sqlc.arg(start_job_id), sqlc.arg(end_job_id))
    AND r.method = 'calculated' AND b.equity <> 0
ORDER BY l.platform, l.symbol;
