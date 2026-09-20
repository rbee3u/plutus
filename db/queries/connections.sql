-- name: InsertConnection :exec
INSERT INTO connections (
    id, portfolio_id, identity_id, label, status, credentials_ciphertext, credentials_nonce,
    credential_key_version, config_json, collection_interval_seconds, freshness_after_seconds,
    last_validated_at, version, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(identity_id), sqlc.arg(label), sqlc.arg(status),
    sqlc.narg(credentials_ciphertext), sqlc.narg(credentials_nonce), sqlc.arg(credential_key_version),
    sqlc.arg(config_json), sqlc.arg(collection_interval_seconds), sqlc.arg(freshness_after_seconds),
    sqlc.arg(last_validated_at), sqlc.arg(version), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: ListConnectionsByPortfolio :many
SELECT id, portfolio_id, identity_id, label, status, credentials_ciphertext, credentials_nonce,
       credential_key_version, config_json, collection_interval_seconds, freshness_after_seconds,
       last_validated_at, version, created_at, updated_at
FROM connections
WHERE portfolio_id = sqlc.arg(portfolio_id)
ORDER BY created_at ASC, id ASC;

-- name: GetConnection :one
SELECT id, portfolio_id, identity_id, label, status, credentials_ciphertext, credentials_nonce,
       credential_key_version, config_json, collection_interval_seconds, freshness_after_seconds,
       last_validated_at, version, created_at, updated_at
FROM connections
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: UpdateConnection :execrows
UPDATE connections
SET label = sqlc.arg(label),
    status = sqlc.arg(status),
    collection_interval_seconds = sqlc.arg(collection_interval_seconds),
    freshness_after_seconds = sqlc.arg(freshness_after_seconds),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version);

-- name: DeleteConnection :execrows
DELETE FROM connections
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version);
