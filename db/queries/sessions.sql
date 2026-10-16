-- name: InsertSession :exec
INSERT INTO sessions (
    id, user_id, token_hash, csrf_secret, created_at, last_seen_at,
    idle_expires_at, absolute_expires_at, revoked_at, user_agent
) VALUES (
    sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(token_hash), sqlc.arg(csrf_secret),
    sqlc.arg(created_at), sqlc.arg(last_seen_at), sqlc.arg(idle_expires_at),
    sqlc.arg(absolute_expires_at), sqlc.arg(revoked_at), sqlc.arg(user_agent)
);

-- name: GetSessionByTokenHash :one
SELECT sqlc.embed(s), sqlc.embed(u)
FROM sessions s JOIN users u ON u.id = s.user_id
WHERE s.token_hash = sqlc.arg(token_hash);

-- name: GetSessionByID :one
SELECT id, user_id, token_hash, csrf_secret, created_at, last_seen_at,
       idle_expires_at, absolute_expires_at, revoked_at, user_agent
FROM sessions
WHERE id = sqlc.arg(id);

-- name: ListSessionsByUser :many
SELECT id, user_id, token_hash, csrf_secret, created_at, last_seen_at,
       idle_expires_at, absolute_expires_at, revoked_at, user_agent
FROM sessions
WHERE user_id = sqlc.arg(user_id) AND revoked_at IS NULL
  AND idle_expires_at >= sqlc.arg(now) AND absolute_expires_at >= sqlc.arg(now)
ORDER BY created_at DESC, id DESC;

-- name: CountActiveSessionsByUser :one
SELECT COUNT(*) FROM sessions
WHERE user_id = sqlc.arg(user_id) AND revoked_at IS NULL
  AND idle_expires_at >= sqlc.arg(now) AND absolute_expires_at >= sqlc.arg(now);

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

-- name: PruneExpiredSessions :execrows
DELETE FROM sessions
WHERE revoked_at IS NOT NULL OR idle_expires_at <= sqlc.arg(now) OR absolute_expires_at <= sqlc.arg(now)
LIMIT 1000;
