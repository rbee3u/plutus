-- name: InsertIdentity :exec
INSERT INTO identities (
    id, portfolio_id, platform, display_name, account_mode, external_ref, status, archived_at,
    connection_status, credentials_ciphertext, credentials_nonce, credential_key_version,
    credentials_updated_at, last_validated_at, last_collected_at, last_error_class, last_error_detail,
    version, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(platform), sqlc.arg(display_name), sqlc.arg(account_mode),
    sqlc.narg(external_ref), sqlc.arg(status), NULL,
    'unconfigured', NULL, NULL, 0,
    NULL, NULL, NULL, NULL, NULL,
    sqlc.arg(version), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: ListIdentitiesByPortfolio :many
SELECT id, portfolio_id, platform, display_name, account_mode, external_ref, status, archived_at,
       connection_status, credentials_ciphertext IS NOT NULL AS credentials_configured, credentials_updated_at,
       last_validated_at, last_collected_at, last_error_class, last_error_detail,
       version, created_at, updated_at
FROM identities
WHERE portfolio_id = sqlc.arg(portfolio_id)
ORDER BY platform ASC, created_at ASC, id ASC;

-- name: GetIdentity :one
SELECT id, portfolio_id, platform, display_name, account_mode, external_ref, status, archived_at,
       connection_status, credentials_ciphertext IS NOT NULL AS credentials_configured, credentials_updated_at,
       last_validated_at, last_collected_at, last_error_class, last_error_detail,
       version, created_at, updated_at
FROM identities
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: GetIdentityCredentials :one
SELECT id, portfolio_id, platform, account_mode, connection_status,
       credentials_ciphertext, credentials_nonce, credential_key_version
FROM identities
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: ListCollectableIdentities :many
SELECT id, portfolio_id, platform, display_name, account_mode, external_ref,
       credentials_ciphertext, credentials_nonce, credential_key_version
FROM identities
WHERE portfolio_id = sqlc.arg(portfolio_id) AND status = 'enabled' AND connection_status = 'enabled'
ORDER BY platform ASC, created_at ASC, id ASC;

-- name: UpdateIdentity :execrows
UPDATE identities
SET display_name = sqlc.arg(display_name),
    account_mode = sqlc.arg(account_mode),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version) AND status <> 'archived';

-- name: SetIdentityCredentials :execrows
UPDATE identities
SET credentials_ciphertext = sqlc.arg(credentials_ciphertext),
    credentials_nonce = sqlc.arg(credentials_nonce),
    credential_key_version = sqlc.arg(credential_key_version),
    credentials_updated_at = sqlc.arg(credentials_updated_at),
    connection_status = 'disabled',
    last_validated_at = NULL,
    last_error_class = NULL,
    last_error_detail = NULL,
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version) AND status <> 'archived';

-- name: ClearIdentityCredentials :execrows
UPDATE identities
SET credentials_ciphertext = NULL,
    credentials_nonce = NULL,
    credential_key_version = 0,
    credentials_updated_at = NULL,
    connection_status = 'unconfigured',
    last_validated_at = NULL,
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version) AND status <> 'archived';

-- name: SetIdentityConnectionStatus :execrows
UPDATE identities
SET connection_status = sqlc.arg(connection_status),
    external_ref = COALESCE(sqlc.narg(external_ref), external_ref),
    last_validated_at = COALESCE(sqlc.narg(last_validated_at), last_validated_at),
    last_error_class = sqlc.narg(last_error_class),
    last_error_detail = sqlc.narg(last_error_detail),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version) AND status <> 'archived';

-- name: RecordIdentityCollection :exec
UPDATE identities
SET last_collected_at = sqlc.arg(last_collected_at),
    last_error_class = sqlc.narg(last_error_class),
    last_error_detail = sqlc.narg(last_error_detail),
    connection_status = IF(sqlc.arg(invalidate) = 1, 'invalid', connection_status),
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: ArchiveIdentity :execrows
UPDATE identities
SET status = 'archived',
    archived_at = sqlc.arg(archived_at),
    connection_status = IF(connection_status = 'enabled', 'disabled', connection_status),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version) AND status <> 'archived';

-- name: CountIdentitiesWithCiphertext :one
SELECT COUNT(*) FROM identities WHERE credentials_ciphertext IS NOT NULL;

-- name: CountReportsForIdentity :one
SELECT COUNT(*)
FROM account_reports r
INNER JOIN accounts a ON a.portfolio_id = r.portfolio_id AND a.id = r.account_id
WHERE r.portfolio_id = sqlc.arg(portfolio_id) AND a.identity_id = sqlc.arg(identity_id);
