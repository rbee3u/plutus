-- name: InsertJob :exec
INSERT INTO jobs (
    id, portfolio_id, kind, status, priority, dedupe_key, requested_by_user_id,
    available_at, lease_owner, lease_expires_at, attempt_count, max_attempts,
    last_error_code, last_error_detail, created_at, updated_at, started_at, finished_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(kind), sqlc.arg(status), sqlc.arg(priority),
    sqlc.arg(dedupe_key), sqlc.narg(requested_by_user_id), sqlc.arg(available_at),
    sqlc.narg(lease_owner), sqlc.narg(lease_expires_at), sqlc.arg(attempt_count),
    sqlc.arg(max_attempts), sqlc.narg(last_error_code), sqlc.narg(last_error_detail),
    sqlc.arg(created_at), sqlc.arg(updated_at), sqlc.narg(started_at), sqlc.narg(finished_at)
);

-- name: InsertProjectionJobDetail :exec
INSERT INTO projection_job_details (
    portfolio_id, job_id, invalid_from, invalid_through, reason
) VALUES (
    sqlc.arg(portfolio_id), sqlc.arg(job_id), sqlc.narg(invalid_from), sqlc.narg(invalid_through), sqlc.arg(reason)
);

-- name: InsertProjectionJobKind :exec
INSERT INTO projection_job_kinds (
    portfolio_id, job_id, projection_kind
) VALUES (
    sqlc.arg(portfolio_id), sqlc.arg(job_id), sqlc.arg(projection_kind)
);

-- name: GetActiveJobByDedupe :one
SELECT
    id, portfolio_id, kind, status, priority, dedupe_key, requested_by_user_id,
    available_at, lease_owner, lease_expires_at, attempt_count, max_attempts,
    last_error_code, last_error_detail, created_at, updated_at, started_at, finished_at
FROM jobs
WHERE portfolio_id = sqlc.arg(portfolio_id) AND active_dedupe_key = sqlc.arg(dedupe_key);

-- name: GetJob :one
SELECT
    id, portfolio_id, kind, status, priority, dedupe_key, requested_by_user_id,
    available_at, lease_owner, lease_expires_at, attempt_count, max_attempts,
    last_error_code, last_error_detail, created_at, updated_at, started_at, finished_at
FROM jobs
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: ListJobsByPortfolio :many
SELECT
    id, portfolio_id, kind, status, priority, dedupe_key, requested_by_user_id,
    available_at, lease_owner, lease_expires_at, attempt_count, max_attempts,
    last_error_code, last_error_detail, created_at, updated_at, started_at, finished_at
FROM jobs
WHERE portfolio_id = sqlc.arg(portfolio_id)
  AND (
      sqlc.arg(has_cursor) = 0
      OR created_at < sqlc.arg(cursor_created_at)
      OR (created_at = sqlc.arg(cursor_created_at) AND id < sqlc.arg(cursor_id))
  )
ORDER BY created_at DESC, id DESC
LIMIT ?;

-- name: ListJobAttempts :many
SELECT
    id, portfolio_id, job_id, attempt_number, worker_id, started_at, finished_at,
    outcome, error_class, error_code, safe_detail, retry_at
FROM job_attempts
WHERE portfolio_id = sqlc.arg(portfolio_id) AND job_id = sqlc.arg(job_id)
ORDER BY attempt_number ASC;

-- name: LockClaimableJob :one
SELECT
    id, portfolio_id, kind, status, priority, dedupe_key, requested_by_user_id,
    available_at, lease_owner, lease_expires_at, attempt_count, max_attempts,
    last_error_code, last_error_detail, created_at, updated_at, started_at, finished_at
FROM jobs
WHERE (status = 'queued' AND available_at <= sqlc.arg(queued_before))
   OR (status = 'running' AND lease_expires_at IS NOT NULL AND lease_expires_at < sqlc.arg(running_before))
ORDER BY priority DESC, available_at ASC, id ASC
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
WHERE id = sqlc.arg(id)
  AND portfolio_id = sqlc.arg(portfolio_id)
  AND lease_owner = sqlc.arg(lease_owner)
  AND status = 'running';

-- name: MarkJobSucceeded :execrows
UPDATE jobs
SET status = 'succeeded',
    lease_owner = NULL,
    lease_expires_at = NULL,
    last_error_code = NULL,
    last_error_detail = NULL,
    finished_at = sqlc.arg(finished_at),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND portfolio_id = sqlc.arg(portfolio_id) AND lease_owner <=> sqlc.arg(lease_owner);

-- name: RequeueJob :execrows
UPDATE jobs
SET status = 'queued',
    lease_owner = NULL,
    lease_expires_at = NULL,
    available_at = sqlc.arg(available_at),
    last_error_code = sqlc.arg(last_error_code),
    last_error_detail = sqlc.arg(last_error_detail),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND portfolio_id = sqlc.arg(portfolio_id) AND lease_owner <=> sqlc.arg(lease_owner);

-- name: MarkJobFailed :execrows
UPDATE jobs
SET status = 'failed',
    lease_owner = NULL,
    lease_expires_at = NULL,
    last_error_code = sqlc.arg(last_error_code),
    last_error_detail = sqlc.arg(last_error_detail),
    finished_at = sqlc.arg(finished_at),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND portfolio_id = sqlc.arg(portfolio_id) AND lease_owner <=> sqlc.arg(lease_owner);

-- name: InsertJobAttempt :exec
INSERT INTO job_attempts (
    id, portfolio_id, job_id, attempt_number, worker_id, started_at, finished_at,
    outcome, error_class, error_code, safe_detail, retry_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(job_id), sqlc.arg(attempt_number),
    sqlc.arg(worker_id), sqlc.arg(started_at), sqlc.arg(finished_at), sqlc.arg(outcome),
    sqlc.narg(error_class), sqlc.narg(error_code), sqlc.narg(safe_detail), sqlc.narg(retry_at)
);

-- name: GetPortfolioGenerationPointer :one
SELECT active_projection_generation_id
FROM portfolios
WHERE id = sqlc.arg(id);
