-- +goose Up
-- mysql.Up checks for unsupported spot records before any pending migration.
RENAME TABLE portfolio_listing_exclusion_events TO portfolio_platform_asset_exclusion_events,
    portfolio_listing_exclusions TO portfolio_platform_asset_exclusions,
    asset_listings TO platform_assets,
    instruments TO futures_contracts;

ALTER TABLE platform_assets RENAME INDEX uq_asset_listings_platform_symbol TO uq_platform_assets_platform_symbol;

ALTER TABLE platform_assets RENAME INDEX idx_asset_listings_asset TO idx_platform_assets_asset;

ALTER TABLE platform_assets DROP CHECK chk_asset_listings_platform,
    ADD CONSTRAINT chk_platform_assets_platform CHECK (platform IN ('binance', 'okx', 'bybit', 'kraken'));

ALTER TABLE platform_assets DROP CHECK chk_asset_listings_source,
    ADD CONSTRAINT chk_platform_assets_source CHECK (mapping_source IN ('auto', 'manual'));

ALTER TABLE futures_contracts RENAME INDEX uq_instruments_platform_symbol TO uq_futures_contracts_platform_symbol;

ALTER TABLE futures_contracts DROP CHECK chk_instruments_platform,
    ADD CONSTRAINT chk_futures_contracts_platform CHECK (platform IN ('binance', 'okx', 'bybit', 'kraken'));

ALTER TABLE futures_contracts DROP CHECK chk_instruments_kind,
    ADD CONSTRAINT chk_futures_contracts_kind CHECK (kind IN ('linear_perpetual', 'inverse_perpetual', 'linear_delivery', 'inverse_delivery'));

ALTER TABLE futures_contracts DROP CHECK chk_instruments_delivery_expiry,
    ADD CONSTRAINT chk_futures_contracts_delivery_expiry CHECK (
        kind NOT IN ('linear_delivery', 'inverse_delivery') OR expiry_at IS NOT NULL
    );

ALTER TABLE balances RENAME COLUMN listing_id TO platform_asset_id;

ALTER TABLE balances RENAME INDEX idx_balances_listing TO idx_balances_platform_asset;

ALTER TABLE balances DROP FOREIGN KEY fk_balances_listing,
    ADD CONSTRAINT fk_balances_platform_asset FOREIGN KEY (platform_asset_id) REFERENCES platform_assets (id) ON DELETE RESTRICT;

ALTER TABLE positions RENAME COLUMN instrument_id TO futures_contract_id;

ALTER TABLE positions RENAME INDEX idx_positions_instrument TO idx_positions_futures_contract;

ALTER TABLE positions DROP FOREIGN KEY fk_positions_instrument,
    ADD CONSTRAINT fk_positions_futures_contract FOREIGN KEY (futures_contract_id) REFERENCES futures_contracts (id) ON DELETE RESTRICT;

ALTER TABLE portfolio_platform_asset_exclusions RENAME COLUMN listing_id TO platform_asset_id;

ALTER TABLE portfolio_platform_asset_exclusions RENAME INDEX idx_listing_exclusions_listing TO idx_platform_asset_exclusions_platform_asset;

ALTER TABLE portfolio_platform_asset_exclusions DROP FOREIGN KEY fk_listing_exclusions_portfolio,
    ADD CONSTRAINT fk_platform_asset_exclusions_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE CASCADE;

ALTER TABLE portfolio_platform_asset_exclusions DROP FOREIGN KEY fk_listing_exclusions_listing,
    ADD CONSTRAINT fk_platform_asset_exclusions_platform_asset FOREIGN KEY (platform_asset_id) REFERENCES platform_assets (id) ON DELETE RESTRICT;

ALTER TABLE portfolio_platform_asset_exclusions DROP FOREIGN KEY fk_listing_exclusions_actor,
    ADD CONSTRAINT fk_platform_asset_exclusions_actor FOREIGN KEY (actor_user_id) REFERENCES users (id) ON DELETE RESTRICT;

ALTER TABLE portfolio_platform_asset_exclusions DROP CHECK chk_listing_exclusions_reason,
    ADD CONSTRAINT chk_platform_asset_exclusions_reason CHECK (ignored = FALSE OR (reason IS NOT NULL AND CHAR_LENGTH(TRIM(reason)) > 0));

ALTER TABLE portfolio_platform_asset_exclusion_events RENAME COLUMN listing_id TO platform_asset_id;

ALTER TABLE portfolio_platform_asset_exclusion_events RENAME INDEX idx_listing_exclusion_events_time TO idx_platform_asset_exclusion_events_time;

ALTER TABLE portfolio_platform_asset_exclusion_events DROP FOREIGN KEY fk_listing_exclusion_events_portfolio,
    ADD CONSTRAINT fk_platform_asset_exclusion_events_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE CASCADE;

ALTER TABLE portfolio_platform_asset_exclusion_events DROP FOREIGN KEY fk_listing_exclusion_events_listing,
    ADD CONSTRAINT fk_platform_asset_exclusion_events_platform_asset FOREIGN KEY (platform_asset_id) REFERENCES platform_assets (id) ON DELETE RESTRICT;

ALTER TABLE portfolio_platform_asset_exclusion_events DROP FOREIGN KEY fk_listing_exclusion_events_actor,
    ADD CONSTRAINT fk_platform_asset_exclusion_events_actor FOREIGN KEY (actor_user_id) REFERENCES users (id) ON DELETE RESTRICT;

ALTER TABLE portfolio_platform_asset_exclusion_events DROP CHECK chk_listing_exclusion_events_reason,
    ADD CONSTRAINT chk_platform_asset_exclusion_events_reason CHECK (ignored = FALSE OR (reason IS NOT NULL AND CHAR_LENGTH(TRIM(reason)) > 0));

ALTER TABLE manual_balances RENAME COLUMN listing_id TO platform_asset_id;

ALTER TABLE manual_balances DROP FOREIGN KEY fk_manual_balances_listing,
    ADD CONSTRAINT fk_manual_balances_platform_asset FOREIGN KEY (platform_asset_id) REFERENCES platform_assets (id) ON DELETE RESTRICT;

-- +goose Down
RENAME TABLE portfolio_platform_asset_exclusion_events TO portfolio_listing_exclusion_events,
    portfolio_platform_asset_exclusions TO portfolio_listing_exclusions,
    platform_assets TO asset_listings,
    futures_contracts TO instruments;

ALTER TABLE asset_listings RENAME INDEX uq_platform_assets_platform_symbol TO uq_asset_listings_platform_symbol;

ALTER TABLE asset_listings RENAME INDEX idx_platform_assets_asset TO idx_asset_listings_asset;

ALTER TABLE asset_listings DROP CHECK chk_platform_assets_platform,
    ADD CONSTRAINT chk_asset_listings_platform CHECK (platform IN ('binance', 'okx', 'bybit', 'kraken'));

ALTER TABLE asset_listings DROP CHECK chk_platform_assets_source,
    ADD CONSTRAINT chk_asset_listings_source CHECK (mapping_source IN ('auto', 'manual'));

ALTER TABLE instruments RENAME INDEX uq_futures_contracts_platform_symbol TO uq_instruments_platform_symbol;

ALTER TABLE instruments DROP CHECK chk_futures_contracts_platform,
    ADD CONSTRAINT chk_instruments_platform CHECK (platform IN ('binance', 'okx', 'bybit', 'kraken'));

ALTER TABLE instruments DROP CHECK chk_futures_contracts_kind,
    ADD CONSTRAINT chk_instruments_kind CHECK (kind IN ('spot', 'linear_perpetual', 'inverse_perpetual', 'linear_delivery', 'inverse_delivery'));

ALTER TABLE instruments DROP CHECK chk_futures_contracts_delivery_expiry,
    ADD CONSTRAINT chk_instruments_delivery_expiry CHECK (
        kind NOT IN ('linear_delivery', 'inverse_delivery') OR expiry_at IS NOT NULL
    );

ALTER TABLE balances RENAME COLUMN platform_asset_id TO listing_id;

ALTER TABLE balances RENAME INDEX idx_balances_platform_asset TO idx_balances_listing;

ALTER TABLE balances DROP FOREIGN KEY fk_balances_platform_asset,
    ADD CONSTRAINT fk_balances_listing FOREIGN KEY (listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT;

ALTER TABLE positions RENAME COLUMN futures_contract_id TO instrument_id;

ALTER TABLE positions RENAME INDEX idx_positions_futures_contract TO idx_positions_instrument;

ALTER TABLE positions DROP FOREIGN KEY fk_positions_futures_contract,
    ADD CONSTRAINT fk_positions_instrument FOREIGN KEY (instrument_id) REFERENCES instruments (id) ON DELETE RESTRICT;

ALTER TABLE portfolio_listing_exclusions RENAME COLUMN platform_asset_id TO listing_id;

ALTER TABLE portfolio_listing_exclusions RENAME INDEX idx_platform_asset_exclusions_platform_asset TO idx_listing_exclusions_listing;

ALTER TABLE portfolio_listing_exclusions DROP FOREIGN KEY fk_platform_asset_exclusions_portfolio,
    ADD CONSTRAINT fk_listing_exclusions_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE CASCADE;

ALTER TABLE portfolio_listing_exclusions DROP FOREIGN KEY fk_platform_asset_exclusions_platform_asset,
    ADD CONSTRAINT fk_listing_exclusions_listing FOREIGN KEY (listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT;

ALTER TABLE portfolio_listing_exclusions DROP FOREIGN KEY fk_platform_asset_exclusions_actor,
    ADD CONSTRAINT fk_listing_exclusions_actor FOREIGN KEY (actor_user_id) REFERENCES users (id) ON DELETE RESTRICT;

ALTER TABLE portfolio_listing_exclusions DROP CHECK chk_platform_asset_exclusions_reason,
    ADD CONSTRAINT chk_listing_exclusions_reason CHECK (ignored = FALSE OR (reason IS NOT NULL AND CHAR_LENGTH(TRIM(reason)) > 0));

ALTER TABLE portfolio_listing_exclusion_events RENAME COLUMN platform_asset_id TO listing_id;

ALTER TABLE portfolio_listing_exclusion_events RENAME INDEX idx_platform_asset_exclusion_events_time TO idx_listing_exclusion_events_time;

ALTER TABLE portfolio_listing_exclusion_events DROP FOREIGN KEY fk_platform_asset_exclusion_events_portfolio,
    ADD CONSTRAINT fk_listing_exclusion_events_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE CASCADE;

ALTER TABLE portfolio_listing_exclusion_events DROP FOREIGN KEY fk_platform_asset_exclusion_events_platform_asset,
    ADD CONSTRAINT fk_listing_exclusion_events_listing FOREIGN KEY (listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT;

ALTER TABLE portfolio_listing_exclusion_events DROP FOREIGN KEY fk_platform_asset_exclusion_events_actor,
    ADD CONSTRAINT fk_listing_exclusion_events_actor FOREIGN KEY (actor_user_id) REFERENCES users (id) ON DELETE RESTRICT;

ALTER TABLE portfolio_listing_exclusion_events DROP CHECK chk_platform_asset_exclusion_events_reason,
    ADD CONSTRAINT chk_listing_exclusion_events_reason CHECK (ignored = FALSE OR (reason IS NOT NULL AND CHAR_LENGTH(TRIM(reason)) > 0));

ALTER TABLE manual_balances RENAME COLUMN platform_asset_id TO listing_id;

ALTER TABLE manual_balances DROP FOREIGN KEY fk_manual_balances_platform_asset,
    ADD CONSTRAINT fk_manual_balances_listing FOREIGN KEY (listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT;
