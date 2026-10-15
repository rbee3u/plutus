-- name: GetManualAccount :one
SELECT a.id, i.platform, i.archived_at
FROM accounts a JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE a.portfolio_id = sqlc.arg(portfolio_id) AND a.identity_id = sqlc.arg(identity_id) AND a.account_type = 'manual';

-- name: LockManualAccount :one
SELECT a.id, i.platform, i.archived_at
FROM accounts a JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE a.portfolio_id = sqlc.arg(portfolio_id) AND a.identity_id = sqlc.arg(identity_id) AND a.account_type = 'manual'
FOR UPDATE;

-- name: GetCurrentManualRevision :one
SELECT * FROM manual_revisions
WHERE portfolio_id = ? AND account_id = ? ORDER BY revision_number DESC LIMIT 1;

-- name: InsertManualRevision :exec
INSERT INTO manual_revisions (portfolio_id, account_id, revision_number, changed_at)
VALUES (sqlc.arg(portfolio_id), sqlc.arg(account_id), sqlc.arg(revision_number), sqlc.narg(changed_at));

-- name: LockCycleManualReportedAt :one
SELECT manual_reported_at FROM collection_cycles
WHERE portfolio_id = ? AND id = ? FOR UPDATE;

-- name: MarkCycleManualReported :exec
UPDATE collection_cycles SET manual_reported_at = sqlc.arg(at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: ListManualBoundary :many
SELECT a.id, i.platform
FROM accounts a JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE a.portfolio_id = sqlc.arg(portfolio_id) AND a.account_type = 'manual' AND i.archived_at IS NULL
ORDER BY a.id;

-- The applied revision is the latest one whose own cycle has committed, even after its point expires.
-- Serialized cycles make report time follow point order. The account's
-- fallback range stops at its latest committed report; the optimizer can
-- prefer a portfolio-wide time scan that walks every newer report of other
-- accounts, so the fallback index is forced.
-- name: GetAppliedManualRevision :one
SELECT r.manual_revision_number
FROM account_reports r FORCE INDEX (idx_account_reports_fallback)
JOIN collection_cycles c ON c.portfolio_id = r.portfolio_id AND c.id = r.cycle_id AND (c.status = 'succeeded' OR EXISTS(SELECT 1 FROM valuation_points p WHERE p.portfolio_id = c.portfolio_id AND p.cycle_id = c.id))
WHERE r.portfolio_id = sqlc.arg(portfolio_id) AND r.account_id = sqlc.arg(account_id) AND r.status = 'succeeded' AND r.manual_revision_number IS NOT NULL
ORDER BY r.recorded_at DESC, r.id DESC LIMIT 1;

-- name: ListPortfolioManualRevisions :many
SELECT r.account_id, r.revision_number, r.changed_at
FROM manual_revisions r
WHERE r.portfolio_id = sqlc.arg(portfolio_id)
AND r.revision_number = (SELECT MAX(v.revision_number) FROM manual_revisions v WHERE v.portfolio_id = r.portfolio_id AND v.account_id = r.account_id)
ORDER BY r.account_id;

-- name: ReportHasHoldings :one
SELECT (
    EXISTS(SELECT 1 FROM balances b WHERE b.portfolio_id = sqlc.arg(portfolio_id) AND b.report_id = sqlc.arg(source_id) AND (b.equity IS NULL OR b.equity <> 0 OR COALESCE(b.liability, 0) <> 0 OR COALESCE(b.interest, 0) <> 0 OR COALESCE(b.unrealized_pnl, 0) <> 0 OR COALESCE(b.available_to_trade, 0) <> 0 OR COALESCE(b.available_to_withdraw, 0) <> 0))
    OR EXISTS(SELECT 1 FROM futures_positions p WHERE p.portfolio_id = sqlc.arg(portfolio_id) AND p.report_id = sqlc.arg(source_id))
    OR EXISTS(SELECT 1 FROM earn_positions e WHERE e.portfolio_id = sqlc.arg(portfolio_id) AND e.report_id = sqlc.arg(source_id))
    OR EXISTS(SELECT 1 FROM dual_positions d WHERE d.portfolio_id = sqlc.arg(portfolio_id) AND d.report_id = sqlc.arg(source_id))
) AS has_holdings;

-- name: InsertManualRevisionBalance :exec
INSERT INTO manual_revision_balances (portfolio_id, account_id, revision_number, asset_id, quantity, changed_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: ListManualRevisionBalances :many
SELECT b.asset_id, b.quantity, b.changed_at, a.symbol AS asset_symbol, a.canonical_asset_symbol
FROM manual_revision_balances b JOIN assets a ON a.portfolio_id = b.portfolio_id AND a.id = b.asset_id
WHERE b.portfolio_id = ? AND b.account_id = ? AND b.revision_number = ? ORDER BY a.symbol, b.asset_id;
