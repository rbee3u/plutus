-- name: ListAuditEventsByPortfolio :many
SELECT portfolio_id, id, actor_user_id, action, resource_type, resource_id, request_id,
       before_json, after_json, metadata_json, occurred_at
FROM audit_events
WHERE portfolio_id = sqlc.arg(portfolio_id)
  AND (sqlc.arg(filter_action) = 0 OR action = sqlc.arg(action))
  AND (sqlc.arg(filter_resource_type) = 0 OR resource_type = sqlc.arg(resource_type))
  AND (
      sqlc.arg(has_cursor) = 0
      OR occurred_at < sqlc.arg(cursor_occurred_at)
      OR (occurred_at = sqlc.arg(cursor_occurred_at) AND id < sqlc.arg(cursor_id))
  )
ORDER BY occurred_at DESC, id DESC
LIMIT ?;

-- name: InsertAuditEvent :exec
INSERT INTO audit_events (
    portfolio_id, id, actor_user_id, action, resource_type, resource_id, request_id,
    before_json, after_json, metadata_json, occurred_at
) VALUES (
    sqlc.arg(portfolio_id), sqlc.arg(id), sqlc.narg(actor_user_id), sqlc.arg(action),
    sqlc.arg(resource_type), sqlc.arg(resource_id), sqlc.arg(request_id),
    sqlc.narg(before_json), sqlc.narg(after_json), sqlc.narg(metadata_json), sqlc.arg(occurred_at)
);

-- Audit pages resolve only their referenced immutable rules, never today's rule.
-- Version zero denotes the implicit initial, non-excluded state.
-- name: ListAuditExclusionVersions :many
SELECT e.id AS event_id, v.version, v.excluded, v.reason, v.effective_at
FROM audit_events e
JOIN asset_exclusion_versions v ON v.portfolio_id = e.portfolio_id AND v.asset_id = e.resource_id
  AND (v.version = CAST(JSON_UNQUOTE(JSON_EXTRACT(e.before_json, '$.version')) AS UNSIGNED)
    OR v.version = CAST(JSON_UNQUOTE(JSON_EXTRACT(e.after_json, '$.version')) AS UNSIGNED))
WHERE e.portfolio_id = sqlc.arg(portfolio_id) AND e.id IN (sqlc.slice('event_ids'))
  AND e.resource_type = 'asset' AND e.action IN ('catalog.asset.exclude', 'catalog.asset.restore');
