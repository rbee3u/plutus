-- name: GetManualAccount :one
SELECT a.id, a.version, i.platform, i.status
FROM accounts a JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE a.portfolio_id = sqlc.arg(portfolio_id) AND a.identity_id = sqlc.arg(identity_id) AND a.account_type = 'manual';

-- name: LockManualAccount :one
SELECT a.id, a.version, i.platform, i.status
FROM accounts a JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE a.portfolio_id = sqlc.arg(portfolio_id) AND a.identity_id = sqlc.arg(identity_id) AND a.account_type = 'manual'
FOR UPDATE;

-- name: ListManualBalances :many
SELECT b.symbol, b.listing_id, b.quantity, b.modified_at, l.asset_symbol
FROM manual_balances b JOIN asset_listings l ON l.id = b.listing_id
WHERE b.portfolio_id = sqlc.arg(portfolio_id) AND b.account_id = sqlc.arg(account_id)
ORDER BY b.symbol;

-- name: PutManualBalance :exec
INSERT INTO manual_balances (portfolio_id, account_id, symbol, listing_id, quantity, modified_at)
VALUES (sqlc.arg(portfolio_id), sqlc.arg(account_id), sqlc.arg(symbol), sqlc.arg(listing_id), sqlc.arg(quantity), sqlc.arg(modified_at))
ON DUPLICATE KEY UPDATE quantity = VALUES(quantity), modified_at = VALUES(modified_at);

-- name: DeleteManualBalance :exec
DELETE FROM manual_balances WHERE portfolio_id = sqlc.arg(portfolio_id) AND account_id = sqlc.arg(account_id) AND symbol = sqlc.arg(symbol);

-- name: AdvanceManualVersion :exec
UPDATE accounts SET version = version + 1, updated_at = sqlc.arg(at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(account_id) AND account_type = 'manual';

-- name: InsertManualRevision :exec
INSERT INTO manual_revisions (portfolio_id, account_id, version, balances_json, modified_at)
VALUES (sqlc.arg(portfolio_id), sqlc.arg(account_id), sqlc.arg(version), sqlc.arg(balances_json), sqlc.arg(modified_at));

-- name: GetManualModifiedAt :one
SELECT modified_at FROM manual_revisions
WHERE portfolio_id = sqlc.arg(portfolio_id) AND account_id = sqlc.arg(account_id)
ORDER BY version DESC LIMIT 1;

-- name: LockManualCycleJob :one
SELECT id FROM jobs WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(job_id) FOR UPDATE;

-- name: GetManualCycle :one
SELECT job_id FROM manual_cycles WHERE portfolio_id = sqlc.arg(portfolio_id) AND job_id = sqlc.arg(job_id);

-- name: InsertManualCycle :exec
INSERT INTO manual_cycles (portfolio_id, job_id, captured_at)
VALUES (sqlc.arg(portfolio_id), sqlc.arg(job_id), sqlc.arg(captured_at));

-- name: LockManualBoundary :many
SELECT a.id, a.identity_id, a.version, i.platform
FROM accounts a JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE a.portfolio_id = sqlc.arg(portfolio_id) AND a.account_type = 'manual' AND i.status = 'enabled'
ORDER BY a.id FOR UPDATE;

-- name: InsertManualCycleAccount :exec
INSERT INTO manual_cycle_accounts (portfolio_id, job_id, account_id, version, balances_json, modified_at)
VALUES (sqlc.arg(portfolio_id), sqlc.arg(job_id), sqlc.arg(account_id), sqlc.arg(version), sqlc.arg(balances_json), sqlc.narg(modified_at));

-- name: ListManualCycleAccounts :many
SELECT m.account_id, m.version, m.balances_json, m.modified_at, a.identity_id, i.platform
FROM manual_cycle_accounts m
JOIN accounts a ON a.portfolio_id = m.portfolio_id AND a.id = m.account_id
JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
WHERE m.portfolio_id = sqlc.arg(portfolio_id) AND m.job_id = sqlc.arg(job_id)
ORDER BY m.account_id;

-- name: GetAppliedManualState :one
SELECT m.version, m.balances_json
FROM valuation_points p JOIN manual_cycle_accounts m ON m.portfolio_id = p.portfolio_id AND m.job_id = p.job_id
WHERE p.portfolio_id = sqlc.arg(portfolio_id) AND m.account_id = sqlc.arg(account_id)
ORDER BY p.evaluated_at DESC, p.id DESC LIMIT 1;

-- name: SetManualReportMetadata :exec
UPDATE account_reports SET manual_state_json = sqlc.arg(state_json), manual_modified_at = sqlc.narg(modified_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND job_id = sqlc.arg(job_id) AND account_id = sqlc.arg(account_id);

-- name: ReportHasHoldings :one
SELECT (
    EXISTS(SELECT 1 FROM balances b WHERE b.report_id = sqlc.arg(source_id) AND (b.equity IS NULL OR b.equity <> 0 OR COALESCE(b.liability, 0) <> 0 OR COALESCE(b.interest, 0) <> 0 OR COALESCE(b.unrealized_pnl, 0) <> 0 OR COALESCE(b.available_to_trade, 0) <> 0 OR COALESCE(b.available_to_withdraw, 0) <> 0))
    OR EXISTS(SELECT 1 FROM positions p WHERE p.report_id = sqlc.arg(source_id))
    OR EXISTS(SELECT 1 FROM earn_positions e WHERE e.report_id = sqlc.arg(source_id))
    OR EXISTS(SELECT 1 FROM dual_positions d WHERE d.report_id = sqlc.arg(source_id))
) AS has_holdings;
