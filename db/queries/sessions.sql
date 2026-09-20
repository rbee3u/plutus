-- name: InsertSession :exec
INSERT INTO sessions (
    id, user_id, token_hash, csrf_secret, created_at, last_seen_at,
    idle_expires_at, absolute_expires_at, revoked_at, client_label
) VALUES (
    sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(token_hash), sqlc.arg(csrf_secret),
    sqlc.arg(created_at), sqlc.arg(last_seen_at), sqlc.arg(idle_expires_at),
    sqlc.arg(absolute_expires_at), sqlc.arg(revoked_at), sqlc.arg(client_label)
);

-- name: GetSessionByTokenHash :one
SELECT id, user_id, token_hash, csrf_secret, created_at, last_seen_at,
       idle_expires_at, absolute_expires_at, revoked_at, client_label
FROM sessions
WHERE token_hash = sqlc.arg(token_hash);

-- name: GetSessionByID :one
SELECT id, user_id, token_hash, csrf_secret, created_at, last_seen_at,
       idle_expires_at, absolute_expires_at, revoked_at, client_label
FROM sessions
WHERE id = sqlc.arg(id);

-- name: ListSessionsByUser :many
SELECT id, user_id, token_hash, csrf_secret, created_at, last_seen_at,
       idle_expires_at, absolute_expires_at, revoked_at, client_label
FROM sessions
WHERE user_id = sqlc.arg(user_id)
ORDER BY created_at DESC, id DESC;

-- name: TouchSession :exec
UPDATE sessions
SET last_seen_at = sqlc.arg(last_seen_at), idle_expires_at = sqlc.arg(idle_expires_at)
WHERE id = sqlc.arg(id);

-- name: RevokeSession :exec
UPDATE sessions
SET revoked_at = sqlc.arg(revoked_at)
WHERE id = sqlc.arg(id) AND revoked_at IS NULL;

-- name: RevokeUserSessions :exec
UPDATE sessions
SET revoked_at = sqlc.arg(revoked_at)
WHERE user_id = sqlc.arg(user_id) AND revoked_at IS NULL;

-- name: RevokeOtherUserSessions :exec
UPDATE sessions
SET revoked_at = sqlc.arg(revoked_at)
WHERE user_id = sqlc.arg(user_id) AND id <> sqlc.arg(keep_id) AND revoked_at IS NULL;
