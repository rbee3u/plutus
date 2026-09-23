-- name: UpsertValuationPoint :exec
INSERT INTO valuation_points (
    id, portfolio_id, job_id, evaluated_at, total_usd, account_count, stale_account_count, incomplete_account_count,
    boundary_contribution_usd, boundary_withdrawal_usd, boundary_json, issues_json, recalculated_at, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(job_id), sqlc.arg(evaluated_at), sqlc.arg(total_usd),
    sqlc.arg(account_count), sqlc.arg(stale_account_count), sqlc.arg(incomplete_account_count),
    sqlc.arg(boundary_contribution_usd), sqlc.arg(boundary_withdrawal_usd), sqlc.narg(boundary_json), sqlc.narg(issues_json),
    NULL, sqlc.arg(created_at)
)
ON DUPLICATE KEY UPDATE
    total_usd = VALUES(total_usd),
    account_count = VALUES(account_count),
    stale_account_count = VALUES(stale_account_count),
    incomplete_account_count = VALUES(incomplete_account_count),
    boundary_contribution_usd = VALUES(boundary_contribution_usd),
    boundary_withdrawal_usd = VALUES(boundary_withdrawal_usd),
    boundary_json = VALUES(boundary_json),
    issues_json = VALUES(issues_json),
    recalculated_at = VALUES(created_at);

-- name: GetFirstValuationPoint :one
SELECT id, portfolio_id, job_id, evaluated_at, total_usd, account_count, stale_account_count, incomplete_account_count,
       boundary_contribution_usd, boundary_withdrawal_usd, boundary_json, issues_json, recalculated_at, created_at
FROM valuation_points
WHERE portfolio_id = sqlc.arg(portfolio_id)
ORDER BY evaluated_at ASC
LIMIT 1;

-- name: GetLatestValuationPoint :one
SELECT id, portfolio_id, job_id, evaluated_at, total_usd, account_count, stale_account_count, incomplete_account_count,
       boundary_contribution_usd, boundary_withdrawal_usd, boundary_json, issues_json, recalculated_at, created_at
FROM valuation_points
WHERE portfolio_id = sqlc.arg(portfolio_id)
ORDER BY evaluated_at DESC
LIMIT 1;

-- name: GetValuationPoint :one
SELECT id, portfolio_id, job_id, evaluated_at, total_usd, account_count, stale_account_count, incomplete_account_count,
       boundary_contribution_usd, boundary_withdrawal_usd, boundary_json, issues_json, recalculated_at, created_at
FROM valuation_points
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: GetValuationPointAtOrBefore :one
SELECT id, portfolio_id, job_id, evaluated_at, total_usd, account_count, stale_account_count, incomplete_account_count,
       boundary_contribution_usd, boundary_withdrawal_usd, boundary_json, issues_json, recalculated_at, created_at
FROM valuation_points
WHERE portfolio_id = sqlc.arg(portfolio_id) AND evaluated_at <= sqlc.arg(at)
ORDER BY evaluated_at DESC
LIMIT 1;

-- name: GetValuationPointBefore :one
SELECT id, portfolio_id, job_id, evaluated_at, total_usd, account_count, stale_account_count, incomplete_account_count,
       boundary_contribution_usd, boundary_withdrawal_usd, boundary_json, issues_json, recalculated_at, created_at
FROM valuation_points
WHERE portfolio_id = sqlc.arg(portfolio_id) AND evaluated_at < sqlc.arg(at)
ORDER BY evaluated_at DESC
LIMIT 1;

-- name: ListValuationPointsBetween :many
SELECT id, portfolio_id, job_id, evaluated_at, total_usd, account_count, stale_account_count, incomplete_account_count,
       boundary_contribution_usd, boundary_withdrawal_usd, boundary_json, issues_json, recalculated_at, created_at
FROM valuation_points
WHERE portfolio_id = sqlc.arg(portfolio_id) AND evaluated_at >= sqlc.arg(from_at) AND evaluated_at <= sqlc.arg(to_at)
ORDER BY evaluated_at ASC;

-- name: CountValuationPoints :one
SELECT COUNT(*) FROM valuation_points WHERE portfolio_id = sqlc.arg(portfolio_id);

-- name: GetValuationPointByJob :one
SELECT id, portfolio_id, job_id, evaluated_at, total_usd, account_count, stale_account_count, incomplete_account_count,
       boundary_contribution_usd, boundary_withdrawal_usd, boundary_json, issues_json, recalculated_at, created_at
FROM valuation_points
WHERE portfolio_id = sqlc.arg(portfolio_id) AND job_id = sqlc.arg(job_id);
