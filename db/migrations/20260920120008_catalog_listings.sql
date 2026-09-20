-- +goose Up
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

-- +goose Down
DROP TABLE IF EXISTS instruments;
DROP TABLE IF EXISTS asset_listings;
