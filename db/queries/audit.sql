-- name: ListAuditEventsByPortfolio :many
SELECT id, portfolio_id, actor_user_id, action, resource_type, resource_id, request_id,
       before_json, after_json, metadata_json, occurred_at
FROM audit_events
WHERE portfolio_id = sqlc.arg(portfolio_id)
  AND (sqlc.arg(filter_action) = 0 OR action IN (sqlc.arg(action), sqlc.arg(legacy_action), sqlc.arg(platform_asset_action)))
  AND (sqlc.arg(filter_resource_type) = 0 OR resource_type IN (sqlc.arg(resource_type), sqlc.arg(legacy_resource_type), sqlc.arg(platform_asset_resource_type)))
  AND (
      sqlc.arg(has_cursor) = 0
      OR occurred_at < sqlc.arg(cursor_occurred_at)
      OR (occurred_at = sqlc.arg(cursor_occurred_at) AND id < sqlc.arg(cursor_id))
  )
ORDER BY occurred_at DESC, id DESC
LIMIT ?;

-- name: InsertAuditEvent :exec
INSERT INTO audit_events (
    id, portfolio_id, actor_user_id, action, resource_type, resource_id, request_id,
    before_json, after_json, metadata_json, occurred_at
) VALUES (
    sqlc.arg(id), sqlc.narg(portfolio_id), sqlc.narg(actor_user_id), sqlc.arg(action),
    sqlc.arg(resource_type), sqlc.arg(resource_id), sqlc.arg(request_id),
    sqlc.narg(before_json), sqlc.narg(after_json), sqlc.narg(metadata_json), sqlc.arg(occurred_at)
);
