-- +goose Up
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
    PRIMARY KEY (id),
    UNIQUE KEY uq_identities_portfolio_id (portfolio_id, id),
    UNIQUE KEY uq_identities_platform_ref (portfolio_id, platform_id, external_ref),
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

-- +goose Down
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS connections;
DROP TABLE IF EXISTS identities;
