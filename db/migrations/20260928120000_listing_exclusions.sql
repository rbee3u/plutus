-- +goose Up
CREATE TABLE portfolio_listing_exclusions (
    portfolio_id BINARY(16) NOT NULL,
    listing_id BINARY(16) NOT NULL,
    ignored BOOLEAN NOT NULL,
    reason VARCHAR(500) NULL,
    version BIGINT UNSIGNED NOT NULL,
    actor_user_id BINARY(16) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (portfolio_id, listing_id),
    KEY idx_listing_exclusions_listing (listing_id),
    CONSTRAINT fk_listing_exclusions_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE CASCADE,
    CONSTRAINT fk_listing_exclusions_listing FOREIGN KEY (listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT,
    CONSTRAINT fk_listing_exclusions_actor FOREIGN KEY (actor_user_id) REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT chk_listing_exclusions_reason CHECK (ignored = FALSE OR (reason IS NOT NULL AND CHAR_LENGTH(TRIM(reason)) > 0))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE portfolio_listing_exclusion_events (
    portfolio_id BINARY(16) NOT NULL,
    listing_id BINARY(16) NOT NULL,
    version BIGINT UNSIGNED NOT NULL,
    ignored BOOLEAN NOT NULL,
    reason VARCHAR(500) NULL,
    actor_user_id BINARY(16) NOT NULL,
    effective_at DATETIME(6) NOT NULL,
    PRIMARY KEY (portfolio_id, listing_id, version),
    KEY idx_listing_exclusion_events_time (portfolio_id, effective_at),
    CONSTRAINT fk_listing_exclusion_events_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE CASCADE,
    CONSTRAINT fk_listing_exclusion_events_listing FOREIGN KEY (listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT,
    CONSTRAINT fk_listing_exclusion_events_actor FOREIGN KEY (actor_user_id) REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT chk_listing_exclusion_events_reason CHECK (ignored = FALSE OR (reason IS NOT NULL AND CHAR_LENGTH(TRIM(reason)) > 0))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS portfolio_listing_exclusion_events;
DROP TABLE IF EXISTS portfolio_listing_exclusions;
