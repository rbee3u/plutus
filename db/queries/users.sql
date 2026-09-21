-- name: InsertUser :exec
INSERT INTO users (id, username, username_normalized, status, password_hash, created_at, updated_at)
VALUES (
    sqlc.arg(id), sqlc.arg(username), sqlc.arg(username_normalized), sqlc.arg(status),
    sqlc.arg(password_hash), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: GetUserByNormalizedUsername :one
SELECT id, username, username_normalized, status, created_at, updated_at, password_hash
FROM users
WHERE username_normalized = sqlc.arg(username_normalized);

-- name: GetUserByID :one
SELECT id, username, username_normalized, status, created_at, updated_at, password_hash
FROM users
WHERE id = sqlc.arg(id);

-- name: SetUserStatus :exec
UPDATE users
SET status = sqlc.arg(status), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: UpdateUserPasswordHash :exec
UPDATE users
SET password_hash = sqlc.arg(password_hash), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);
