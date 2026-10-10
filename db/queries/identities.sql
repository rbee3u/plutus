-- name: InsertIdentity :exec
INSERT INTO identities (
    id, portfolio_id, platform, display_name, display_name_key, account_structure, last_observed_margin_mode, platform_login_ref, archived_at,
    connection_status, credentials_ciphertext, credentials_nonce, credentials_key_version,
    credentials_set_at, last_validation_succeeded_at, last_collection_finished_at, last_error_class, last_error_detail,
    version, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(platform), sqlc.arg(display_name), sqlc.arg(display_name_key), sqlc.arg(account_structure),
    NULL, sqlc.narg(platform_login_ref), NULL,
    'unconfigured', NULL, NULL, NULL,
    NULL, NULL, NULL, NULL, NULL,
    sqlc.arg(version), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: ListIdentitiesByPortfolio :many
SELECT id, portfolio_id, platform, display_name, display_name_key, account_structure, last_observed_margin_mode, platform_login_ref, archived_at,
       connection_status, credentials_ciphertext IS NOT NULL AS credentials_configured, credentials_set_at,
       last_validation_succeeded_at, last_collection_finished_at, last_error_class, last_error_detail,
       version, created_at, updated_at
FROM identities
WHERE portfolio_id = sqlc.arg(portfolio_id)
ORDER BY id;

-- name: GetIdentity :one
SELECT id, portfolio_id, platform, display_name, display_name_key, account_structure, last_observed_margin_mode, platform_login_ref, archived_at,
       connection_status, credentials_ciphertext IS NOT NULL AS credentials_configured, credentials_set_at,
       last_validation_succeeded_at, last_collection_finished_at, last_error_class, last_error_detail,
       version, created_at, updated_at
FROM identities
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: GetIdentityCredentials :one
SELECT id, portfolio_id, platform, account_structure, connection_status,
       credentials_ciphertext, credentials_nonce, credentials_key_version
FROM identities
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: ListCollectableIdentities :many
SELECT id, portfolio_id, platform, display_name, display_name_key, account_structure, last_observed_margin_mode, platform_login_ref,
       credentials_ciphertext, credentials_nonce, credentials_key_version, version
FROM identities
WHERE portfolio_id = sqlc.arg(portfolio_id) AND archived_at IS NULL AND connection_status = 'enabled'
ORDER BY id;

-- name: UpdateIdentity :execrows
UPDATE identities
SET display_name = sqlc.arg(display_name),
    display_name_key = sqlc.arg(display_name_key),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version) AND archived_at IS NULL;

-- name: SetIdentityCredentials :execrows
UPDATE identities
SET credentials_ciphertext = sqlc.arg(credentials_ciphertext),
    credentials_nonce = sqlc.arg(credentials_nonce),
    credentials_key_version = sqlc.arg(credentials_key_version),
    credentials_set_at = sqlc.arg(credentials_set_at),
    connection_status = 'disabled',
    last_validation_succeeded_at = NULL,
    last_error_class = NULL,
    last_error_detail = NULL,
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version) AND archived_at IS NULL;

-- name: ClearIdentityCredentials :execrows
UPDATE identities
SET credentials_ciphertext = NULL,
    credentials_nonce = NULL,
    credentials_key_version = NULL,
    credentials_set_at = NULL,
    connection_status = 'unconfigured',
    last_validation_succeeded_at = NULL,
    last_error_class = NULL,
    last_error_detail = NULL,
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version) AND archived_at IS NULL;

-- name: SetIdentityConnectionStatus :execrows
UPDATE identities
SET connection_status = sqlc.arg(connection_status),
    platform_login_ref = COALESCE(sqlc.narg(platform_login_ref), platform_login_ref),
    last_observed_margin_mode = COALESCE(sqlc.narg(last_observed_margin_mode), last_observed_margin_mode),
    last_validation_succeeded_at = COALESCE(sqlc.narg(last_validation_succeeded_at), last_validation_succeeded_at),
    last_error_class = sqlc.narg(last_error_class),
    last_error_detail = sqlc.narg(last_error_detail),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version) AND archived_at IS NULL;

-- name: RecordIdentityCollection :exec
UPDATE identities
SET last_collection_finished_at = sqlc.arg(last_collection_finished_at),
    last_error_class = sqlc.narg(last_error_class),
    last_error_detail = sqlc.narg(last_error_detail),
    connection_status = IF(sqlc.arg(invalidate) = 1, 'invalid', connection_status),
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version) AND archived_at IS NULL;

-- name: GetIdentityMarginModeForUpdate :one
SELECT last_observed_margin_mode
FROM identities
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version) AND archived_at IS NULL
FOR UPDATE;

-- name: SetIdentityMarginMode :exec
UPDATE identities
SET last_observed_margin_mode = sqlc.arg(last_observed_margin_mode),
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version) AND archived_at IS NULL;

-- name: ArchiveIdentity :execrows
UPDATE identities
SET archived_at = sqlc.arg(archived_at),
    connection_status = IF(connection_status = 'enabled', 'disabled', connection_status),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version) AND archived_at IS NULL;

-- name: CountIdentitiesWithCiphertext :one
SELECT COUNT(*) FROM identities WHERE credentials_ciphertext IS NOT NULL;
