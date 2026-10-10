-- Release self and stale references only inside the Portfolio deletion transaction.
-- The report CHECK requires the missing method and unknown value together.
-- name: ReleasePortfolioReportSources :exec
UPDATE account_reports
SET method = 'missing', value_usd = NULL, source_report_id = NULL
WHERE portfolio_id = ?;

-- name: DeletePortfolioBalances :exec
DELETE FROM balances WHERE portfolio_id = ?;

-- name: DeletePortfolioPositions :exec
DELETE FROM positions WHERE portfolio_id = ?;

-- name: DeletePortfolioEarnPositions :exec
DELETE FROM earn_positions WHERE portfolio_id = ?;

-- name: DeletePortfolioDualPositions :exec
DELETE FROM dual_positions WHERE portfolio_id = ?;

-- name: DeletePortfolioManualCycleAccounts :exec
DELETE FROM manual_cycle_accounts WHERE portfolio_id = ?;

-- name: DeletePortfolioManualBalances :exec
DELETE FROM manual_balances WHERE portfolio_id = ?;

-- name: DeletePortfolioManualRevisions :exec
DELETE FROM manual_revisions WHERE portfolio_id = ?;

-- name: DeletePortfolioCapitalFlows :exec
DELETE FROM capital_flows WHERE portfolio_id = ?;

-- name: DeletePortfolioValuationPoints :exec
DELETE FROM valuation_points WHERE portfolio_id = ?;

-- name: DeletePortfolioAuditEvents :exec
DELETE FROM audit_events WHERE portfolio_id = ?;

-- name: DeletePortfolioIdempotencyRecords :exec
DELETE FROM idempotency_records WHERE portfolio_id = ?;

-- name: DeletePortfolioAssetExclusionEvents :exec
DELETE FROM portfolio_asset_exclusion_events WHERE portfolio_id = ?;

-- name: DeletePortfolioAssetExclusions :exec
DELETE FROM portfolio_asset_exclusions WHERE portfolio_id = ?;

-- name: DeletePortfolioAccountReports :exec
DELETE FROM account_reports WHERE portfolio_id = ?;

-- name: DeletePortfolioManualCycles :exec
DELETE FROM manual_cycles WHERE portfolio_id = ?;

-- name: DeletePortfolioAccounts :exec
DELETE FROM accounts WHERE portfolio_id = ?;

-- name: DeletePortfolioJobs :exec
DELETE FROM jobs WHERE portfolio_id = ?;

-- name: DeletePortfolioIdentities :exec
DELETE FROM identities WHERE portfolio_id = ?;

