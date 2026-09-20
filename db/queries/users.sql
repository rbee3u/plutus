-- name: InsertUser :exec
INSERT INTO users (id, username, username_normalized, status, created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(username), sqlc.arg(username_normalized), sqlc.arg(status), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: GetUserByNormalizedUsername :one
SELECT id, username, username_normalized, status, created_at, updated_at
FROM users
WHERE username_normalized = sqlc.arg(username_normalized);

-- name: GetUserByID :one
SELECT id, username, username_normalized, status, created_at, updated_at
FROM users
WHERE id = sqlc.arg(id);

-- name: SetUserStatus :exec
UPDATE users
SET status = sqlc.arg(status), updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: InsertUserCredential :exec
INSERT INTO user_credentials (user_id, password_hash, password_changed_at, created_at, updated_at)
VALUES (sqlc.arg(user_id), sqlc.arg(password_hash), sqlc.arg(password_changed_at), sqlc.arg(created_at), sqlc.arg(updated_at));

-- name: GetUserCredential :one
SELECT user_id, password_hash, password_changed_at, created_at, updated_at
FROM user_credentials
WHERE user_id = sqlc.arg(user_id);

-- name: UpdateUserCredential :exec
UPDATE user_credentials
SET password_hash = sqlc.arg(password_hash), password_changed_at = sqlc.arg(password_changed_at), updated_at = sqlc.arg(updated_at)
WHERE user_id = sqlc.arg(user_id);
