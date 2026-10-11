-- +goose Up
-- Prelaunch baseline. Every Portfolio-owned key leads with portfolio_id, so
-- composite references cannot cross Portfolios.

CREATE TABLE users (
    id BINARY(16) NOT NULL,
    username VARCHAR(191) NOT NULL,
    username_key VARCHAR(573) NOT NULL,
    status VARCHAR(32) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_users_username_key (username_key),
    CONSTRAINT chk_users_status CHECK (status IN ('active', 'disabled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

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
    user_agent VARCHAR(255) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_sessions_token_hash (token_hash),
    KEY idx_sessions_user (user_id, created_at, id),
    CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE idempotency_records (
    user_id BINARY(16) NOT NULL,
    method VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    path VARCHAR(512) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    idempotency_key VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    request_hash BINARY(32) NOT NULL,
    response_status SMALLINT UNSIGNED NOT NULL,
    response_headers_json JSON NULL,
    response_body_json JSON NOT NULL,
    expires_at DATETIME(6) NOT NULL,
    CONSTRAINT chk_idempotency_records_response CHECK (response_status BETWEEN 200 AND 299),
    PRIMARY KEY (user_id, method, path, idempotency_key),
    KEY idx_idempotency_records_expiry (expires_at),
    CONSTRAINT fk_idempotency_records_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE portfolios (
    id BINARY(16) NOT NULL,
    user_id BINARY(16) NOT NULL,
    name VARCHAR(120) NOT NULL,
    name_key VARCHAR(360) NOT NULL,
    timezone VARCHAR(64) NOT NULL,
    collection_interval_seconds INT UNSIGNED NOT NULL DEFAULT 1800,
    is_initial BOOLEAN NOT NULL DEFAULT FALSE,
    deletion_requested_at DATETIME(6) NULL,
    price_anchors_before DATETIME(6) NULL,
    history_before DATETIME(6) NULL,
    initial_user_id BINARY(16) AS (IF(is_initial, user_id, NULL)) STORED,
    version BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    CONSTRAINT chk_portfolios_is_initial CHECK (is_initial IN (0, 1)),
    PRIMARY KEY (id),
    UNIQUE KEY uq_portfolios_user_name (user_id, name_key),
    UNIQUE KEY uq_portfolios_initial_user (initial_user_id),
    CONSTRAINT chk_portfolios_interval CHECK (collection_interval_seconds BETWEEN 900 AND 86400),
    CONSTRAINT chk_portfolios_version CHECK (version >= 1),
    CONSTRAINT fk_portfolios_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE identities (
    portfolio_id BINARY(16) NOT NULL,
    id BINARY(16) NOT NULL,
    platform VARCHAR(32) NOT NULL,
    display_name VARCHAR(120) NOT NULL,
    display_name_key VARCHAR(360) NOT NULL,
    active_name_key VARCHAR(360) AS (IF(archived_at IS NULL, display_name_key, NULL)) STORED,
    account_structure VARCHAR(64) NOT NULL,
    last_observed_margin_mode VARCHAR(64) NULL,
    platform_login_ref VARCHAR(255) NULL,
    active_login_ref VARCHAR(255) AS (IF(archived_at IS NULL, platform_login_ref, NULL)) STORED,
    archived_at DATETIME(6) NULL,
    connection_status VARCHAR(32) NOT NULL,
    credentials_ciphertext LONGBLOB NULL,
    credentials_nonce BINARY(12) NULL,
    credentials_key_version SMALLINT UNSIGNED NULL,
    credentials_set_at DATETIME(6) NULL,
    last_validation_succeeded_at DATETIME(6) NULL,
    last_collection_finished_at DATETIME(6) NULL,
    last_error_class VARCHAR(64) NULL,
    last_error_detail VARCHAR(1024) NULL,
    version BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    CONSTRAINT chk_identities_enabled CHECK (connection_status <> 'enabled' OR (platform_login_ref IS NOT NULL AND OCTET_LENGTH(platform_login_ref) > 0 AND archived_at IS NULL)),
    PRIMARY KEY (portfolio_id, id),
    UNIQUE KEY uq_identities_active_name (portfolio_id, platform, active_name_key),
    UNIQUE KEY uq_identities_active_login (portfolio_id, platform, active_login_ref),
    CONSTRAINT chk_identities_connection_status CHECK (connection_status IN ('unconfigured', 'disabled', 'enabled', 'invalid')),
    CONSTRAINT chk_identities_credentials CHECK ((credentials_ciphertext IS NULL AND credentials_nonce IS NULL AND credentials_key_version IS NULL AND credentials_set_at IS NULL) OR (credentials_ciphertext IS NOT NULL AND credentials_nonce IS NOT NULL AND credentials_key_version IS NOT NULL AND credentials_key_version >= 1 AND credentials_set_at IS NOT NULL)),
    CONSTRAINT chk_identities_configured CHECK ((connection_status = 'unconfigured') = (credentials_ciphertext IS NULL)),
    CONSTRAINT chk_identities_platform_login_ref_bytes CHECK (OCTET_LENGTH(platform_login_ref) BETWEEN 1 AND 255),
    CONSTRAINT chk_identities_version CHECK (version >= 1),
    CONSTRAINT fk_identities_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE accounts (
    portfolio_id BINARY(16) NOT NULL,
    id BINARY(16) NOT NULL,
    identity_id BINARY(16) NOT NULL,
    account_type VARCHAR(64) NOT NULL,
    selected BOOLEAN NOT NULL,
    CONSTRAINT chk_accounts_selected CHECK (selected IN (0, 1)),
    PRIMARY KEY (portfolio_id, id),
    UNIQUE KEY uq_accounts_identity_type (portfolio_id, identity_id, account_type),
    CONSTRAINT fk_accounts_identity FOREIGN KEY (portfolio_id, identity_id) REFERENCES identities (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT chk_accounts_manual_selected CHECK (account_type <> 'manual' OR selected)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE assets (
    portfolio_id BINARY(16) NOT NULL,
    id BINARY(16) NOT NULL,
    platform VARCHAR(32) NOT NULL,
    symbol VARCHAR(64) NOT NULL,
    canonical_asset_symbol VARCHAR(64) NOT NULL,
    mapping_version BIGINT UNSIGNED NOT NULL,
    first_observed_at DATETIME(6) NOT NULL,
    last_observed_at DATETIME(6) NOT NULL,
    CONSTRAINT fk_assets_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    PRIMARY KEY (portfolio_id, id),
    UNIQUE KEY uq_assets_platform_symbol (portfolio_id, platform, symbol),
    CONSTRAINT chk_assets_symbol_bytes CHECK (OCTET_LENGTH(symbol) BETWEEN 1 AND 64),
    CONSTRAINT chk_assets_canonical_asset_symbol_bytes CHECK (OCTET_LENGTH(canonical_asset_symbol) BETWEEN 1 AND 64),
    CONSTRAINT chk_assets_mapping_version CHECK (mapping_version >= 1),
    CONSTRAINT chk_assets_observation_order CHECK (first_observed_at <= last_observed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

-- Each row is one symbol with one immutable set of observed terms. Changed
-- terms add a row under the same symbol; referenced history is retained.
CREATE TABLE futures_contracts (
    portfolio_id BINARY(16) NOT NULL,
    id BINARY(16) NOT NULL,
    platform VARCHAR(32) NOT NULL,
    symbol VARCHAR(64) NOT NULL,
    terms_hash BINARY(32) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    base_asset_symbol VARCHAR(64) NULL,
    quote_asset_symbol VARCHAR(64) NULL,
    settlement_asset_symbol VARCHAR(64) NULL,
    expiry_at DATETIME(6) NULL,
    contract_size DECIMAL(65,30) NULL,
    first_observed_at DATETIME(6) NOT NULL,
    last_observed_at DATETIME(6) NOT NULL,
    PRIMARY KEY (portfolio_id, id),
    UNIQUE KEY uq_futures_contracts_terms (portfolio_id, platform, symbol, terms_hash),
    CONSTRAINT fk_futures_contracts_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT chk_futures_contracts_symbol_bytes CHECK (OCTET_LENGTH(symbol) BETWEEN 1 AND 64),
    CONSTRAINT chk_futures_contracts_delivery_expiry CHECK ((kind IN ('linear_delivery', 'inverse_delivery') AND expiry_at IS NOT NULL) OR (kind IN ('linear_perpetual', 'inverse_perpetual') AND expiry_at IS NULL)),
    CONSTRAINT chk_futures_contracts_contract_size CHECK (contract_size > 0),
    CONSTRAINT chk_futures_contracts_observation_order CHECK (first_observed_at <= last_observed_at),
    CONSTRAINT chk_futures_contracts_base_asset_symbol_bytes CHECK (OCTET_LENGTH(base_asset_symbol) BETWEEN 1 AND 64),
    CONSTRAINT chk_futures_contracts_quote_asset_symbol_bytes CHECK (OCTET_LENGTH(quote_asset_symbol) BETWEEN 1 AND 64),
    CONSTRAINT chk_futures_contracts_settlement_asset_symbol_bytes CHECK (OCTET_LENGTH(settlement_asset_symbol) BETWEEN 1 AND 64)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE price_observations (
    portfolio_id BINARY(16) NOT NULL,
    id BINARY(16) NOT NULL,
    platform VARCHAR(32) NOT NULL,
    requested_canonical_asset_symbol VARCHAR(64) NOT NULL,
    source_market_ref VARCHAR(255) NOT NULL,
    quote_canonical_asset_symbol VARCHAR(64) NOT NULL,
    price DECIMAL(65,30) NOT NULL,
    retrieved_at DATETIME(6) NOT NULL,
    source_observed_at DATETIME(6) NULL,
    daily_anchor BOOLEAN NOT NULL DEFAULT FALSE,
    price_at DATETIME(6) AS (COALESCE(source_observed_at, retrieved_at)) STORED,
    CONSTRAINT fk_price_observations_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT chk_price_observations_daily_anchor CHECK (daily_anchor IN (0, 1)),
    PRIMARY KEY (portfolio_id, id),
    KEY idx_price_observations_retention (portfolio_id, daily_anchor, retrieved_at, id),
    KEY idx_price_observations_selection (portfolio_id, platform, requested_canonical_asset_symbol, price_at, retrieved_at, id),
    CONSTRAINT chk_price_observations_positive CHECK (price > 0),
    CONSTRAINT chk_price_observations_usd_quote CHECK (quote_canonical_asset_symbol IN ('USD', 'USDT', 'USDC')),
    CONSTRAINT chk_price_observations_source_market_ref_bytes CHECK (OCTET_LENGTH(source_market_ref) BETWEEN 1 AND 255),
    CONSTRAINT chk_price_observations_requested_canonical_asset_symbol_bytes CHECK (OCTET_LENGTH(requested_canonical_asset_symbol) BETWEEN 1 AND 64)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE collection_cycles (
    portfolio_id BINARY(16) NOT NULL,
    id BINARY(16) NOT NULL,
    trigger_kind VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    active_portfolio_id BINARY(16) AS (IF(status IN ('queued', 'running'), portfolio_id, NULL)) STORED,
    attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
    available_at DATETIME(6) NOT NULL,
    lease_owner VARCHAR(191) NULL,
    lease_expires_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL,
    started_at DATETIME(6) NULL,
    collection_finished_at DATETIME(6) NULL,
    manual_reported_at DATETIME(6) NULL,
    finished_at DATETIME(6) NULL,
    error_class VARCHAR(64) NULL,
    error_detail VARCHAR(1024) NULL,
    PRIMARY KEY (portfolio_id, id),
    UNIQUE KEY uq_collection_cycles_active (active_portfolio_id),
    KEY idx_collection_cycles_timeline (portfolio_id, created_at, id),
    CONSTRAINT fk_collection_cycles_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT chk_collection_cycles_trigger_kind CHECK (trigger_kind IN ('schedule', 'user')),
    CONSTRAINT chk_collection_cycles_status CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    CONSTRAINT chk_collection_cycles_lease CHECK ((status = 'running') = (lease_owner IS NOT NULL) AND (lease_owner IS NULL) = (lease_expires_at IS NULL)),
    CONSTRAINT chk_collection_cycles_running CHECK (status <> 'running' OR (attempt_count >= 1 AND started_at IS NOT NULL)),
    CONSTRAINT chk_collection_cycles_terminal CHECK ((status IN ('succeeded', 'failed')) = (finished_at IS NOT NULL)),
    CONSTRAINT chk_collection_cycles_success CHECK (status <> 'succeeded' OR (collection_finished_at IS NOT NULL AND error_class IS NULL AND error_detail IS NULL))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE manual_revisions (
    portfolio_id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    revision_number BIGINT UNSIGNED NOT NULL,
    changed_at DATETIME(6) NULL,
    CONSTRAINT chk_manual_revisions_changed CHECK (revision_number >= 1 AND (revision_number = 1) = (changed_at IS NULL)),
    PRIMARY KEY (portfolio_id, account_id, revision_number),
    CONSTRAINT fk_manual_revisions_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

-- A Manual report references the immutable revision whose quantities it copies.
CREATE TABLE account_reports (
    portfolio_id BINARY(16) NOT NULL,
    id BINARY(16) NOT NULL,
    cycle_id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    status VARCHAR(32) NOT NULL,
    collected_at DATETIME(6) NULL,
    source_observed_at DATETIME(6) NULL,
    recorded_at DATETIME(6) NOT NULL,
    margin_mode VARCHAR(64) NULL,
    error_class VARCHAR(64) NULL,
    error_detail VARCHAR(1024) NULL,
    reported_equity_usd DECIMAL(65,30) NULL,
    adjusted_equity_usd DECIMAL(65,30) NULL,
    available_equity_usd DECIMAL(65,30) NULL,
    initial_margin_requirement_usd DECIMAL(65,30) NULL,
    maintenance_margin_requirement_usd DECIMAL(65,30) NULL,
    reported_initial_margin_ratio DECIMAL(65,30) NULL,
    reported_maintenance_margin_ratio DECIMAL(65,30) NULL,
    reported_margin_ratio DECIMAL(65,30) NULL,
    spot_margin_unrealized_pnl_usd DECIMAL(65,30) NULL,
    manual_revision_number BIGINT UNSIGNED NULL,
    CONSTRAINT chk_account_reports_failed_measures CHECK (status <> 'failed' OR (reported_equity_usd IS NULL AND adjusted_equity_usd IS NULL AND available_equity_usd IS NULL AND initial_margin_requirement_usd IS NULL AND maintenance_margin_requirement_usd IS NULL AND reported_initial_margin_ratio IS NULL AND reported_maintenance_margin_ratio IS NULL AND reported_margin_ratio IS NULL AND spot_margin_unrealized_pnl_usd IS NULL)),
    PRIMARY KEY (portfolio_id, id),
    UNIQUE KEY uq_account_reports_cycle_account (portfolio_id, cycle_id, account_id),
    UNIQUE KEY uq_account_reports_account_report (portfolio_id, account_id, id),
    KEY idx_account_reports_retention (portfolio_id, recorded_at, id),
    KEY idx_account_reports_fallback (portfolio_id, account_id, status, recorded_at, id),
    KEY idx_account_reports_manual_revision (portfolio_id, account_id, manual_revision_number),
    CONSTRAINT fk_account_reports_cycle FOREIGN KEY (portfolio_id, cycle_id) REFERENCES collection_cycles (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_account_reports_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_account_reports_manual_revision FOREIGN KEY (portfolio_id, account_id, manual_revision_number) REFERENCES manual_revisions (portfolio_id, account_id, revision_number) ON DELETE RESTRICT,
    CONSTRAINT chk_account_reports_manual_revision CHECK (manual_revision_number IS NULL OR status = 'succeeded'),
    CONSTRAINT chk_account_reports_observation CHECK ((status = 'succeeded' AND collected_at IS NOT NULL AND error_class IS NULL AND error_detail IS NULL) OR (status = 'failed' AND collected_at IS NULL AND source_observed_at IS NULL)),
    CONSTRAINT chk_account_reports_error CHECK (status <> 'failed' OR error_class IS NOT NULL)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE balances (
    portfolio_id BINARY(16) NOT NULL,
    report_id BINARY(16) NOT NULL,
    asset_id BINARY(16) NOT NULL,
    equity DECIMAL(65,30) NOT NULL,
    available_to_withdraw DECIMAL(65,30) NULL,
    available_to_trade DECIMAL(65,30) NULL,
    liability DECIMAL(65,30) NULL,
    interest DECIMAL(65,30) NULL,
    unrealized_pnl DECIMAL(65,30) NULL,
    PRIMARY KEY (portfolio_id, report_id, asset_id),
    KEY idx_balances_asset (portfolio_id, asset_id, report_id),
    CONSTRAINT fk_balances_report FOREIGN KEY (portfolio_id, report_id) REFERENCES account_reports (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_balances_asset FOREIGN KEY (portfolio_id, asset_id) REFERENCES assets (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE futures_positions (
    portfolio_id BINARY(16) NOT NULL,
    report_id BINARY(16) NOT NULL,
    position_key VARCHAR(255) NOT NULL,
    futures_contract_id BINARY(16) NOT NULL,
    side VARCHAR(16) NOT NULL,
    position_mode VARCHAR(32) NOT NULL,
    source_quantity DECIMAL(65,30) NOT NULL,
    source_quantity_unit VARCHAR(16) NOT NULL,
    entry_price DECIMAL(65,30) NULL,
    mark_price DECIMAL(65,30) NULL,
    liquidation_price DECIMAL(65,30) NULL,
    unrealized_pnl DECIMAL(65,30) NULL,
    leverage DECIMAL(65,30) NULL,
    margin DECIMAL(65,30) NULL,
    margin_setting VARCHAR(32) NULL,
    margin_asset_symbol VARCHAR(64) NULL,
    margin_basis VARCHAR(32) NULL,
    CONSTRAINT chk_futures_positions_source_quantity CHECK (source_quantity > 0),
    PRIMARY KEY (portfolio_id, report_id, position_key),
    KEY idx_futures_positions_contract (portfolio_id, futures_contract_id, report_id),
    CONSTRAINT fk_futures_positions_report FOREIGN KEY (portfolio_id, report_id) REFERENCES account_reports (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_futures_positions_contract FOREIGN KEY (portfolio_id, futures_contract_id) REFERENCES futures_contracts (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT chk_futures_positions_position_key_bytes CHECK (OCTET_LENGTH(position_key) BETWEEN 1 AND 255),
    CONSTRAINT chk_futures_positions_margin_asset_symbol_bytes CHECK (OCTET_LENGTH(margin_asset_symbol) BETWEEN 1 AND 64),
    CONSTRAINT chk_futures_positions_side CHECK (side IN ('long', 'short')),
    CONSTRAINT chk_futures_positions_source_quantity_unit CHECK (source_quantity_unit IN ('contracts', 'base', 'quote')),
    CONSTRAINT chk_futures_positions_position_mode CHECK (position_mode IN ('one_way', 'hedged')),
    CONSTRAINT chk_futures_positions_margin_setting CHECK (margin_setting IN ('cross', 'isolated')),
    CONSTRAINT chk_futures_positions_margin_basis CHECK (margin_basis IN ('initial', 'allocated'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE earn_positions (
    portfolio_id BINARY(16) NOT NULL,
    report_id BINARY(16) NOT NULL,
    position_key VARCHAR(255) NOT NULL,
    product_ref VARCHAR(255) NULL,
    product_name VARCHAR(255) NULL,
    product_type VARCHAR(32) NOT NULL,
    granularity VARCHAR(32) NOT NULL,
    amount_basis VARCHAR(32) NOT NULL,
    status VARCHAR(64) NULL,
    subscribed_at DATETIME(6) NULL,
    maturity_at DATETIME(6) NULL,
    details_json JSON NOT NULL,
    PRIMARY KEY (portfolio_id, report_id, position_key),
    CONSTRAINT fk_earn_positions_report FOREIGN KEY (portfolio_id, report_id) REFERENCES account_reports (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT chk_earn_positions_position_key_bytes CHECK (OCTET_LENGTH(position_key) BETWEEN 1 AND 255),
    CONSTRAINT chk_earn_positions_product_ref_bytes CHECK (OCTET_LENGTH(product_ref) BETWEEN 1 AND 255),
    CONSTRAINT chk_earn_positions_product_type CHECK (product_type IN ('flexible', 'fixed', 'bonded', 'timed', 'on_chain')),
    CONSTRAINT chk_earn_positions_granularity CHECK (granularity IN ('product', 'position', 'strategy')),
    CONSTRAINT chk_earn_positions_amount_basis CHECK (amount_basis IN ('principal', 'total', 'allocated')),
    CONSTRAINT chk_earn_positions_details_object CHECK (JSON_TYPE(details_json) = 'OBJECT')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE dual_positions (
    portfolio_id BINARY(16) NOT NULL,
    report_id BINARY(16) NOT NULL,
    position_key VARCHAR(255) NOT NULL,
    product_ref VARCHAR(255) NULL,
    order_ref VARCHAR(255) NULL,
    base_asset_symbol VARCHAR(64) NULL,
    quote_asset_symbol VARCHAR(64) NULL,
    principal_asset_symbol VARCHAR(64) NOT NULL,
    principal DECIMAL(65,30) NOT NULL,
    direction VARCHAR(32) NULL,
    target_price DECIMAL(65,30) NULL,
    expiry_at DATETIME(6) NULL,
    annualized_rate DECIMAL(65,30) NULL,
    rate_kind VARCHAR(32) NULL,
    status VARCHAR(64) NOT NULL,
    subscribed_at DATETIME(6) NULL,
    PRIMARY KEY (portfolio_id, report_id, position_key),
    CONSTRAINT fk_dual_positions_report FOREIGN KEY (portfolio_id, report_id) REFERENCES account_reports (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT chk_dual_positions_principal CHECK (principal > 0),
    CONSTRAINT chk_dual_positions_target_price CHECK (target_price > 0),
    CONSTRAINT chk_dual_positions_rate CHECK (annualized_rate >= 0 AND ((annualized_rate IS NULL) = (rate_kind IS NULL))),
    CONSTRAINT chk_dual_positions_status CHECK (status IN ('pending', 'active', 'settling', 'redeeming', 'refunding')),
    CONSTRAINT chk_dual_positions_pair CHECK (base_asset_symbol IS NULL OR quote_asset_symbol IS NULL OR (base_asset_symbol <> quote_asset_symbol AND principal_asset_symbol IN (base_asset_symbol, quote_asset_symbol))),
    CONSTRAINT chk_dual_positions_principal_direction CHECK ((direction <> 'buy_low' OR quote_asset_symbol IS NULL OR principal_asset_symbol = quote_asset_symbol) AND (direction <> 'sell_high' OR base_asset_symbol IS NULL OR principal_asset_symbol = base_asset_symbol)),
    CONSTRAINT chk_dual_positions_direction CHECK (direction IN ('buy_low', 'sell_high')),
    CONSTRAINT chk_dual_positions_rate_kind CHECK (rate_kind IN ('apr', 'apy', 'annualized_yield')),
    CONSTRAINT chk_dual_positions_position_key_bytes CHECK (OCTET_LENGTH(position_key) BETWEEN 1 AND 255),
    CONSTRAINT chk_dual_positions_product_ref_bytes CHECK (OCTET_LENGTH(product_ref) BETWEEN 1 AND 255),
    CONSTRAINT chk_dual_positions_order_ref_bytes CHECK (OCTET_LENGTH(order_ref) BETWEEN 1 AND 255),
    CONSTRAINT chk_dual_positions_base_asset_symbol_bytes CHECK (OCTET_LENGTH(base_asset_symbol) BETWEEN 1 AND 64),
    CONSTRAINT chk_dual_positions_quote_asset_symbol_bytes CHECK (OCTET_LENGTH(quote_asset_symbol) BETWEEN 1 AND 64),
    CONSTRAINT chk_dual_positions_principal_asset_symbol_bytes CHECK (OCTET_LENGTH(principal_asset_symbol) BETWEEN 1 AND 64)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE valuation_points (
    portfolio_id BINARY(16) NOT NULL,
    cycle_id BINARY(16) NOT NULL,
    evaluated_at DATETIME(6) NOT NULL,
    total_usd DECIMAL(65,30) NOT NULL,
    account_count INT UNSIGNED NOT NULL,
    stale_account_count INT UNSIGNED NOT NULL,
    incomplete_account_count INT UNSIGNED NOT NULL,
    calculation_version BIGINT UNSIGNED NOT NULL,
    detail_state VARCHAR(16) NOT NULL DEFAULT 'full',
    PRIMARY KEY (portfolio_id, cycle_id),
    KEY idx_valuation_points_timeline (portfolio_id, evaluated_at, cycle_id, calculation_version),
    KEY idx_valuation_points_retention (portfolio_id, detail_state, evaluated_at, cycle_id),
    CONSTRAINT fk_valuation_points_cycle FOREIGN KEY (portfolio_id, cycle_id) REFERENCES collection_cycles (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT chk_valuation_points_counts CHECK (stale_account_count <= account_count AND incomplete_account_count <= account_count),
    CONSTRAINT chk_valuation_points_versions CHECK (calculation_version >= 1),
    CONSTRAINT chk_valuation_points_detail_state CHECK (detail_state IN ('full', 'daily', 'pruning', 'expired'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE account_valuations (
    portfolio_id BINARY(16) NOT NULL,
    cycle_id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    source_report_id BINARY(16) NULL,
    valuation_method VARCHAR(32) NOT NULL,
    value_usd DECIMAL(65,30) NULL,
    PRIMARY KEY (portfolio_id, cycle_id, account_id),
    UNIQUE KEY uq_account_valuations_source (portfolio_id, cycle_id, account_id, source_report_id),
    KEY idx_account_valuations_source_report (portfolio_id, account_id, source_report_id),
    CONSTRAINT fk_account_valuations_point FOREIGN KEY (portfolio_id, cycle_id) REFERENCES valuation_points (portfolio_id, cycle_id) ON DELETE RESTRICT,
    CONSTRAINT fk_account_valuations_current_report FOREIGN KEY (portfolio_id, cycle_id, account_id) REFERENCES account_reports (portfolio_id, cycle_id, account_id) ON DELETE RESTRICT,
    CONSTRAINT fk_account_valuations_source_report FOREIGN KEY (portfolio_id, account_id, source_report_id) REFERENCES account_reports (portfolio_id, account_id, id) ON DELETE RESTRICT,
    CONSTRAINT chk_account_valuations_valuation_method CHECK (valuation_method IN ('reported', 'calculated', 'missing')),
    CONSTRAINT chk_account_valuations_value CHECK ((valuation_method = 'missing' AND source_report_id IS NULL AND value_usd IS NULL) OR (valuation_method <> 'missing' AND source_report_id IS NOT NULL AND value_usd IS NOT NULL))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE asset_exclusion_versions (
    portfolio_id BINARY(16) NOT NULL,
    asset_id BINARY(16) NOT NULL,
    version BIGINT UNSIGNED NOT NULL,
    excluded BOOLEAN NOT NULL,
    reason VARCHAR(500) NULL,
    effective_at DATETIME(6) NOT NULL,
    CONSTRAINT chk_asset_exclusion_versions_excluded CHECK (excluded IN (0, 1)),
    PRIMARY KEY (portfolio_id, asset_id, version),
    KEY idx_asset_exclusion_versions_effective (portfolio_id, effective_at),
    CONSTRAINT fk_asset_exclusion_versions_asset FOREIGN KEY (portfolio_id, asset_id) REFERENCES assets (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT chk_asset_exclusion_versions_version CHECK (version >= 1),
    CONSTRAINT chk_asset_exclusion_versions_reason CHECK ((excluded AND reason IS NOT NULL AND CHAR_LENGTH(TRIM(reason)) > 0) OR (NOT excluded AND reason IS NULL))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE capital_flows (
    portfolio_id BINARY(16) NOT NULL,
    id BINARY(16) NOT NULL,
    account_id BINARY(16) NULL,
    direction VARCHAR(32) NOT NULL,
    canonical_asset_symbol VARCHAR(64) NOT NULL,
    quantity DECIMAL(65,30) NOT NULL,
    value_usd DECIMAL(65,30) NULL,
    pricing_method VARCHAR(32) NOT NULL,
    pricing_platform VARCHAR(32) NULL,
    price_observation_id BINARY(16) NULL,
    pricing_attempted_at DATETIME(6) NOT NULL,
    calculation_version BIGINT UNSIGNED NOT NULL,
    effective_at DATETIME(6) NOT NULL,
    note VARCHAR(500) NULL,
    version BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    CONSTRAINT chk_capital_flows_parity CHECK (pricing_method <> 'usd_parity' OR (canonical_asset_symbol IN ('USD', 'USDT', 'USDC') AND value_usd = quantity)),
    PRIMARY KEY (portfolio_id, id),
    KEY idx_capital_flows_account (portfolio_id, account_id),
    KEY idx_capital_flows_price (portfolio_id, price_observation_id),
    KEY idx_capital_flows_timeline (portfolio_id, effective_at, id),
    CONSTRAINT fk_capital_flows_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT fk_capital_flows_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_capital_flows_price FOREIGN KEY (portfolio_id, price_observation_id) REFERENCES price_observations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT chk_capital_flows_direction CHECK (direction IN ('contribution', 'withdrawal')),
    CONSTRAINT chk_capital_flows_quantity CHECK (quantity > 0),
    CONSTRAINT chk_capital_flows_value CHECK (value_usd >= 0),
    CONSTRAINT chk_capital_flows_version CHECK (version >= 1 AND calculation_version >= 1),
    CONSTRAINT chk_capital_flows_pricing CHECK ((pricing_method IN ('explicit', 'usd_parity') AND value_usd IS NOT NULL AND price_observation_id IS NULL) OR (pricing_method = 'market_price' AND value_usd IS NOT NULL AND price_observation_id IS NOT NULL AND pricing_platform IS NOT NULL) OR (pricing_method = 'unavailable' AND value_usd IS NULL AND price_observation_id IS NULL)),
    CONSTRAINT chk_capital_flows_canonical_asset_symbol_bytes CHECK (OCTET_LENGTH(canonical_asset_symbol) BETWEEN 1 AND 64)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE audit_events (
    portfolio_id BINARY(16) NOT NULL,
    id BINARY(16) NOT NULL,
    actor_user_id BINARY(16) NULL,
    action VARCHAR(64) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id BINARY(16) NOT NULL,
    request_id VARCHAR(64) NULL,
    before_json JSON NULL,
    after_json JSON NULL,
    metadata_json JSON NULL,
    occurred_at DATETIME(6) NOT NULL,
    KEY idx_audit_events_actor (actor_user_id),
    PRIMARY KEY (portfolio_id, id),
    KEY idx_audit_events_timeline (portfolio_id, occurred_at, id),
    CONSTRAINT fk_audit_events_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT fk_audit_events_actor FOREIGN KEY (actor_user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE manual_revision_balances (
    portfolio_id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    revision_number BIGINT UNSIGNED NOT NULL,
    asset_id BINARY(16) NOT NULL,
    quantity DECIMAL(65,30) NOT NULL,
    changed_at DATETIME(6) NOT NULL,
    PRIMARY KEY (portfolio_id, account_id, revision_number, asset_id),
    KEY idx_manual_revision_balances_asset (portfolio_id, asset_id),
    CONSTRAINT fk_manual_revision_balances_revision FOREIGN KEY (portfolio_id, account_id, revision_number) REFERENCES manual_revisions (portfolio_id, account_id, revision_number) ON DELETE RESTRICT,
    CONSTRAINT fk_manual_revision_balances_asset FOREIGN KEY (portfolio_id, asset_id) REFERENCES assets (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT chk_manual_revision_balances_revision CHECK (revision_number > 1),
    CONSTRAINT chk_manual_revision_balances_quantity CHECK (quantity <> 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

-- source_report_id is fixed by (cycle_id, account_id); keeping it in the key
-- lets the primary key back the Account Valuation reference.
CREATE TABLE balance_valuations (
    portfolio_id BINARY(16) NOT NULL,
    cycle_id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    source_report_id BINARY(16) NOT NULL,
    asset_id BINARY(16) NOT NULL,
    canonical_asset_symbol VARCHAR(64) NOT NULL,
    pricing_method VARCHAR(32) NOT NULL,
    price_observation_id BINARY(16) NULL,
    value_usd DECIMAL(65,30) NULL,
    PRIMARY KEY (portfolio_id, cycle_id, account_id, source_report_id, asset_id),
    KEY idx_balance_valuations_balance (portfolio_id, source_report_id, asset_id),
    KEY idx_balance_valuations_price (portfolio_id, price_observation_id),
    CONSTRAINT fk_balance_valuations_account FOREIGN KEY (portfolio_id, cycle_id, account_id, source_report_id) REFERENCES account_valuations (portfolio_id, cycle_id, account_id, source_report_id) ON DELETE RESTRICT,
    CONSTRAINT fk_balance_valuations_balance FOREIGN KEY (portfolio_id, source_report_id, asset_id) REFERENCES balances (portfolio_id, report_id, asset_id) ON DELETE RESTRICT,
    CONSTRAINT fk_balance_valuations_price FOREIGN KEY (portfolio_id, price_observation_id) REFERENCES price_observations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT chk_balance_valuations_symbol CHECK (OCTET_LENGTH(canonical_asset_symbol) BETWEEN 1 AND 64),
    CONSTRAINT chk_balance_valuations_pricing CHECK (
        (pricing_method = 'market_price' AND price_observation_id IS NOT NULL AND value_usd IS NOT NULL)
        OR (pricing_method = 'usd_parity' AND price_observation_id IS NULL AND value_usd IS NOT NULL AND canonical_asset_symbol IN ('USD', 'USDT', 'USDC'))
        OR (pricing_method = 'zero_equity' AND price_observation_id IS NULL AND value_usd IS NOT NULL AND value_usd = 0)
        OR (pricing_method IN ('unavailable', 'excluded') AND price_observation_id IS NULL AND value_usd IS NULL)
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE boundary_events (
    portfolio_id BINARY(16) NOT NULL,
    cycle_id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    direction VARCHAR(32) NOT NULL,
    valuation_method VARCHAR(32) NOT NULL,
    value_usd DECIMAL(65,30) NULL,
    valuation_complete BOOLEAN NOT NULL,
    PRIMARY KEY (portfolio_id, cycle_id, account_id),
    KEY idx_boundary_events_account (portfolio_id, account_id),
    CONSTRAINT fk_boundary_events_point FOREIGN KEY (portfolio_id, cycle_id) REFERENCES valuation_points (portfolio_id, cycle_id) ON DELETE RESTRICT,
    CONSTRAINT fk_boundary_events_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT chk_boundary_events_complete CHECK (valuation_complete IN (0, 1) AND (NOT valuation_complete OR value_usd IS NOT NULL)),
    CONSTRAINT chk_boundary_events_direction CHECK (direction IN ('contribution', 'withdrawal')),
    CONSTRAINT chk_boundary_events_value CHECK ((valuation_method = 'missing' AND value_usd IS NULL) OR (valuation_method IN ('reported', 'calculated') AND value_usd IS NOT NULL))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

-- Manual adjustments compare adopted quantities that expire with point detail,
-- so they are frozen at first commit. Other comparison markers are derived.
CREATE TABLE manual_balance_changes (
    portfolio_id BINARY(16) NOT NULL,
    cycle_id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    PRIMARY KEY (portfolio_id, cycle_id, account_id),
    KEY idx_manual_balance_changes_account (portfolio_id, account_id),
    CONSTRAINT fk_manual_balance_changes_point FOREIGN KEY (portfolio_id, cycle_id) REFERENCES valuation_points (portfolio_id, cycle_id) ON DELETE RESTRICT,
    CONSTRAINT fk_manual_balance_changes_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

-- Rows are sparse: one is appended only when a nonzero calculated Asset's
-- treatment or adopted mapping changes.
CREATE TABLE calculated_asset_changes (
    portfolio_id BINARY(16) NOT NULL,
    asset_id BINARY(16) NOT NULL,
    evaluated_at DATETIME(6) NOT NULL,
    cycle_id BINARY(16) NOT NULL,
    treatment VARCHAR(16) NOT NULL,
    canonical_asset_symbol VARCHAR(64) NULL,
    PRIMARY KEY (portfolio_id, asset_id, evaluated_at, cycle_id),
    KEY idx_calculated_asset_changes_point (portfolio_id, cycle_id),
    CONSTRAINT fk_calculated_asset_changes_asset FOREIGN KEY (portfolio_id, asset_id) REFERENCES assets (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_calculated_asset_changes_point FOREIGN KEY (portfolio_id, cycle_id) REFERENCES valuation_points (portfolio_id, cycle_id) ON DELETE RESTRICT,
    CONSTRAINT chk_calculated_asset_changes_treatment CHECK (treatment IN ('absent', 'excluded', 'included') AND (treatment = 'included') = (canonical_asset_symbol IS NOT NULL)),
    CONSTRAINT chk_calculated_asset_changes_symbol CHECK (OCTET_LENGTH(canonical_asset_symbol) BETWEEN 1 AND 64)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

-- +goose Down
DROP TABLE IF EXISTS calculated_asset_changes;
DROP TABLE IF EXISTS manual_balance_changes;
DROP TABLE IF EXISTS boundary_events;
DROP TABLE IF EXISTS balance_valuations;
DROP TABLE IF EXISTS manual_revision_balances;
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS capital_flows;
DROP TABLE IF EXISTS asset_exclusion_versions;
DROP TABLE IF EXISTS account_valuations;
DROP TABLE IF EXISTS valuation_points;
DROP TABLE IF EXISTS dual_positions;
DROP TABLE IF EXISTS earn_positions;
DROP TABLE IF EXISTS futures_positions;
DROP TABLE IF EXISTS balances;
DROP TABLE IF EXISTS account_reports;
DROP TABLE IF EXISTS manual_revisions;
DROP TABLE IF EXISTS collection_cycles;
DROP TABLE IF EXISTS price_observations;
DROP TABLE IF EXISTS futures_contracts;
DROP TABLE IF EXISTS assets;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS identities;
DROP TABLE IF EXISTS portfolios;
DROP TABLE IF EXISTS idempotency_records;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
