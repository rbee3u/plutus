-- +goose Up
CREATE TABLE users (
    id BINARY(16) NOT NULL,
    username VARCHAR(191) NOT NULL,
    username_normalized VARCHAR(191) NOT NULL,
    status VARCHAR(32) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_users_username_normalized (username_normalized),
    KEY idx_users_status (status),
    CONSTRAINT chk_users_status CHECK (status IN ('active', 'disabled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE sessions (
    id BINARY(16) NOT NULL,
    user_id BINARY(16) NOT NULL,
    token_hash BINARY(32) NOT NULL,
    csrf_secret BINARY(32) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    last_seen_at DATETIME(6) NOT NULL,
    idle_expires_at DATETIME(6) NOT NULL,
    absolute_expires_at DATETIME(6) NOT NULL,
    revoked_at DATETIME(6) NULL,
    client_label VARCHAR(255) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_sessions_token_hash (token_hash),
    KEY idx_sessions_user_expiry (user_id, idle_expires_at, absolute_expires_at),
    KEY idx_sessions_absolute_expires (absolute_expires_at),
    CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE assets (
    id BINARY(16) NOT NULL,
    code VARCHAR(64) NOT NULL,
    name VARCHAR(191) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    decimals SMALLINT UNSIGNED NULL,
    status VARCHAR(32) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_assets_code (code),
    CONSTRAINT chk_assets_kind CHECK (kind IN ('fiat', 'crypto', 'other')),
    CONSTRAINT chk_assets_status CHECK (status IN ('active', 'provisional', 'retired'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE platforms (
    id BINARY(16) NOT NULL,
    code VARCHAR(32) NOT NULL,
    name VARCHAR(120) NOT NULL,
    status VARCHAR(32) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_platforms_code (code),
    CONSTRAINT chk_platforms_status CHECK (status IN ('planned', 'enabled', 'disabled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO assets (id, code, name, kind, decimals, status, created_at, updated_at) VALUES
    (UNHEX('01A0BDFC1FA47A549FA124F4721A71AF'), 'USD', 'US Dollar', 'fiat', 2, 'active', '2026-01-01 00:00:00.000000', '2026-01-01 00:00:00.000000'),
    (UNHEX('01A0BDFC1FA47AF4BEDF65D968B9122B'), 'CNY', 'Chinese Yuan', 'fiat', 2, 'active', '2026-01-01 00:00:00.000000', '2026-01-01 00:00:00.000000');

INSERT INTO platforms (id, code, name, status, created_at, updated_at) VALUES
    (UNHEX('01A0BDFC1FA47B00B990A610A553E10C'), 'binance', 'Binance', 'enabled', '2026-01-01 00:00:00.000000', '2026-01-01 00:00:00.000000'),
    (UNHEX('01A0BDFC1FA47B08BC44A2A59E4665B8'), 'okx', 'OKX', 'enabled', '2026-01-01 00:00:00.000000', '2026-01-01 00:00:00.000000'),
    (UNHEX('01A0BDFC1FA47B10990C04AA9D8AC659'), 'bybit', 'Bybit', 'enabled', '2026-01-01 00:00:00.000000', '2026-01-01 00:00:00.000000');

CREATE TABLE portfolios (
    id BINARY(16) NOT NULL,
    user_id BINARY(16) NOT NULL,
    name VARCHAR(120) NOT NULL,
    timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Shanghai',
    display_asset_id BINARY(16) NOT NULL,
    status VARCHAR(32) NOT NULL,
    undeletable BOOLEAN NOT NULL DEFAULT FALSE,
    active_projection_generation_id BINARY(16) NULL,
    version BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    archived_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_portfolios_user_id (user_id, id),
    UNIQUE KEY uq_portfolios_user_name (user_id, name),
    KEY idx_portfolios_user (user_id),
    KEY idx_portfolios_status (status),
    CONSTRAINT chk_portfolios_status CHECK (status IN ('active', 'archived')),
    CONSTRAINT fk_portfolios_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT fk_portfolios_display_asset FOREIGN KEY (display_asset_id) REFERENCES assets (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identities (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    platform_id BINARY(16) NOT NULL,
    parent_identity_id BINARY(16) NULL,
    external_ref VARCHAR(191) NOT NULL,
    display_name VARCHAR(120) NOT NULL,
    account_mode VARCHAR(64) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    version BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    archived_at DATETIME(6) NULL,
    active_external_ref VARCHAR(191) AS (IF(status = 'archived', NULL, external_ref)) STORED,
    PRIMARY KEY (id),
    UNIQUE KEY uq_identities_portfolio_id (portfolio_id, id),
    UNIQUE KEY uq_identities_active_platform_ref (portfolio_id, platform_id, active_external_ref),
    CONSTRAINT chk_identities_kind CHECK (kind IN ('main', 'subordinate')),
    CONSTRAINT chk_identities_status CHECK (status IN ('enabled', 'disabled', 'archived')),
    CONSTRAINT chk_identities_account_mode CHECK (account_mode IN ('binance_pm', 'okx_mcm', 'okx_pm', 'bybit_uta_2_0')),
    CONSTRAINT fk_identities_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT fk_identities_platform FOREIGN KEY (platform_id) REFERENCES platforms (id) ON DELETE RESTRICT,
    CONSTRAINT fk_identities_parent FOREIGN KEY (portfolio_id, parent_identity_id) REFERENCES identities (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE connections (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    identity_id BINARY(16) NOT NULL,
    label VARCHAR(120) NOT NULL,
    status VARCHAR(32) NOT NULL,
    credentials_ciphertext LONGBLOB NULL,
    credentials_nonce BINARY(12) NULL,
    credential_key_version SMALLINT UNSIGNED NOT NULL,
    config_json JSON NOT NULL,
    collection_interval_seconds INT UNSIGNED NOT NULL DEFAULT 900,
    freshness_after_seconds INT UNSIGNED NOT NULL DEFAULT 1800,
    last_validated_at DATETIME(6) NULL,
    version BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    enabled_identity_id BINARY(16) AS (IF(status = 'enabled', identity_id, NULL)) STORED,
    PRIMARY KEY (id),
    UNIQUE KEY uq_connections_portfolio_id (portfolio_id, id),
    UNIQUE KEY uq_connections_enabled_identity (portfolio_id, enabled_identity_id),
    CONSTRAINT chk_connections_status CHECK (status IN ('enabled', 'disabled', 'invalid')),
    CONSTRAINT chk_connections_credentials CHECK (
        (credentials_ciphertext IS NULL AND credentials_nonce IS NULL)
        OR (credentials_ciphertext IS NOT NULL AND credentials_nonce IS NOT NULL)
    ),
    CONSTRAINT fk_connections_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT fk_connections_identity FOREIGN KEY (portfolio_id, identity_id) REFERENCES identities (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE accounts (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    identity_id BINARY(16) NOT NULL,
    external_ref VARCHAR(191) NOT NULL,
    display_name VARCHAR(120) NOT NULL,
    account_type VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    first_seen_at DATETIME(6) NOT NULL,
    last_seen_at DATETIME(6) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_accounts_portfolio_id (portfolio_id, id),
    UNIQUE KEY uq_accounts_identity_ref (portfolio_id, identity_id, external_ref),
    KEY idx_accounts_identity_status (portfolio_id, identity_id, status),
    CONSTRAINT chk_accounts_status CHECK (status IN ('active', 'inactive', 'disappeared')),
    CONSTRAINT fk_accounts_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT fk_accounts_identity FOREIGN KEY (portfolio_id, identity_id) REFERENCES identities (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE jobs (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    priority SMALLINT NOT NULL,
    dedupe_key VARCHAR(255) NOT NULL,
    active_dedupe_key VARCHAR(255) AS (IF(status IN ('queued', 'running'), dedupe_key, NULL)) STORED,
    requested_by_user_id BINARY(16) NULL,
    available_at DATETIME(6) NOT NULL,
    lease_owner VARCHAR(191) NULL,
    lease_expires_at DATETIME(6) NULL,
    attempt_count INT UNSIGNED NOT NULL,
    max_attempts INT UNSIGNED NOT NULL,
    last_error_code VARCHAR(64) NULL,
    last_error_detail VARCHAR(1024) NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    started_at DATETIME(6) NULL,
    finished_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_jobs_portfolio_id (portfolio_id, id),
    UNIQUE KEY uq_jobs_active_dedupe (portfolio_id, active_dedupe_key),
    KEY idx_jobs_claim (status, available_at, priority, lease_expires_at),
    CONSTRAINT chk_jobs_kind CHECK (kind IN ('collection', 'projection')),
    CONSTRAINT chk_jobs_status CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    CONSTRAINT fk_jobs_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT fk_jobs_requested_by FOREIGN KEY (requested_by_user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE job_attempts (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    job_id BINARY(16) NOT NULL,
    attempt_number INT UNSIGNED NOT NULL,
    worker_id VARCHAR(191) NOT NULL,
    started_at DATETIME(6) NOT NULL,
    finished_at DATETIME(6) NULL,
    outcome VARCHAR(32) NOT NULL,
    error_class VARCHAR(64) NULL,
    error_code VARCHAR(64) NULL,
    safe_detail VARCHAR(1024) NULL,
    retry_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_job_attempts_portfolio_id (portfolio_id, id),
    UNIQUE KEY uq_job_attempts_number (portfolio_id, job_id, attempt_number),
    KEY idx_job_attempts_finished (finished_at),
    CONSTRAINT chk_job_attempts_outcome CHECK (outcome IN ('succeeded', 'failed', 'cancelled')),
    CONSTRAINT fk_job_attempts_job FOREIGN KEY (portfolio_id, job_id) REFERENCES jobs (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE projection_job_details (
    portfolio_id BINARY(16) NOT NULL,
    job_id BINARY(16) NOT NULL,
    invalid_from DATETIME(6) NULL,
    invalid_through DATETIME(6) NULL,
    reason VARCHAR(1024) NOT NULL,
    PRIMARY KEY (portfolio_id, job_id),
    CONSTRAINT fk_projection_job_details_job FOREIGN KEY (portfolio_id, job_id) REFERENCES jobs (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE projection_job_kinds (
    portfolio_id BINARY(16) NOT NULL,
    job_id BINARY(16) NOT NULL,
    projection_kind VARCHAR(64) NOT NULL,
    PRIMARY KEY (portfolio_id, job_id, projection_kind),
    CONSTRAINT fk_projection_job_kinds_details FOREIGN KEY (portfolio_id, job_id) REFERENCES projection_job_details (portfolio_id, job_id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE collection_job_details (
    portfolio_id BINARY(16) NOT NULL,
    job_id BINARY(16) NOT NULL,
    connection_id BINARY(16) NOT NULL,
    `trigger` VARCHAR(32) NOT NULL,
    scope VARCHAR(64) NOT NULL,
    PRIMARY KEY (portfolio_id, job_id),
    CONSTRAINT chk_collection_job_trigger CHECK (`trigger` IN ('schedule', 'manual', 'retry')),
    CONSTRAINT fk_collection_job_details_job FOREIGN KEY (portfolio_id, job_id) REFERENCES jobs (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_collection_job_details_connection FOREIGN KEY (portfolio_id, connection_id) REFERENCES connections (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE projection_generations (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    job_id BINARY(16) NOT NULL,
    status VARCHAR(32) NOT NULL,
    timezone VARCHAR(64) NOT NULL,
    display_asset_id BINARY(16) NOT NULL,
    source_watermark_at DATETIME(6) NOT NULL,
    started_at DATETIME(6) NOT NULL,
    completed_at DATETIME(6) NULL,
    activated_at DATETIME(6) NULL,
    failure_code VARCHAR(64) NULL,
    failure_detail VARCHAR(1024) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_projection_generations_portfolio_id (portfolio_id, id),
    CONSTRAINT chk_projection_generations_status CHECK (status IN ('building', 'ready', 'active', 'failed', 'retired')),
    CONSTRAINT fk_projection_generations_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT fk_projection_generations_job FOREIGN KEY (portfolio_id, job_id) REFERENCES jobs (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_projection_generations_asset FOREIGN KEY (display_asset_id) REFERENCES assets (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE portfolios
    ADD CONSTRAINT fk_portfolios_active_generation
        FOREIGN KEY (id, active_projection_generation_id)
        REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT;

CREATE TABLE audit_events (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NULL,
    actor_user_id BINARY(16) NULL,
    action VARCHAR(64) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id BINARY(16) NOT NULL,
    request_id VARCHAR(64) NOT NULL,
    before_json JSON NULL,
    after_json JSON NULL,
    metadata_json JSON NULL,
    occurred_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    KEY idx_audit_events_portfolio_time (portfolio_id, occurred_at),
    KEY idx_audit_events_actor_time (actor_user_id, occurred_at),
    KEY idx_audit_events_resource (resource_type, resource_id),
    CONSTRAINT fk_audit_events_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT fk_audit_events_actor FOREIGN KEY (actor_user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE idempotency_records (
    id BINARY(16) NOT NULL,
    user_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NULL,
    `key` VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    method VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    path VARCHAR(512) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    request_hash BINARY(32) NOT NULL,
    response_status SMALLINT UNSIGNED NOT NULL,
    response_headers_json JSON NULL,
    response_body_json JSON NULL,
    created_at DATETIME(6) NOT NULL,
    expires_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_idempotency_user_request (user_id, method, path, `key`),
    KEY idx_idempotency_expires (expires_at),
    CONSTRAINT fk_idempotency_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT fk_idempotency_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE asset_listings (
    id BINARY(16) NOT NULL,
    platform_id BINARY(16) NOT NULL,
    external_ref VARCHAR(191) NOT NULL,
    symbol VARCHAR(64) NOT NULL,
    name VARCHAR(191) NULL,
    status VARCHAR(32) NOT NULL,
    first_seen_at DATETIME(6) NOT NULL,
    last_seen_at DATETIME(6) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_asset_listings_platform_ref (platform_id, external_ref),
    KEY idx_asset_listings_platform_symbol (platform_id, symbol),
    CONSTRAINT chk_asset_listings_status CHECK (status IN ('active', 'inactive', 'unknown')),
    CONSTRAINT fk_asset_listings_platform FOREIGN KEY (platform_id) REFERENCES platforms (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE instruments (
    id BINARY(16) NOT NULL,
    platform_id BINARY(16) NOT NULL,
    external_ref VARCHAR(191) NOT NULL,
    symbol VARCHAR(64) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    base_listing_id BINARY(16) NULL,
    quote_listing_id BINARY(16) NULL,
    underlying_listing_id BINARY(16) NULL,
    settlement_listing_id BINARY(16) NULL,
    expiry_at DATETIME(6) NULL,
    contract_size DECIMAL(65, 30) NULL,
    status VARCHAR(32) NOT NULL,
    first_seen_at DATETIME(6) NOT NULL,
    last_seen_at DATETIME(6) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_instruments_platform_ref (platform_id, external_ref),
    KEY idx_instruments_symbol (symbol),
    KEY idx_instruments_underlying (underlying_listing_id),
    KEY idx_instruments_settlement (settlement_listing_id),
    KEY idx_instruments_status (status),
    CONSTRAINT chk_instruments_kind CHECK (kind IN (
        'spot',
        'linear_perpetual',
        'inverse_perpetual',
        'linear_delivery',
        'inverse_delivery'
    )),
    CONSTRAINT chk_instruments_status CHECK (status IN ('active', 'expired', 'unknown')),
    CONSTRAINT chk_instruments_delivery_expiry CHECK (
        kind NOT IN ('linear_delivery', 'inverse_delivery') OR expiry_at IS NOT NULL
    ),
    CONSTRAINT fk_instruments_platform FOREIGN KEY (platform_id) REFERENCES platforms (id) ON DELETE RESTRICT,
    CONSTRAINT fk_instruments_base_listing FOREIGN KEY (base_listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT,
    CONSTRAINT fk_instruments_quote_listing FOREIGN KEY (quote_listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT,
    CONSTRAINT fk_instruments_underlying_listing FOREIGN KEY (underlying_listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT,
    CONSTRAINT fk_instruments_settlement_listing FOREIGN KEY (settlement_listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE asset_listing_mappings (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    asset_listing_id BINARY(16) NOT NULL,
    current_revision_id BINARY(16) NULL,
    version BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_alm_portfolio_id (portfolio_id, id),
    UNIQUE KEY uq_alm_listing (portfolio_id, asset_listing_id),
    CONSTRAINT fk_alm_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT fk_alm_listing FOREIGN KEY (asset_listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE asset_listing_mapping_revisions (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    mapping_id BINARY(16) NOT NULL,
    asset_id BINARY(16) NOT NULL,
    state VARCHAR(32) NOT NULL,
    display_name_override VARCHAR(191) NULL,
    classification_json JSON NULL,
    reason VARCHAR(1024) NULL,
    created_by_user_id BINARY(16) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_almr_portfolio_id (portfolio_id, id),
    KEY idx_almr_mapping (portfolio_id, mapping_id),
    CONSTRAINT chk_almr_state CHECK (state IN ('provisional', 'confirmed')),
    CONSTRAINT fk_almr_mapping FOREIGN KEY (portfolio_id, mapping_id) REFERENCES asset_listing_mappings (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_almr_asset FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT,
    CONSTRAINT fk_almr_user FOREIGN KEY (created_by_user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE asset_listing_mappings
    ADD CONSTRAINT fk_alm_current_revision
        FOREIGN KEY (portfolio_id, current_revision_id)
        REFERENCES asset_listing_mapping_revisions (portfolio_id, id) ON DELETE RESTRICT;

CREATE TABLE catalog_pins (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    asset_listing_id BINARY(16) NULL,
    instrument_id BINARY(16) NULL,
    current_revision_id BINARY(16) NULL,
    version BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    listing_subject_key BINARY(16) AS (IF(asset_listing_id IS NOT NULL, asset_listing_id, NULL)) STORED,
    instrument_subject_key BINARY(16) AS (IF(instrument_id IS NOT NULL, instrument_id, NULL)) STORED,
    PRIMARY KEY (id),
    UNIQUE KEY uq_catalog_pins_portfolio_id (portfolio_id, id),
    UNIQUE KEY uq_catalog_pins_listing (portfolio_id, listing_subject_key),
    UNIQUE KEY uq_catalog_pins_instrument (portfolio_id, instrument_subject_key),
    CONSTRAINT chk_catalog_pins_subject CHECK (
        (asset_listing_id IS NOT NULL AND instrument_id IS NULL)
        OR (asset_listing_id IS NULL AND instrument_id IS NOT NULL)
    ),
    CONSTRAINT fk_catalog_pins_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT fk_catalog_pins_listing FOREIGN KEY (asset_listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT,
    CONSTRAINT fk_catalog_pins_instrument FOREIGN KEY (instrument_id) REFERENCES instruments (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE catalog_pin_revisions (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    catalog_pin_id BINARY(16) NOT NULL,
    enabled BOOLEAN NOT NULL,
    reason VARCHAR(1024) NULL,
    created_by_user_id BINARY(16) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_cpr_portfolio_id (portfolio_id, id),
    KEY idx_cpr_pin (portfolio_id, catalog_pin_id),
    CONSTRAINT fk_cpr_pin FOREIGN KEY (portfolio_id, catalog_pin_id) REFERENCES catalog_pins (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_cpr_user FOREIGN KEY (created_by_user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE catalog_pins
    ADD CONSTRAINT fk_catalog_pins_current_revision
        FOREIGN KEY (portfolio_id, current_revision_id)
        REFERENCES catalog_pin_revisions (portfolio_id, id) ON DELETE RESTRICT;

CREATE TABLE collection_runs (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    job_id BINARY(16) NOT NULL,
    connection_id BINARY(16) NOT NULL,
    identity_id BINARY(16) NOT NULL,
    adapter_name VARCHAR(64) NOT NULL,
    adapter_version VARCHAR(64) NOT NULL,
    started_at DATETIME(6) NOT NULL,
    finished_at DATETIME(6) NULL,
    status VARCHAR(32) NOT NULL,
    successful_scope_count INT UNSIGNED NOT NULL,
    failed_scope_count INT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_collection_runs_portfolio_id (portfolio_id, id),
    KEY idx_collection_runs_identity_started (portfolio_id, identity_id, started_at),
    CONSTRAINT chk_collection_runs_status CHECK (status IN ('running', 'succeeded', 'partial', 'failed')),
    CONSTRAINT fk_collection_runs_job FOREIGN KEY (portfolio_id, job_id) REFERENCES jobs (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_collection_runs_connection FOREIGN KEY (portfolio_id, connection_id) REFERENCES connections (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_collection_runs_identity FOREIGN KEY (portfolio_id, identity_id) REFERENCES identities (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_snapshots (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    collection_run_id BINARY(16) NOT NULL,
    identity_id BINARY(16) NOT NULL,
    started_at DATETIME(6) NOT NULL,
    completed_at DATETIME(6) NULL,
    status VARCHAR(32) NOT NULL,
    completeness VARCHAR(32) NOT NULL,
    expected_scope_count INT UNSIGNED NOT NULL,
    successful_scope_count INT UNSIGNED NOT NULL,
    failed_scope_count INT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_identity_snapshots_portfolio_id (portfolio_id, id),
    KEY idx_identity_snapshots_identity (portfolio_id, identity_id, started_at),
    CONSTRAINT chk_identity_snapshots_status CHECK (status IN ('complete', 'partial', 'failed')),
    CONSTRAINT chk_identity_snapshots_completeness CHECK (completeness IN ('complete', 'partial')),
    CONSTRAINT fk_identity_snapshots_run FOREIGN KEY (portfolio_id, collection_run_id) REFERENCES collection_runs (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_identity_snapshots_identity FOREIGN KEY (portfolio_id, identity_id) REFERENCES identities (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE collection_failures (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    collection_run_id BINARY(16) NOT NULL,
    identity_snapshot_id BINARY(16) NOT NULL,
    account_id BINARY(16) NULL,
    scope_kind VARCHAR(64) NOT NULL,
    scope_ref VARCHAR(191) NULL,
    phase VARCHAR(64) NOT NULL,
    error_class VARCHAR(64) NOT NULL,
    error_code VARCHAR(64) NOT NULL,
    safe_detail VARCHAR(1024) NOT NULL,
    retriable BOOLEAN NOT NULL,
    occurred_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_collection_failures_portfolio_id (portfolio_id, id),
    KEY idx_collection_failures_run (portfolio_id, collection_run_id),
    CONSTRAINT fk_collection_failures_run FOREIGN KEY (portfolio_id, collection_run_id) REFERENCES collection_runs (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_collection_failures_snapshot FOREIGN KEY (portfolio_id, identity_snapshot_id) REFERENCES identity_snapshots (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_collection_failures_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE observations (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    identity_snapshot_id BINARY(16) NOT NULL,
    identity_id BINARY(16) NOT NULL,
    account_id BINARY(16) NULL,
    kind VARCHAR(32) NOT NULL,
    source_observed_at DATETIME(6) NOT NULL,
    collected_at DATETIME(6) NOT NULL,
    completeness VARCHAR(32) NOT NULL,
    adapter_name VARCHAR(64) NOT NULL,
    adapter_version VARCHAR(64) NOT NULL,
    schema_version VARCHAR(64) NOT NULL,
    version BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_observations_portfolio_id (portfolio_id, id),
    KEY idx_observations_kind_time (portfolio_id, kind, source_observed_at),
    KEY idx_observations_identity_time (portfolio_id, identity_id, source_observed_at),
    KEY idx_observations_account_kind_time (portfolio_id, account_id, kind, source_observed_at),
    CONSTRAINT chk_observations_kind CHECK (kind IN (
        'balance',
        'position',
        'reported_equity',
        'account_metric',
        'price'
    )),
    CONSTRAINT chk_observations_completeness CHECK (completeness IN ('complete', 'partial')),
    CONSTRAINT fk_observations_snapshot FOREIGN KEY (portfolio_id, identity_snapshot_id) REFERENCES identity_snapshots (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_observations_identity FOREIGN KEY (portfolio_id, identity_id) REFERENCES identities (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_observations_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE balance_observations (
    portfolio_id BINARY(16) NOT NULL,
    observation_id BINARY(16) NOT NULL,
    asset_listing_id BINARY(16) NOT NULL,
    total_quantity DECIMAL(65, 30) NULL,
    available_quantity DECIMAL(65, 30) NULL,
    locked_quantity DECIMAL(65, 30) NULL,
    PRIMARY KEY (portfolio_id, observation_id),
    CONSTRAINT chk_balance_observations_quantity CHECK (
        total_quantity IS NOT NULL
        OR available_quantity IS NOT NULL
        OR locked_quantity IS NOT NULL
    ),
    CONSTRAINT fk_balance_observations_header FOREIGN KEY (portfolio_id, observation_id) REFERENCES observations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_balance_observations_listing FOREIGN KEY (asset_listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE position_observations (
    portfolio_id BINARY(16) NOT NULL,
    observation_id BINARY(16) NOT NULL,
    instrument_id BINARY(16) NOT NULL,
    platform_position_ref VARCHAR(191) NOT NULL,
    side VARCHAR(32) NOT NULL,
    quantity DECIMAL(65, 30) NOT NULL,
    entry_price DECIMAL(65, 30) NULL,
    mark_price DECIMAL(65, 30) NULL,
    unrealized_value DECIMAL(65, 30) NULL,
    leverage DECIMAL(65, 30) NULL,
    PRIMARY KEY (portfolio_id, observation_id),
    KEY idx_position_observations_lineage (portfolio_id, instrument_id, platform_position_ref),
    CONSTRAINT chk_position_observations_side CHECK (side IN ('long', 'short', 'net')),
    CONSTRAINT fk_position_observations_header FOREIGN KEY (portfolio_id, observation_id) REFERENCES observations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_position_observations_instrument FOREIGN KEY (instrument_id) REFERENCES instruments (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE reported_equity_observations (
    portfolio_id BINARY(16) NOT NULL,
    observation_id BINARY(16) NOT NULL,
    denomination_listing_id BINARY(16) NOT NULL,
    amount DECIMAL(65, 30) NOT NULL,
    PRIMARY KEY (portfolio_id, observation_id),
    CONSTRAINT fk_reo_header FOREIGN KEY (portfolio_id, observation_id) REFERENCES observations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_reo_listing FOREIGN KEY (denomination_listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE reported_equity_covered_identities (
    portfolio_id BINARY(16) NOT NULL,
    observation_id BINARY(16) NOT NULL,
    identity_id BINARY(16) NOT NULL,
    PRIMARY KEY (portfolio_id, observation_id, identity_id),
    CONSTRAINT fk_reci_header FOREIGN KEY (portfolio_id, observation_id) REFERENCES reported_equity_observations (portfolio_id, observation_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reci_identity FOREIGN KEY (portfolio_id, identity_id) REFERENCES identities (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE reported_equity_covered_accounts (
    portfolio_id BINARY(16) NOT NULL,
    observation_id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    PRIMARY KEY (portfolio_id, observation_id, account_id),
    CONSTRAINT fk_reca_header FOREIGN KEY (portfolio_id, observation_id) REFERENCES reported_equity_observations (portfolio_id, observation_id) ON DELETE RESTRICT,
    CONSTRAINT fk_reca_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE account_metric_observations (
    portfolio_id BINARY(16) NOT NULL,
    observation_id BINARY(16) NOT NULL,
    metric_code VARCHAR(64) NOT NULL,
    unit VARCHAR(64) NOT NULL,
    decimal_value DECIMAL(65, 30) NULL,
    text_value VARCHAR(512) NULL,
    semantics_version VARCHAR(64) NOT NULL,
    PRIMARY KEY (portfolio_id, observation_id),
    CONSTRAINT chk_account_metric_value CHECK (
        (decimal_value IS NOT NULL AND text_value IS NULL)
        OR (decimal_value IS NULL AND text_value IS NOT NULL)
    ),
    CONSTRAINT fk_account_metric_header FOREIGN KEY (portfolio_id, observation_id) REFERENCES observations (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE price_observations (
    portfolio_id BINARY(16) NOT NULL,
    observation_id BINARY(16) NOT NULL,
    instrument_id BINARY(16) NULL,
    base_listing_id BINARY(16) NULL,
    quote_listing_id BINARY(16) NOT NULL,
    price_type VARCHAR(32) NOT NULL,
    price DECIMAL(65, 30) NOT NULL,
    PRIMARY KEY (portfolio_id, observation_id),
    KEY idx_price_observations_instrument (portfolio_id, instrument_id, price_type),
    KEY idx_price_observations_listing (portfolio_id, base_listing_id, quote_listing_id, price_type),
    CONSTRAINT chk_price_observations_subject CHECK (
        (instrument_id IS NOT NULL AND base_listing_id IS NULL)
        OR (instrument_id IS NULL AND base_listing_id IS NOT NULL)
    ),
    CONSTRAINT chk_price_observations_type CHECK (price_type IN ('market', 'mark', 'index', 'exchange_rate')),
    CONSTRAINT fk_price_observations_header FOREIGN KEY (portfolio_id, observation_id) REFERENCES observations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_price_observations_instrument FOREIGN KEY (instrument_id) REFERENCES instruments (id) ON DELETE RESTRICT,
    CONSTRAINT fk_price_observations_base FOREIGN KEY (base_listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT,
    CONSTRAINT fk_price_observations_quote FOREIGN KEY (quote_listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE capital_baselines (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    current_revision_id BINARY(16) NULL,
    version BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_capital_baselines_portfolio_id (portfolio_id, id),
    UNIQUE KEY uq_capital_baselines_portfolio (portfolio_id),
    CONSTRAINT fk_capital_baselines_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE capital_baseline_revisions (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    capital_baseline_id BINARY(16) NOT NULL,
    effective_at DATETIME(6) NOT NULL,
    asset_id BINARY(16) NOT NULL,
    quantity DECIMAL(65, 30) NOT NULL,
    note VARCHAR(1024) NULL,
    created_by_user_id BINARY(16) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_cbr_portfolio_id (portfolio_id, id),
    KEY idx_cbr_baseline (portfolio_id, capital_baseline_id),
    CONSTRAINT fk_cbr_baseline FOREIGN KEY (portfolio_id, capital_baseline_id) REFERENCES capital_baselines (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_cbr_asset FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT,
    CONSTRAINT fk_cbr_user FOREIGN KEY (created_by_user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE capital_baselines
    ADD CONSTRAINT fk_capital_baselines_current_revision
        FOREIGN KEY (portfolio_id, current_revision_id)
        REFERENCES capital_baseline_revisions (portfolio_id, id) ON DELETE RESTRICT;

CREATE TABLE capital_flows (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    current_revision_id BINARY(16) NULL,
    version BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_capital_flows_portfolio_id (portfolio_id, id),
    CONSTRAINT fk_capital_flows_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE capital_flow_revisions (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    capital_flow_id BINARY(16) NOT NULL,
    direction VARCHAR(32) NOT NULL,
    quantity DECIMAL(65, 30) NOT NULL,
    asset_id BINARY(16) NOT NULL,
    effective_at DATETIME(6) NOT NULL,
    account_id BINARY(16) NULL,
    note VARCHAR(1024) NULL,
    state VARCHAR(32) NOT NULL,
    created_by_user_id BINARY(16) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_cfr_portfolio_id (portfolio_id, id),
    KEY idx_cfr_flow (portfolio_id, capital_flow_id),
    CONSTRAINT chk_cfr_direction CHECK (direction IN ('contribution', 'withdrawal')),
    CONSTRAINT chk_cfr_quantity CHECK (quantity > 0),
    CONSTRAINT chk_cfr_state CHECK (state IN ('active', 'deleted')),
    CONSTRAINT fk_cfr_flow FOREIGN KEY (portfolio_id, capital_flow_id) REFERENCES capital_flows (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_cfr_asset FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT,
    CONSTRAINT fk_cfr_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_cfr_user FOREIGN KEY (created_by_user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE capital_flows
    ADD CONSTRAINT fk_capital_flows_current_revision
        FOREIGN KEY (portfolio_id, current_revision_id)
        REFERENCES capital_flow_revisions (portfolio_id, id) ON DELETE RESTRICT;

CREATE TABLE latest_balance_projections (
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    identity_id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    asset_listing_id BINARY(16) NOT NULL,
    asset_id BINARY(16) NULL,
    observation_id BINARY(16) NOT NULL,
    total_quantity DECIMAL(65, 30) NULL,
    available_quantity DECIMAL(65, 30) NULL,
    locked_quantity DECIMAL(65, 30) NULL,
    source_observed_at DATETIME(6) NOT NULL,
    freshness_state VARCHAR(32) NOT NULL,
    completeness VARCHAR(32) NOT NULL,
    PRIMARY KEY (generation_id, account_id, asset_listing_id),
    CONSTRAINT chk_lbp_freshness CHECK (freshness_state IN ('fresh', 'stale', 'unknown')),
    CONSTRAINT chk_lbp_completeness CHECK (completeness IN ('complete', 'partial', 'missing')),
    CONSTRAINT fk_lbp_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_lbp_identity FOREIGN KEY (portfolio_id, identity_id) REFERENCES identities (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_lbp_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_lbp_listing FOREIGN KEY (asset_listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT,
    CONSTRAINT fk_lbp_asset FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT,
    CONSTRAINT fk_lbp_observation FOREIGN KEY (portfolio_id, observation_id) REFERENCES observations (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE latest_position_projections (
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    identity_id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    instrument_id BINARY(16) NOT NULL,
    platform_position_ref VARCHAR(191) NOT NULL,
    side VARCHAR(32) NOT NULL,
    quantity DECIMAL(65, 30) NOT NULL,
    selected_mark_price DECIMAL(65, 30) NULL,
    underlying_long_quantity DECIMAL(65, 30) NOT NULL,
    underlying_short_quantity DECIMAL(65, 30) NOT NULL,
    observation_id BINARY(16) NOT NULL,
    source_observed_at DATETIME(6) NOT NULL,
    freshness_state VARCHAR(32) NOT NULL,
    completeness VARCHAR(32) NOT NULL,
    PRIMARY KEY (generation_id, account_id, instrument_id, platform_position_ref),
    CONSTRAINT chk_lpp_side CHECK (side IN ('long', 'short', 'net')),
    CONSTRAINT chk_lpp_freshness CHECK (freshness_state IN ('fresh', 'stale', 'unknown')),
    CONSTRAINT chk_lpp_completeness CHECK (completeness IN ('complete', 'partial', 'missing')),
    CONSTRAINT fk_lpp_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_lpp_identity FOREIGN KEY (portfolio_id, identity_id) REFERENCES identities (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_lpp_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_lpp_instrument FOREIGN KEY (instrument_id) REFERENCES instruments (id) ON DELETE RESTRICT,
    CONSTRAINT fk_lpp_observation FOREIGN KEY (portfolio_id, observation_id) REFERENCES observations (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE portfolio_valuation_projections (
    id BINARY(16) NOT NULL,
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    evaluated_at DATETIME(6) NOT NULL,
    denomination_asset_id BINARY(16) NOT NULL,
    amount DECIMAL(65, 30) NULL,
    completeness VARCHAR(32) NOT NULL,
    coverage_fingerprint VARCHAR(128) NOT NULL,
    freshness_state VARCHAR(32) NOT NULL,
    source_cutoff_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_pvp_portfolio_id (portfolio_id, id),
    UNIQUE KEY uq_pvp_eval (generation_id, evaluated_at, denomination_asset_id),
    KEY idx_pvp_generation_time (generation_id, evaluated_at),
    CONSTRAINT chk_pvp_completeness CHECK (completeness IN ('complete', 'partial', 'missing')),
    CONSTRAINT chk_pvp_freshness CHECK (freshness_state IN ('fresh', 'stale', 'unknown')),
    CONSTRAINT fk_pvp_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_pvp_asset FOREIGN KEY (denomination_asset_id) REFERENCES assets (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE valuation_contribution_projections (
    id BINARY(16) NOT NULL,
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    portfolio_valuation_id BINARY(16) NOT NULL,
    contribution_kind VARCHAR(32) NOT NULL,
    identity_id BINARY(16) NULL,
    account_id BINARY(16) NULL,
    asset_id BINARY(16) NULL,
    amount DECIMAL(65, 30) NULL,
    completeness VARCHAR(32) NOT NULL,
    coverage_fingerprint VARCHAR(128) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_vcp_portfolio_id (portfolio_id, id),
    KEY idx_vcp_valuation (portfolio_id, portfolio_valuation_id),
    CONSTRAINT chk_vcp_kind CHECK (contribution_kind IN ('reported_equity', 'calculated')),
    CONSTRAINT chk_vcp_completeness CHECK (completeness IN ('complete', 'partial', 'missing')),
    CONSTRAINT fk_vcp_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_vcp_valuation FOREIGN KEY (portfolio_id, portfolio_valuation_id) REFERENCES portfolio_valuation_projections (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_vcp_identity FOREIGN KEY (portfolio_id, identity_id) REFERENCES identities (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_vcp_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_vcp_asset FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE valuation_contribution_sources (
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    valuation_contribution_id BINARY(16) NOT NULL,
    observation_id BINARY(16) NOT NULL,
    source_role VARCHAR(64) NOT NULL,
    PRIMARY KEY (valuation_contribution_id, observation_id, source_role),
    CONSTRAINT fk_vcs_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_vcs_contribution FOREIGN KEY (portfolio_id, valuation_contribution_id) REFERENCES valuation_contribution_projections (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_vcs_observation FOREIGN KEY (portfolio_id, observation_id) REFERENCES observations (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE exposure_projections (
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    evaluated_at DATETIME(6) NOT NULL,
    asset_id BINARY(16) NOT NULL,
    long_quantity DECIMAL(65, 30) NOT NULL,
    short_quantity DECIMAL(65, 30) NOT NULL,
    net_quantity DECIMAL(65, 30) NOT NULL,
    long_value DECIMAL(65, 30) NULL,
    short_value DECIMAL(65, 30) NULL,
    net_value DECIMAL(65, 30) NULL,
    denomination_asset_id BINARY(16) NOT NULL,
    completeness VARCHAR(32) NOT NULL,
    freshness_state VARCHAR(32) NOT NULL,
    PRIMARY KEY (generation_id, evaluated_at, asset_id),
    CONSTRAINT chk_exp_completeness CHECK (completeness IN ('complete', 'partial', 'missing')),
    CONSTRAINT chk_exp_freshness CHECK (freshness_state IN ('fresh', 'stale', 'unknown')),
    CONSTRAINT fk_exp_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_exp_asset FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT,
    CONSTRAINT fk_exp_denomination FOREIGN KEY (denomination_asset_id) REFERENCES assets (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE contributed_capital_projections (
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    evaluated_at DATETIME(6) NOT NULL,
    denomination_asset_id BINARY(16) NOT NULL,
    amount DECIMAL(65, 30) NULL,
    completeness VARCHAR(32) NOT NULL,
    source_revision_watermark BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (generation_id, evaluated_at, denomination_asset_id),
    CONSTRAINT chk_ccp_completeness CHECK (completeness IN ('complete', 'partial', 'missing')),
    CONSTRAINT fk_ccp_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_ccp_asset FOREIGN KEY (denomination_asset_id) REFERENCES assets (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE value_change_projections (
    id BINARY(16) NOT NULL,
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    period_kind VARCHAR(32) NOT NULL,
    period_start_date DATE NOT NULL,
    period_end_date DATE NOT NULL,
    as_of_at DATETIME(6) NOT NULL,
    beginning_valuation_id BINARY(16) NOT NULL,
    ending_valuation_id BINARY(16) NOT NULL,
    contribution_amount DECIMAL(65, 30) NULL,
    withdrawal_amount DECIMAL(65, 30) NULL,
    amount DECIMAL(65, 30) NULL,
    state VARCHAR(32) NOT NULL,
    coverage_fingerprint VARCHAR(128) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_vch_period (generation_id, period_kind, period_start_date, period_end_date, as_of_at),
    CONSTRAINT chk_vch_period_kind CHECK (period_kind IN ('day', 'iso_week', 'month', 'quarter', 'year')),
    CONSTRAINT chk_vch_state CHECK (state IN ('complete', 'provisional', 'incomplete', 'incomparable')),
    CONSTRAINT fk_vch_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_vch_begin FOREIGN KEY (portfolio_id, beginning_valuation_id) REFERENCES portfolio_valuation_projections (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_vch_end FOREIGN KEY (portfolio_id, ending_valuation_id) REFERENCES portfolio_valuation_projections (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE period_summary_projections (
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    period_kind VARCHAR(32) NOT NULL,
    period_start_date DATE NOT NULL,
    period_end_date DATE NOT NULL,
    as_of_at DATETIME(6) NOT NULL,
    valuation_open DECIMAL(65, 30) NULL,
    valuation_close DECIMAL(65, 30) NULL,
    valuation_high DECIMAL(65, 30) NULL,
    valuation_low DECIMAL(65, 30) NULL,
    contributed_capital_close DECIMAL(65, 30) NULL,
    value_change DECIMAL(65, 30) NULL,
    freshness_state VARCHAR(32) NOT NULL,
    completeness VARCHAR(32) NOT NULL,
    PRIMARY KEY (generation_id, period_kind, period_start_date, period_end_date, as_of_at),
    CONSTRAINT chk_psp_period_kind CHECK (period_kind IN ('day', 'iso_week', 'month', 'quarter', 'year')),
    CONSTRAINT chk_psp_freshness CHECK (freshness_state IN ('fresh', 'stale', 'unknown')),
    CONSTRAINT chk_psp_completeness CHECK (completeness IN ('complete', 'partial', 'missing')),
    CONSTRAINT fk_psp_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE projection_issues (
    id BINARY(16) NOT NULL,
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    projection_kind VARCHAR(64) NOT NULL,
    projection_key VARCHAR(255) NOT NULL,
    severity VARCHAR(32) NOT NULL,
    code VARCHAR(64) NOT NULL,
    summary VARCHAR(1024) NOT NULL,
    details_json JSON NULL,
    identity_id BINARY(16) NULL,
    account_id BINARY(16) NULL,
    observation_id BINARY(16) NULL,
    first_effective_at DATETIME(6) NOT NULL,
    last_effective_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    KEY idx_projection_issues_generation (generation_id, projection_kind),
    CONSTRAINT chk_projection_issues_severity CHECK (severity IN ('info', 'warning', 'error')),
    CONSTRAINT fk_projection_issues_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_projection_issues_identity FOREIGN KEY (portfolio_id, identity_id) REFERENCES identities (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_projection_issues_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_projection_issues_observation FOREIGN KEY (portfolio_id, observation_id) REFERENCES observations (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS projection_issues;
DROP TABLE IF EXISTS period_summary_projections;
DROP TABLE IF EXISTS value_change_projections;
DROP TABLE IF EXISTS contributed_capital_projections;
DROP TABLE IF EXISTS exposure_projections;
DROP TABLE IF EXISTS valuation_contribution_sources;
DROP TABLE IF EXISTS valuation_contribution_projections;
DROP TABLE IF EXISTS portfolio_valuation_projections;
DROP TABLE IF EXISTS latest_position_projections;
DROP TABLE IF EXISTS latest_balance_projections;
ALTER TABLE capital_flows DROP FOREIGN KEY fk_capital_flows_current_revision;
ALTER TABLE capital_baselines DROP FOREIGN KEY fk_capital_baselines_current_revision;
DROP TABLE IF EXISTS capital_flow_revisions;
DROP TABLE IF EXISTS capital_flows;
DROP TABLE IF EXISTS capital_baseline_revisions;
DROP TABLE IF EXISTS capital_baselines;
DROP TABLE IF EXISTS price_observations;
DROP TABLE IF EXISTS account_metric_observations;
DROP TABLE IF EXISTS reported_equity_covered_accounts;
DROP TABLE IF EXISTS reported_equity_covered_identities;
DROP TABLE IF EXISTS reported_equity_observations;
DROP TABLE IF EXISTS position_observations;
DROP TABLE IF EXISTS balance_observations;
DROP TABLE IF EXISTS observations;
DROP TABLE IF EXISTS collection_failures;
DROP TABLE IF EXISTS identity_snapshots;
DROP TABLE IF EXISTS collection_runs;
ALTER TABLE catalog_pins DROP FOREIGN KEY fk_catalog_pins_current_revision;
ALTER TABLE asset_listing_mappings DROP FOREIGN KEY fk_alm_current_revision;
DROP TABLE IF EXISTS catalog_pin_revisions;
DROP TABLE IF EXISTS catalog_pins;
DROP TABLE IF EXISTS asset_listing_mapping_revisions;
DROP TABLE IF EXISTS asset_listing_mappings;
DROP TABLE IF EXISTS instruments;
DROP TABLE IF EXISTS asset_listings;
DROP TABLE IF EXISTS idempotency_records;
DROP TABLE IF EXISTS audit_events;
ALTER TABLE portfolios DROP FOREIGN KEY fk_portfolios_active_generation;
DROP TABLE IF EXISTS projection_generations;
DROP TABLE IF EXISTS collection_job_details;
DROP TABLE IF EXISTS projection_job_kinds;
DROP TABLE IF EXISTS projection_job_details;
DROP TABLE IF EXISTS job_attempts;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS connections;
DROP TABLE IF EXISTS identities;
DROP TABLE IF EXISTS portfolios;
DROP TABLE IF EXISTS platforms;
DROP TABLE IF EXISTS assets;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
