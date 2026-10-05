-- name: InsertJob :exec
INSERT INTO jobs (
    id, portfolio_id, kind, status, `trigger`, dedupe_key, payload_json, requested_by_user_id,
    available_at, attempt_count, max_attempts, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(kind), 'queued', sqlc.arg(trigger_kind), sqlc.arg(dedupe_key),
    sqlc.narg(payload_json), sqlc.narg(requested_by_user_id), sqlc.arg(available_at), 0, sqlc.arg(max_attempts),
    sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: GetActiveJobByDedupe :one
SELECT id, portfolio_id, kind, status, `trigger`, dedupe_key, payload_json, requested_by_user_id,
       available_at, lease_owner, lease_expires_at, attempt_count, max_attempts,
       last_error_class, last_detail, created_at, updated_at, started_at, finished_at
FROM jobs
WHERE portfolio_id = sqlc.arg(portfolio_id) AND active_dedupe_key = sqlc.arg(dedupe_key);

-- name: GetJob :one
SELECT id, portfolio_id, kind, status, `trigger`, dedupe_key, payload_json, requested_by_user_id,
       available_at, lease_owner, lease_expires_at, attempt_count, max_attempts,
       last_error_class, last_detail, created_at, updated_at, started_at, finished_at
FROM jobs
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: ListJobsByPortfolio :many
SELECT id, portfolio_id, kind, status, `trigger`, dedupe_key, payload_json, requested_by_user_id,
       available_at, lease_owner, lease_expires_at, attempt_count, max_attempts,
       last_error_class, last_detail, created_at, updated_at, started_at, finished_at
FROM jobs
WHERE portfolio_id = sqlc.arg(portfolio_id)
  AND (sqlc.arg(filter_kind) = 0 OR kind = sqlc.arg(kind))
  AND (
      sqlc.arg(has_cursor) = 0
      OR created_at < sqlc.arg(cursor_created_at)
      OR (created_at = sqlc.arg(cursor_created_at) AND id < sqlc.arg(cursor_id))
  )
ORDER BY created_at DESC, id DESC
LIMIT ?;

-- name: ListSucceededCycleJobs :many
SELECT id, portfolio_id, created_at, finished_at
FROM jobs
WHERE portfolio_id = sqlc.arg(portfolio_id) AND kind = 'collection_cycle' AND status = 'succeeded'
ORDER BY created_at ASC, id ASC;

-- name: LockClaimableJob :one
SELECT id, portfolio_id, kind, status, `trigger`, dedupe_key, payload_json, requested_by_user_id,
       available_at, lease_owner, lease_expires_at, attempt_count, max_attempts,
       last_error_class, last_detail, created_at, updated_at, started_at, finished_at
FROM jobs
WHERE (status = 'queued' AND available_at <= sqlc.arg(queued_before))
   OR (status = 'running' AND lease_expires_at IS NOT NULL AND lease_expires_at < sqlc.arg(running_before))
ORDER BY available_at ASC, id ASC
LIMIT 1
FOR UPDATE SKIP LOCKED;

-- name: MarkJobRunning :execrows
UPDATE jobs
SET status = 'running',
    attempt_count = sqlc.arg(attempt_count),
    lease_owner = sqlc.arg(lease_owner),
    lease_expires_at = sqlc.arg(lease_expires_at),
    started_at = sqlc.arg(started_at),
    updated_at = sqlc.arg(updated_at),
    finished_at = NULL
WHERE id = sqlc.arg(id) AND portfolio_id = sqlc.arg(portfolio_id);

-- name: RenewJobLease :execrows
UPDATE jobs
SET lease_expires_at = sqlc.arg(lease_expires_at),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND portfolio_id = sqlc.arg(portfolio_id) AND lease_owner = sqlc.arg(lease_owner) AND status = 'running';

-- name: MarkJobSucceeded :execrows
UPDATE jobs
SET status = 'succeeded',
    lease_owner = NULL,
    lease_expires_at = NULL,
    last_error_class = NULL,
    last_detail = sqlc.narg(last_detail),
    finished_at = sqlc.arg(finished_at),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND portfolio_id = sqlc.arg(portfolio_id) AND lease_owner <=> sqlc.arg(lease_owner);

-- name: RequeueJob :execrows
UPDATE jobs
SET status = 'queued',
    lease_owner = NULL,
    lease_expires_at = NULL,
    available_at = sqlc.arg(available_at),
    last_error_class = sqlc.arg(last_error_class),
    last_detail = sqlc.arg(last_detail),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND portfolio_id = sqlc.arg(portfolio_id) AND lease_owner <=> sqlc.arg(lease_owner);

-- name: MarkJobFailed :execrows
UPDATE jobs
SET status = 'failed',
    lease_owner = NULL,
    lease_expires_at = NULL,
    last_error_class = sqlc.arg(last_error_class),
    last_detail = sqlc.arg(last_detail),
    finished_at = sqlc.arg(finished_at),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND portfolio_id = sqlc.arg(portfolio_id) AND lease_owner <=> sqlc.arg(lease_owner);

-- name: ListCycleJobsWithPoints :many
SELECT j.id, j.portfolio_id, j.status, j.`trigger`, j.attempt_count, j.max_attempts, j.last_error_class, j.last_detail,
       j.created_at, j.started_at, j.finished_at,
       p.id AS point_id, p.evaluated_at, p.total_usd, p.account_count, p.stale_account_count, p.incomplete_account_count,
       p.boundary_contribution_usd, p.boundary_withdrawal_usd, p.recalculated_at
FROM jobs j
LEFT JOIN valuation_points p ON p.portfolio_id = j.portfolio_id AND p.job_id = j.id
WHERE j.portfolio_id = sqlc.arg(portfolio_id) AND j.kind = 'collection_cycle'
  AND (
      sqlc.arg(has_cursor) = 0
      OR j.created_at < sqlc.arg(cursor_created_at)
      OR (j.created_at = sqlc.arg(cursor_created_at) AND j.id < sqlc.arg(cursor_id))
  )
ORDER BY j.created_at DESC, j.id DESC
LIMIT ?;

-- name: DeleteFinishedJobsBefore :execrows
DELETE j FROM jobs j
WHERE j.status IN ('succeeded', 'failed', 'cancelled')
  AND j.finished_at IS NOT NULL AND j.finished_at < sqlc.arg(before)
  AND NOT EXISTS (SELECT 1 FROM account_reports r WHERE r.portfolio_id = j.portfolio_id AND r.job_id = j.id)
  AND NOT EXISTS (SELECT 1 FROM valuation_points p WHERE p.portfolio_id = j.portfolio_id AND p.job_id = j.id)
  AND NOT EXISTS (SELECT 1 FROM manual_cycles m WHERE m.portfolio_id = j.portfolio_id AND m.job_id = j.id);
