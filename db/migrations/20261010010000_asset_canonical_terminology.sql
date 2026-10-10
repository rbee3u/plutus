-- +goose Up
-- Preserve identities, financial values, and retained JSON wire formats.
RENAME TABLE platform_assets TO assets;

RENAME TABLE portfolio_platform_asset_exclusions TO portfolio_asset_exclusions;

RENAME TABLE portfolio_platform_asset_exclusion_events TO portfolio_asset_exclusion_events;

ALTER TABLE assets RENAME COLUMN asset_symbol TO canonical_symbol;

ALTER TABLE balances RENAME COLUMN platform_asset_id TO asset_id;

ALTER TABLE manual_balances RENAME COLUMN platform_asset_id TO asset_id;

ALTER TABLE manual_balances RENAME COLUMN platform_asset_symbol TO asset_symbol;

ALTER TABLE portfolio_asset_exclusions RENAME COLUMN platform_asset_id TO asset_id;

ALTER TABLE portfolio_asset_exclusion_events RENAME COLUMN platform_asset_id TO asset_id;

ALTER TABLE futures_contracts RENAME COLUMN base_platform_asset_symbol TO base_asset_symbol;

ALTER TABLE futures_contracts RENAME COLUMN quote_platform_asset_symbol TO quote_asset_symbol;

ALTER TABLE futures_contracts RENAME COLUMN settlement_platform_asset_symbol TO settlement_asset_symbol;

ALTER TABLE dual_positions RENAME COLUMN base_platform_asset_symbol TO base_asset_symbol;

ALTER TABLE dual_positions RENAME COLUMN quote_platform_asset_symbol TO quote_asset_symbol;

ALTER TABLE dual_positions RENAME COLUMN invest_platform_asset_symbol TO invest_asset_symbol;

ALTER TABLE positions RENAME COLUMN margin_platform_asset_symbol TO margin_asset_symbol;

ALTER TABLE prices RENAME COLUMN asset_symbol TO canonical_asset_symbol;

ALTER TABLE prices RENAME COLUMN quote_asset_symbol TO quote_canonical_asset_symbol;

ALTER TABLE capital_flows RENAME COLUMN asset_symbol TO canonical_asset_symbol;

ALTER TABLE assets RENAME INDEX uq_platform_assets_platform_symbol TO uq_assets_platform_symbol;

ALTER TABLE assets RENAME INDEX idx_platform_assets_asset_symbol TO idx_assets_canonical_symbol;

ALTER TABLE balances RENAME INDEX idx_balances_platform_asset TO idx_balances_asset;

ALTER TABLE portfolio_asset_exclusions RENAME INDEX idx_platform_asset_exclusions_platform_asset TO idx_asset_exclusions_asset;

ALTER TABLE portfolio_asset_exclusion_events RENAME INDEX idx_platform_asset_exclusion_events_time TO idx_asset_exclusion_events_time;

ALTER TABLE assets DROP CHECK chk_platform_assets_platform,
    ADD CONSTRAINT chk_assets_platform CHECK (platform IN ('binance', 'okx', 'bybit', 'kraken'));

ALTER TABLE assets DROP CHECK chk_platform_assets_mapping_source,
    ADD CONSTRAINT chk_assets_mapping_source CHECK (mapping_source IN ('auto', 'manual'));

ALTER TABLE portfolio_asset_exclusions DROP CHECK chk_platform_asset_exclusions_reason,
    ADD CONSTRAINT chk_asset_exclusions_reason CHECK (ignored = FALSE OR (reason IS NOT NULL AND CHAR_LENGTH(TRIM(reason)) > 0));

ALTER TABLE portfolio_asset_exclusion_events DROP CHECK chk_platform_asset_exclusion_events_reason,
    ADD CONSTRAINT chk_asset_exclusion_events_reason CHECK (ignored = FALSE OR (reason IS NOT NULL AND CHAR_LENGTH(TRIM(reason)) > 0));

ALTER TABLE balances DROP FOREIGN KEY fk_balances_platform_asset,
    ADD CONSTRAINT fk_balances_asset FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT;

ALTER TABLE manual_balances DROP FOREIGN KEY fk_manual_balances_platform_asset,
    ADD CONSTRAINT fk_manual_balances_asset FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT;

ALTER TABLE portfolio_asset_exclusions DROP FOREIGN KEY fk_platform_asset_exclusions_platform_asset,
    ADD CONSTRAINT fk_asset_exclusions_asset FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT;

ALTER TABLE portfolio_asset_exclusions DROP FOREIGN KEY fk_platform_asset_exclusions_portfolio,
    ADD CONSTRAINT fk_asset_exclusions_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE CASCADE;

ALTER TABLE portfolio_asset_exclusions DROP FOREIGN KEY fk_platform_asset_exclusions_actor,
    ADD CONSTRAINT fk_asset_exclusions_actor FOREIGN KEY (actor_user_id) REFERENCES users (id) ON DELETE RESTRICT;

ALTER TABLE portfolio_asset_exclusion_events DROP FOREIGN KEY fk_platform_asset_exclusion_events_platform_asset,
    ADD CONSTRAINT fk_asset_exclusion_events_asset FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT;

ALTER TABLE portfolio_asset_exclusion_events DROP FOREIGN KEY fk_platform_asset_exclusion_events_portfolio,
    ADD CONSTRAINT fk_asset_exclusion_events_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE CASCADE;

ALTER TABLE portfolio_asset_exclusion_events DROP FOREIGN KEY fk_platform_asset_exclusion_events_actor,
    ADD CONSTRAINT fk_asset_exclusion_events_actor FOREIGN KEY (actor_user_id) REFERENCES users (id) ON DELETE RESTRICT;

-- +goose Down
ALTER TABLE portfolio_asset_exclusion_events DROP FOREIGN KEY fk_asset_exclusion_events_actor,
    ADD CONSTRAINT fk_platform_asset_exclusion_events_actor FOREIGN KEY (actor_user_id) REFERENCES users (id) ON DELETE RESTRICT;

ALTER TABLE portfolio_asset_exclusion_events DROP FOREIGN KEY fk_asset_exclusion_events_portfolio,
    ADD CONSTRAINT fk_platform_asset_exclusion_events_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE CASCADE;

ALTER TABLE portfolio_asset_exclusion_events DROP FOREIGN KEY fk_asset_exclusion_events_asset,
    ADD CONSTRAINT fk_platform_asset_exclusion_events_platform_asset FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT;

ALTER TABLE portfolio_asset_exclusions DROP FOREIGN KEY fk_asset_exclusions_actor,
    ADD CONSTRAINT fk_platform_asset_exclusions_actor FOREIGN KEY (actor_user_id) REFERENCES users (id) ON DELETE RESTRICT;

ALTER TABLE portfolio_asset_exclusions DROP FOREIGN KEY fk_asset_exclusions_portfolio,
    ADD CONSTRAINT fk_platform_asset_exclusions_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE CASCADE;

ALTER TABLE portfolio_asset_exclusions DROP FOREIGN KEY fk_asset_exclusions_asset,
    ADD CONSTRAINT fk_platform_asset_exclusions_platform_asset FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT;

ALTER TABLE manual_balances DROP FOREIGN KEY fk_manual_balances_asset,
    ADD CONSTRAINT fk_manual_balances_platform_asset FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT;

ALTER TABLE balances DROP FOREIGN KEY fk_balances_asset,
    ADD CONSTRAINT fk_balances_platform_asset FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT;

ALTER TABLE portfolio_asset_exclusion_events DROP CHECK chk_asset_exclusion_events_reason,
    ADD CONSTRAINT chk_platform_asset_exclusion_events_reason CHECK (ignored = FALSE OR (reason IS NOT NULL AND CHAR_LENGTH(TRIM(reason)) > 0));

ALTER TABLE portfolio_asset_exclusions DROP CHECK chk_asset_exclusions_reason,
    ADD CONSTRAINT chk_platform_asset_exclusions_reason CHECK (ignored = FALSE OR (reason IS NOT NULL AND CHAR_LENGTH(TRIM(reason)) > 0));

ALTER TABLE assets DROP CHECK chk_assets_mapping_source,
    ADD CONSTRAINT chk_platform_assets_mapping_source CHECK (mapping_source IN ('auto', 'manual'));

ALTER TABLE assets DROP CHECK chk_assets_platform,
    ADD CONSTRAINT chk_platform_assets_platform CHECK (platform IN ('binance', 'okx', 'bybit', 'kraken'));

ALTER TABLE portfolio_asset_exclusion_events RENAME INDEX idx_asset_exclusion_events_time TO idx_platform_asset_exclusion_events_time;

ALTER TABLE portfolio_asset_exclusions RENAME INDEX idx_asset_exclusions_asset TO idx_platform_asset_exclusions_platform_asset;

ALTER TABLE balances RENAME INDEX idx_balances_asset TO idx_balances_platform_asset;

ALTER TABLE assets RENAME INDEX idx_assets_canonical_symbol TO idx_platform_assets_asset_symbol;

ALTER TABLE assets RENAME INDEX uq_assets_platform_symbol TO uq_platform_assets_platform_symbol;

ALTER TABLE capital_flows RENAME COLUMN canonical_asset_symbol TO asset_symbol;

ALTER TABLE prices RENAME COLUMN quote_canonical_asset_symbol TO quote_asset_symbol;

ALTER TABLE prices RENAME COLUMN canonical_asset_symbol TO asset_symbol;

ALTER TABLE positions RENAME COLUMN margin_asset_symbol TO margin_platform_asset_symbol;

ALTER TABLE dual_positions RENAME COLUMN invest_asset_symbol TO invest_platform_asset_symbol;

ALTER TABLE dual_positions RENAME COLUMN quote_asset_symbol TO quote_platform_asset_symbol;

ALTER TABLE dual_positions RENAME COLUMN base_asset_symbol TO base_platform_asset_symbol;

ALTER TABLE futures_contracts RENAME COLUMN settlement_asset_symbol TO settlement_platform_asset_symbol;

ALTER TABLE futures_contracts RENAME COLUMN quote_asset_symbol TO quote_platform_asset_symbol;

ALTER TABLE futures_contracts RENAME COLUMN base_asset_symbol TO base_platform_asset_symbol;

ALTER TABLE portfolio_asset_exclusion_events RENAME COLUMN asset_id TO platform_asset_id;

ALTER TABLE portfolio_asset_exclusions RENAME COLUMN asset_id TO platform_asset_id;

ALTER TABLE manual_balances RENAME COLUMN asset_symbol TO platform_asset_symbol;

ALTER TABLE manual_balances RENAME COLUMN asset_id TO platform_asset_id;

ALTER TABLE balances RENAME COLUMN asset_id TO platform_asset_id;

ALTER TABLE assets RENAME COLUMN canonical_symbol TO asset_symbol;

RENAME TABLE portfolio_asset_exclusion_events TO portfolio_platform_asset_exclusion_events;

RENAME TABLE portfolio_asset_exclusions TO portfolio_platform_asset_exclusions;

RENAME TABLE assets TO platform_assets;
