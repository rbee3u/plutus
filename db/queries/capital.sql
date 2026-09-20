-- name: GetCapitalBaselineByPortfolio :one
SELECT id, portfolio_id, current_revision_id, version, created_at, updated_at
FROM capital_baselines
WHERE portfolio_id = sqlc.arg(portfolio_id);

-- name: InsertCapitalBaseline :exec
INSERT INTO capital_baselines (
    id, portfolio_id, current_revision_id, version, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.narg(current_revision_id), sqlc.arg(version),
    sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: SetCapitalBaselineRevision :exec
UPDATE capital_baselines
SET current_revision_id = sqlc.arg(current_revision_id),
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: UpdateCapitalBaselinePointer :execrows
UPDATE capital_baselines
SET current_revision_id = sqlc.arg(current_revision_id),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version);

-- name: GetCapitalBaselineRevision :one
SELECT id, portfolio_id, capital_baseline_id, effective_at, asset_id, quantity, note, created_by_user_id, created_at
FROM capital_baseline_revisions
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: InsertCapitalBaselineRevision :exec
INSERT INTO capital_baseline_revisions (
    id, portfolio_id, capital_baseline_id, effective_at, asset_id, quantity, note, created_by_user_id, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(capital_baseline_id), sqlc.arg(effective_at),
    sqlc.arg(asset_id), sqlc.arg(quantity), sqlc.narg(note), sqlc.arg(created_by_user_id), sqlc.arg(created_at)
);

-- name: ListCapitalBaselineRevisions :many
SELECT id, portfolio_id, capital_baseline_id, effective_at, asset_id, quantity, note, created_by_user_id, created_at
FROM capital_baseline_revisions
WHERE portfolio_id = sqlc.arg(portfolio_id) AND capital_baseline_id = sqlc.arg(capital_baseline_id)
  AND (
      sqlc.arg(has_cursor) = 0
      OR created_at < sqlc.arg(cursor_created_at)
      OR (created_at = sqlc.arg(cursor_created_at) AND id < sqlc.arg(cursor_id))
  )
ORDER BY created_at DESC, id DESC
LIMIT ?;

-- name: GetCapitalFlow :one
SELECT id, portfolio_id, current_revision_id, version, created_at, updated_at
FROM capital_flows
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: CountActiveCapitalFlows :one
SELECT COUNT(*)
FROM capital_flows f
INNER JOIN capital_flow_revisions r
    ON r.portfolio_id = f.portfolio_id AND r.id = f.current_revision_id
WHERE f.portfolio_id = sqlc.arg(portfolio_id) AND r.state = 'active';

-- name: ListActiveCapitalFlows :many
SELECT
    f.id, f.portfolio_id, f.current_revision_id, f.version, f.created_at, f.updated_at,
    r.direction, r.quantity, r.asset_id, r.effective_at, r.account_id, r.note, r.state, r.id AS revision_id
FROM capital_flows f
INNER JOIN capital_flow_revisions r
    ON r.portfolio_id = f.portfolio_id AND r.id = f.current_revision_id
WHERE f.portfolio_id = sqlc.arg(portfolio_id) AND r.state = 'active'
ORDER BY r.effective_at ASC, f.id ASC
LIMIT ? OFFSET ?;

-- name: InsertCapitalFlow :exec
INSERT INTO capital_flows (
    id, portfolio_id, current_revision_id, version, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.narg(current_revision_id), sqlc.arg(version),
    sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: SetCapitalFlowRevision :exec
UPDATE capital_flows
SET current_revision_id = sqlc.arg(current_revision_id),
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: UpdateCapitalFlowPointer :execrows
UPDATE capital_flows
SET current_revision_id = sqlc.arg(current_revision_id),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version);

-- name: GetCapitalFlowRevision :one
SELECT id, portfolio_id, capital_flow_id, direction, quantity, asset_id, effective_at, account_id, note, state,
       created_by_user_id, created_at
FROM capital_flow_revisions
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: InsertCapitalFlowRevision :exec
INSERT INTO capital_flow_revisions (
    id, portfolio_id, capital_flow_id, direction, quantity, asset_id, effective_at, account_id, note, state,
    created_by_user_id, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(capital_flow_id), sqlc.arg(direction), sqlc.arg(quantity),
    sqlc.arg(asset_id), sqlc.arg(effective_at), sqlc.narg(account_id), sqlc.narg(note), sqlc.arg(state),
    sqlc.arg(created_by_user_id), sqlc.arg(created_at)
);

-- name: ListCapitalFlowRevisions :many
SELECT id, portfolio_id, capital_flow_id, direction, quantity, asset_id, effective_at, account_id, note, state,
       created_by_user_id, created_at
FROM capital_flow_revisions
WHERE portfolio_id = sqlc.arg(portfolio_id) AND capital_flow_id = sqlc.arg(capital_flow_id)
  AND (
      sqlc.arg(has_cursor) = 0
      OR created_at < sqlc.arg(cursor_created_at)
      OR (created_at = sqlc.arg(cursor_created_at) AND id < sqlc.arg(cursor_id))
  )
ORDER BY created_at DESC, id DESC
LIMIT ?;
