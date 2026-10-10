-- name: InsertUser :exec
INSERT INTO users (id, username, username_key, status, password_hash, created_at, updated_at)
VALUES (
    sqlc.arg(id), sqlc.arg(username), sqlc.arg(username_key), sqlc.arg(status),
    sqlc.arg(password_hash), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: GetUserByUsernameKey :one
SELECT id, username, username_key, status, password_hash, created_at, updated_at
FROM users
WHERE username_key = sqlc.arg(username_key);

-- name: GetUserByID :one
SELECT id, username, username_key, status, password_hash, created_at, updated_at
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

-- name: LockUser :one
SELECT * FROM users WHERE id = sqlc.arg(id) FOR UPDATE;
