-- +goose Up
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

-- +goose Down
ALTER TABLE catalog_pins DROP FOREIGN KEY fk_catalog_pins_current_revision;
ALTER TABLE asset_listing_mappings DROP FOREIGN KEY fk_alm_current_revision;
DROP TABLE IF EXISTS catalog_pin_revisions;
DROP TABLE IF EXISTS catalog_pins;
DROP TABLE IF EXISTS asset_listing_mapping_revisions;
DROP TABLE IF EXISTS asset_listing_mappings;
