-- +goose Up
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

-- +goose Down
DROP TABLE IF EXISTS price_observations;
DROP TABLE IF EXISTS account_metric_observations;
DROP TABLE IF EXISTS reported_equity_covered_accounts;
DROP TABLE IF EXISTS reported_equity_covered_identities;
DROP TABLE IF EXISTS reported_equity_observations;
DROP TABLE IF EXISTS position_observations;
DROP TABLE IF EXISTS balance_observations;
DROP TABLE IF EXISTS observations;
