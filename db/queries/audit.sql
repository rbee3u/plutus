-- name: InsertAuditEvent :exec
INSERT INTO audit_events (
    id, portfolio_id, actor_user_id, action, resource_type, resource_id, request_id,
    before_json, after_json, metadata_json, occurred_at
) VALUES (
    sqlc.arg(id), sqlc.narg(portfolio_id), sqlc.narg(actor_user_id), sqlc.arg(action),
    sqlc.arg(resource_type), sqlc.arg(resource_id), sqlc.arg(request_id),
    sqlc.narg(before_json), sqlc.narg(after_json), sqlc.narg(metadata_json), sqlc.arg(occurred_at)
);
