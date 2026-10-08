-- name: ListCurrentListingExclusions :many
SELECT listing_id, ignored, reason, version, updated_at
FROM portfolio_listing_exclusions WHERE portfolio_id = ?;

-- name: LockExclusionPortfolio :one
SELECT id FROM portfolios WHERE id = ? FOR UPDATE;

-- name: GetListingExclusion :one
SELECT ignored, reason, version, updated_at
FROM portfolio_listing_exclusions WHERE portfolio_id = ? AND listing_id = ?;

-- name: UpsertListingExclusion :exec
INSERT INTO portfolio_listing_exclusions
    (portfolio_id, listing_id, ignored, reason, version, actor_user_id, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE ignored = VALUES(ignored), reason = VALUES(reason),
    version = VALUES(version), actor_user_id = VALUES(actor_user_id), updated_at = VALUES(updated_at);

-- name: InsertListingExclusionEvent :exec
INSERT INTO portfolio_listing_exclusion_events
    (portfolio_id, listing_id, version, ignored, reason, actor_user_id, effective_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListListingExclusionsBefore :many
SELECT listing_id, ignored
FROM portfolio_listing_exclusion_events
WHERE portfolio_id = ? AND effective_at < ?
ORDER BY listing_id, version DESC;

-- name: ListListingExclusionHistory :many
SELECT listing_id, effective_at, ignored
FROM portfolio_listing_exclusion_events WHERE portfolio_id = ?
ORDER BY listing_id, version DESC;

-- name: ListCalculatedListingsAtJobs :many
SELECT DISTINCT l.id, l.platform, l.symbol
FROM account_reports r
JOIN balances b ON b.report_id = r.source_report_id
JOIN asset_listings l ON l.id = b.listing_id
WHERE r.portfolio_id = sqlc.arg(portfolio_id)
    AND r.job_id IN (sqlc.arg(start_job_id), sqlc.arg(end_job_id))
    AND r.method = 'calculated' AND b.equity <> 0
ORDER BY l.platform, l.symbol;
