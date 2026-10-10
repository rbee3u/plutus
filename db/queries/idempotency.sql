-- name: LockIdempotencyRecord :one
SELECT request_hash, response_status, response_headers_json, response_body_json, expires_at
FROM idempotency_records
WHERE user_id = ? AND method = ? AND path = ? AND idempotency_key = ?
FOR UPDATE;

-- name: DeleteIdempotencyRecord :exec
DELETE FROM idempotency_records WHERE user_id = ? AND method = ? AND path = ? AND idempotency_key = ?;

-- name: InsertIdempotencyRecord :exec
INSERT INTO idempotency_records (
    user_id, method, path, idempotency_key, request_hash, response_status, response_headers_json, response_body_json, expires_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeleteExpiredIdempotencyRecords :execrows
DELETE FROM idempotency_records WHERE expires_at <= ? LIMIT 1000;
