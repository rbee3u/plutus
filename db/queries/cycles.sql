-- name: InsertCollectionCycle :exec
INSERT INTO collection_cycles (portfolio_id, id, trigger_kind, status, available_at, created_at)
VALUES (sqlc.arg(portfolio_id), sqlc.arg(id), sqlc.arg(trigger_kind), 'queued', sqlc.arg(created_at), sqlc.arg(created_at));

-- name: GetCollectionCycle :one
SELECT * FROM collection_cycles WHERE portfolio_id = ? AND id = ?;

-- name: LockActiveCollectionCycle :one
SELECT * FROM collection_cycles WHERE active_portfolio_id = ? FOR UPDATE;

-- name: ListSucceededCycles :many
SELECT id, portfolio_id, created_at, collection_finished_at, finished_at
FROM collection_cycles
WHERE portfolio_id = sqlc.arg(portfolio_id) AND status = 'succeeded'
ORDER BY created_at ASC, id ASC;

-- Only active cycles have a non-NULL active_portfolio_id, so the unique key
-- bounds the scan by the number of Portfolios.
-- name: LockClaimableCollectionCycle :one
SELECT * FROM collection_cycles
WHERE active_portfolio_id IS NOT NULL
  AND ((status = 'queued' AND available_at <= sqlc.arg(queued_before))
       OR (status = 'running' AND lease_expires_at < sqlc.arg(running_before)))
ORDER BY available_at ASC, portfolio_id ASC, id ASC
LIMIT 1
FOR UPDATE SKIP LOCKED;

-- name: MarkCollectionCycleRunning :execrows
UPDATE collection_cycles
SET status = 'running',
    attempt_count = sqlc.arg(attempt_count),
    lease_owner = sqlc.arg(lease_owner),
    lease_expires_at = sqlc.arg(lease_expires_at),
    started_at = COALESCE(started_at, sqlc.arg(started_at))
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND status IN ('queued', 'running');

-- name: RenewCollectionCycleLease :execrows
UPDATE collection_cycles
SET lease_expires_at = sqlc.arg(lease_expires_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id)
  AND status = 'running' AND lease_owner = sqlc.arg(lease_owner) AND lease_expires_at > sqlc.arg(lease_at);

-- name: LockRunningCollectionCycle :one
SELECT id FROM collection_cycles
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id)
  AND status = 'running' AND lease_owner = sqlc.arg(lease_owner) AND lease_expires_at > sqlc.arg(lease_at)
FOR UPDATE;

-- name: MarkCollectionCycleCollected :exec
UPDATE collection_cycles SET collection_finished_at = COALESCE(collection_finished_at, sqlc.arg(at))
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: MarkCollectionCycleSucceeded :execrows
UPDATE collection_cycles
SET status = 'succeeded',
    lease_owner = NULL,
    lease_expires_at = NULL,
    finished_at = sqlc.arg(finished_at),
    error_class = NULL,
    error_detail = NULL
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id)
  AND status = 'running' AND lease_owner = sqlc.arg(lease_owner) AND lease_expires_at > sqlc.arg(lease_at);

-- name: RequeueCollectionCycle :execrows
UPDATE collection_cycles
SET status = 'queued',
    lease_owner = NULL,
    lease_expires_at = NULL,
    available_at = sqlc.arg(available_at),
    error_class = sqlc.narg(error_class),
    error_detail = sqlc.narg(error_detail)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id)
  AND status = 'running' AND lease_owner = sqlc.arg(lease_owner) AND lease_expires_at > sqlc.arg(lease_at);

-- name: MarkCollectionCycleFailed :execrows
UPDATE collection_cycles
SET status = 'failed',
    lease_owner = NULL,
    lease_expires_at = NULL,
    finished_at = sqlc.arg(finished_at),
    error_class = sqlc.narg(error_class),
    error_detail = sqlc.narg(error_detail)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id)
  AND status = 'running' AND lease_owner = sqlc.arg(lease_owner) AND lease_expires_at > sqlc.arg(lease_at);

-- Ends the active cycle without a lease check: retry exhaustion after a lost
-- worker, and Portfolio deletion requests.
-- name: FailActiveCollectionCycle :execrows
UPDATE collection_cycles
SET status = 'failed',
    lease_owner = NULL,
    lease_expires_at = NULL,
    finished_at = sqlc.arg(finished_at),
    error_class = sqlc.narg(error_class),
    error_detail = sqlc.narg(error_detail)
WHERE active_portfolio_id = sqlc.arg(portfolio_id);

-- name: ListCyclesWithPoints :many
SELECT c.id, c.portfolio_id, c.status, c.trigger_kind, c.attempt_count, c.error_class, c.error_detail,
       c.created_at, c.started_at, c.collection_finished_at, c.finished_at,
       p.cycle_id AS point_cycle_id, p.evaluated_at, p.total_usd, p.account_count,
       p.stale_account_count, p.incomplete_account_count, p.detail_state, p.calculation_version
FROM collection_cycles c
LEFT JOIN valuation_points p ON p.portfolio_id = c.portfolio_id AND p.cycle_id = c.id
WHERE c.portfolio_id = sqlc.arg(portfolio_id)
  AND (sqlc.arg(has_cursor) = 0 OR c.created_at < sqlc.arg(cursor_created_at)
       OR (c.created_at = sqlc.arg(cursor_created_at) AND c.id < sqlc.arg(cursor_id)))
ORDER BY c.created_at DESC, c.id DESC LIMIT ?;
