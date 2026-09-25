-- +goose Up
-- Plutus baseline schema. See docs/technical-design.md section 5 for the normative catalog.

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

CREATE TABLE portfolios (
    id BINARY(16) NOT NULL,
    user_id BINARY(16) NOT NULL,
    name VARCHAR(120) NOT NULL,
    timezone VARCHAR(64) NOT NULL,
    collection_interval_seconds INT UNSIGNED NOT NULL DEFAULT 900,
    undeletable BOOLEAN NOT NULL DEFAULT FALSE,
    version BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_portfolios_user_id (user_id, id),
    UNIQUE KEY uq_portfolios_user_name (user_id, name),
    CONSTRAINT chk_portfolios_interval CHECK (collection_interval_seconds BETWEEN 60 AND 86400),
    CONSTRAINT fk_portfolios_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- An Identity is one Platform login. Its read-only credentials (the Connection) live on the same row.
CREATE TABLE identities (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    platform VARCHAR(32) NOT NULL,
    display_name VARCHAR(120) NOT NULL,
    account_structure VARCHAR(64) NOT NULL,
    margin_mode VARCHAR(64) NULL,
    external_ref VARCHAR(191) NULL,
    status VARCHAR(32) NOT NULL,
    archived_at DATETIME(6) NULL,
    connection_status VARCHAR(32) NOT NULL,
    credentials_ciphertext LONGBLOB NULL,
    credentials_nonce BINARY(12) NULL,
    credential_key_version SMALLINT UNSIGNED NOT NULL DEFAULT 0,
    credentials_updated_at DATETIME(6) NULL,
    last_validated_at DATETIME(6) NULL,
    last_collected_at DATETIME(6) NULL,
    last_error_class VARCHAR(64) NULL,
    last_error_detail VARCHAR(1024) NULL,
    version BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    active_display_name VARCHAR(120) AS (IF(status = 'archived', NULL, display_name)) STORED,
    PRIMARY KEY (id),
    UNIQUE KEY uq_identities_portfolio_id (portfolio_id, id),
    UNIQUE KEY uq_identities_active_name (portfolio_id, platform, active_display_name),
    KEY idx_identities_portfolio_status (portfolio_id, status),
    CONSTRAINT chk_identities_platform CHECK (platform IN ('binance', 'okx', 'bybit', 'kraken')),
    CONSTRAINT chk_identities_account_structure CHECK (
        (platform = 'binance' AND account_structure IN ('binance_classic', 'binance_pm'))
        OR (platform = 'okx' AND account_structure = 'okx_unified')
        OR (platform = 'bybit' AND account_structure = 'bybit_uta_2_0')
        OR (platform = 'kraken' AND account_structure IN ('kraken_separate', 'kraken_unified'))
    ),
    CONSTRAINT chk_identities_margin_mode CHECK (
        margin_mode IS NULL
        OR (account_structure = 'okx_unified' AND margin_mode IN ('okx_spot', 'okx_futures', 'okx_mcm', 'okx_pm'))
        OR (account_structure = 'bybit_uta_2_0' AND margin_mode IN ('bybit_isolated', 'bybit_cross', 'bybit_pm'))
    ),
    CONSTRAINT chk_identities_status CHECK (status IN ('enabled', 'archived')),
    CONSTRAINT chk_identities_connection_status CHECK (connection_status IN ('unconfigured', 'disabled', 'enabled', 'invalid')),
    CONSTRAINT chk_identities_credentials CHECK (
        (credentials_ciphertext IS NULL AND credentials_nonce IS NULL)
        OR (credentials_ciphertext IS NOT NULL AND credentials_nonce IS NOT NULL)
    ),
    CONSTRAINT chk_identities_enabled_needs_credentials CHECK (
        connection_status = 'unconfigured' OR credentials_ciphertext IS NOT NULL
    ),
    CONSTRAINT fk_identities_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Accounts are the Account Types the User selected for an Identity, never discovery results.
CREATE TABLE accounts (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    identity_id BINARY(16) NOT NULL,
    account_type VARCHAR(64) NOT NULL,
    display_name VARCHAR(120) NOT NULL,
    selected BOOLEAN NOT NULL DEFAULT TRUE,
    version BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_accounts_portfolio_id (portfolio_id, id),
    UNIQUE KEY uq_accounts_identity_type (portfolio_id, identity_id, account_type),
    CONSTRAINT fk_accounts_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT fk_accounts_identity FOREIGN KEY (portfolio_id, identity_id) REFERENCES identities (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Shared catalog: a Platform symbol and the canonical Asset symbol it maps to.
CREATE TABLE asset_listings (
    id BINARY(16) NOT NULL,
    platform VARCHAR(32) NOT NULL,
    symbol VARCHAR(64) NOT NULL,
    asset_symbol VARCHAR(64) NOT NULL,
    name VARCHAR(191) NULL,
    mapping_source VARCHAR(32) NOT NULL DEFAULT 'auto',
    first_seen_at DATETIME(6) NOT NULL,
    last_seen_at DATETIME(6) NOT NULL,
    version BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_asset_listings_platform_symbol (platform, symbol),
    KEY idx_asset_listings_asset (asset_symbol),
    CONSTRAINT chk_asset_listings_platform CHECK (platform IN ('binance', 'okx', 'bybit', 'kraken')),
    CONSTRAINT chk_asset_listings_source CHECK (mapping_source IN ('auto', 'manual'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE instruments (
    id BINARY(16) NOT NULL,
    platform VARCHAR(32) NOT NULL,
    symbol VARCHAR(64) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    base_symbol VARCHAR(64) NULL,
    quote_symbol VARCHAR(64) NULL,
    settlement_symbol VARCHAR(64) NULL,
    expiry_at DATETIME(6) NULL,
    contract_size DECIMAL(65,30) NULL,
    first_seen_at DATETIME(6) NOT NULL,
    last_seen_at DATETIME(6) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_instruments_platform_symbol (platform, symbol),
    CONSTRAINT chk_instruments_platform CHECK (platform IN ('binance', 'okx', 'bybit', 'kraken')),
    CONSTRAINT chk_instruments_kind CHECK (kind IN ('spot', 'linear_perpetual', 'inverse_perpetual', 'linear_delivery', 'inverse_delivery')),
    CONSTRAINT chk_instruments_delivery_expiry CHECK (
        kind NOT IN ('linear_delivery', 'inverse_delivery') OR expiry_at IS NOT NULL
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Durable work. A collection_cycle job is the Collection Cycle itself.
CREATE TABLE jobs (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    `trigger` VARCHAR(32) NOT NULL,
    dedupe_key VARCHAR(255) NOT NULL,
    payload_json JSON NULL,
    requested_by_user_id BINARY(16) NULL,
    available_at DATETIME(6) NOT NULL,
    lease_owner VARCHAR(191) NULL,
    lease_expires_at DATETIME(6) NULL,
    attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
    max_attempts INT UNSIGNED NOT NULL DEFAULT 3,
    last_error_class VARCHAR(64) NULL,
    last_detail VARCHAR(1024) NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    started_at DATETIME(6) NULL,
    finished_at DATETIME(6) NULL,
    active_dedupe_key VARCHAR(255) AS (IF(status IN ('queued', 'running'), dedupe_key, NULL)) STORED,
    PRIMARY KEY (id),
    UNIQUE KEY uq_jobs_portfolio_id (portfolio_id, id),
    UNIQUE KEY uq_jobs_active_dedupe (portfolio_id, active_dedupe_key),
    KEY idx_jobs_claim (status, available_at, lease_expires_at),
    KEY idx_jobs_portfolio_kind_created (portfolio_id, kind, created_at),
    CONSTRAINT chk_jobs_kind CHECK (kind IN ('collection_cycle', 'valuation_recalc')),
    CONSTRAINT chk_jobs_status CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    CONSTRAINT chk_jobs_trigger CHECK (`trigger` IN ('schedule', 'manual', 'system')),
    CONSTRAINT fk_jobs_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT fk_jobs_requested_by FOREIGN KEY (requested_by_user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- One row per selected Account per Collection Cycle: the Account Report (observation columns)
-- and its Account Valuation (value columns). source_report_id = id when fresh, an earlier ok
-- report when stale, NULL when the Account has never succeeded.
CREATE TABLE account_reports (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    job_id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    status VARCHAR(32) NOT NULL,
    observed_at DATETIME(6) NULL,
    error_class VARCHAR(64) NULL,
    error_detail VARCHAR(1024) NULL,
    equity DECIMAL(65,30) NULL,
    adjusted_equity DECIMAL(65,30) NULL,
    available_equity DECIMAL(65,30) NULL,
    initial_margin_ratio DECIMAL(65,30) NULL,
    maintenance_margin_ratio DECIMAL(65,30) NULL,
    unified_margin_ratio DECIMAL(65,30) NULL,
    value_usd DECIMAL(65,30) NULL,
    method VARCHAR(32) NOT NULL,
    source_report_id BINARY(16) NULL,
    omitted_json JSON NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_account_reports_portfolio_id (portfolio_id, id),
    UNIQUE KEY uq_account_reports_job_account (portfolio_id, job_id, account_id),
    KEY idx_account_reports_account_ok (portfolio_id, account_id, status, created_at),
    CONSTRAINT chk_account_reports_status CHECK (status IN ('ok', 'failed')),
    CONSTRAINT chk_account_reports_method CHECK (method IN ('reported', 'calculated', 'missing')),
    CONSTRAINT chk_account_reports_ok_observed CHECK (status <> 'ok' OR observed_at IS NOT NULL),
    CONSTRAINT chk_account_reports_failed_error CHECK (status <> 'failed' OR error_class IS NOT NULL),
    CONSTRAINT chk_account_reports_missing CHECK (
        (method = 'missing' AND value_usd IS NULL AND source_report_id IS NULL)
        OR (method <> 'missing' AND source_report_id IS NOT NULL)
    ),
    CONSTRAINT fk_account_reports_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT fk_account_reports_job FOREIGN KEY (portfolio_id, job_id) REFERENCES jobs (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_account_reports_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_account_reports_source FOREIGN KEY (portfolio_id, source_report_id) REFERENCES account_reports (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE balances (
    report_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    listing_id BINARY(16) NOT NULL,
    equity DECIMAL(65,30) NOT NULL,
    available_to_withdraw DECIMAL(65,30) NULL,
    available_to_trade DECIMAL(65,30) NULL,
    liability DECIMAL(65,30) NULL,
    interest DECIMAL(65,30) NULL,
    unrealized_pnl DECIMAL(65,30) NULL,
    PRIMARY KEY (report_id, listing_id),
    KEY idx_balances_listing (listing_id),
    CONSTRAINT fk_balances_report FOREIGN KEY (portfolio_id, report_id) REFERENCES account_reports (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_balances_listing FOREIGN KEY (listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE positions (
    report_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    platform_position_ref VARCHAR(191) NOT NULL,
    instrument_id BINARY(16) NOT NULL,
    side VARCHAR(16) NOT NULL,
    quantity DECIMAL(65,30) NOT NULL,
    quantity_unit VARCHAR(16) NOT NULL,
    entry_price DECIMAL(65,30) NULL,
    mark_price DECIMAL(65,30) NULL,
    liquidation_price DECIMAL(65,30) NULL,
    unrealized_pnl DECIMAL(65,30) NULL,
    leverage DECIMAL(65,30) NULL,
    margin DECIMAL(65,30) NULL,
    PRIMARY KEY (report_id, platform_position_ref),
    KEY idx_positions_instrument (instrument_id),
    CONSTRAINT chk_positions_side CHECK (side IN ('long', 'short')),
    CONSTRAINT chk_positions_unit CHECK (quantity_unit IN ('contracts', 'base', 'quote')),
    CONSTRAINT fk_positions_report FOREIGN KEY (portfolio_id, report_id) REFERENCES account_reports (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_positions_instrument FOREIGN KEY (instrument_id) REFERENCES instruments (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Platform market prices in USD-equivalent terms (quoted in USDT or USDC, treated as USD).
CREATE TABLE prices (
    platform VARCHAR(32) NOT NULL,
    asset_symbol VARCHAR(64) NOT NULL,
    observed_at DATETIME(6) NOT NULL,
    price DECIMAL(65,30) NOT NULL,
    quote_symbol VARCHAR(64) NOT NULL,
    PRIMARY KEY (platform, asset_symbol, observed_at),
    CONSTRAINT chk_prices_platform CHECK (platform IN ('binance', 'okx', 'bybit', 'kraken'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE capital_flows (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    direction VARCHAR(32) NOT NULL,
    asset_symbol VARCHAR(64) NOT NULL,
    quantity DECIMAL(65,30) NOT NULL,
    value_usd DECIMAL(65,30) NULL,
    effective_at DATETIME(6) NOT NULL,
    account_id BINARY(16) NULL,
    note VARCHAR(1024) NULL,
    created_by_user_id BINARY(16) NOT NULL,
    version BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_capital_flows_portfolio_id (portfolio_id, id),
    KEY idx_capital_flows_effective (portfolio_id, effective_at),
    CONSTRAINT chk_capital_flows_direction CHECK (direction IN ('contribution', 'withdrawal')),
    CONSTRAINT chk_capital_flows_quantity CHECK (quantity > 0),
    CONSTRAINT fk_capital_flows_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT fk_capital_flows_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_capital_flows_user FOREIGN KEY (created_by_user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- One Valuation Point per Collection Cycle. History queries read only this table.
CREATE TABLE valuation_points (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    job_id BINARY(16) NOT NULL,
    evaluated_at DATETIME(6) NOT NULL,
    total_usd DECIMAL(65,30) NOT NULL,
    account_count INT UNSIGNED NOT NULL,
    stale_account_count INT UNSIGNED NOT NULL,
    incomplete_account_count INT UNSIGNED NOT NULL,
    boundary_contribution_usd DECIMAL(65,30) NOT NULL DEFAULT 0,
    boundary_withdrawal_usd DECIMAL(65,30) NOT NULL DEFAULT 0,
    boundary_json JSON NULL,
    issues_json JSON NULL,
    recalculated_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_valuation_points_portfolio_id (portfolio_id, id),
    UNIQUE KEY uq_valuation_points_job (portfolio_id, job_id),
    UNIQUE KEY uq_valuation_points_time (portfolio_id, evaluated_at),
    CONSTRAINT fk_valuation_points_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT fk_valuation_points_job FOREIGN KEY (portfolio_id, job_id) REFERENCES jobs (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

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

-- +goose Down
DROP TABLE IF EXISTS idempotency_records;
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS valuation_points;
DROP TABLE IF EXISTS capital_flows;
DROP TABLE IF EXISTS prices;
DROP TABLE IF EXISTS positions;
DROP TABLE IF EXISTS balances;
DROP TABLE IF EXISTS account_reports;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS instruments;
DROP TABLE IF EXISTS asset_listings;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS identities;
DROP TABLE IF EXISTS portfolios;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
