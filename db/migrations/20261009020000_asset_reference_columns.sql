-- +goose Up
-- These are retained Platform codes, not the Asset mappings applied at valuation.
ALTER TABLE manual_balances RENAME COLUMN symbol TO platform_asset_symbol;
ALTER TABLE dual_positions RENAME COLUMN base_symbol TO base_platform_asset_symbol;
ALTER TABLE dual_positions RENAME COLUMN quote_symbol TO quote_platform_asset_symbol;
ALTER TABLE dual_positions RENAME COLUMN invest_symbol TO invest_platform_asset_symbol;
ALTER TABLE futures_contracts RENAME COLUMN base_symbol TO base_platform_asset_symbol;
ALTER TABLE futures_contracts RENAME COLUMN quote_symbol TO quote_platform_asset_symbol;
ALTER TABLE futures_contracts RENAME COLUMN settlement_symbol TO settlement_platform_asset_symbol;
ALTER TABLE positions RENAME COLUMN margin_currency TO margin_platform_asset_symbol;

-- Prices already use the cross-Platform Asset namespace for valuation lookup.
ALTER TABLE prices RENAME COLUMN quote_symbol TO quote_asset_symbol;
ALTER TABLE platform_assets RENAME INDEX idx_platform_assets_asset TO idx_platform_assets_asset_symbol;

ALTER TABLE platform_assets DROP CHECK chk_platform_assets_source,
    ADD CONSTRAINT chk_platform_assets_mapping_source CHECK (mapping_source IN ('auto', 'manual'));
ALTER TABLE manual_balances DROP CHECK chk_manual_quantity,
    ADD CONSTRAINT chk_manual_balances_quantity CHECK (quantity <> 0);
ALTER TABLE dual_positions DROP CHECK chk_dual_principal,
    ADD CONSTRAINT chk_dual_positions_principal CHECK (principal > 0);
ALTER TABLE dual_positions DROP CHECK chk_dual_direction,
    ADD CONSTRAINT chk_dual_positions_direction CHECK (direction IS NULL OR direction IN ('buy_low', 'sell_high'));
ALTER TABLE dual_positions DROP CHECK chk_dual_status,
    ADD CONSTRAINT chk_dual_positions_status CHECK (status IN ('pending', 'active', 'settling', 'redeeming', 'refunding'));
ALTER TABLE dual_positions DROP CHECK chk_dual_rate_kind,
    ADD CONSTRAINT chk_dual_positions_rate_kind CHECK (rate_kind IS NULL OR rate_kind IN ('apr', 'apy', 'annualized_yield'));
ALTER TABLE positions DROP CHECK chk_positions_unit,
    ADD CONSTRAINT chk_positions_quantity_unit CHECK (quantity_unit IN ('contracts', 'base', 'quote'));
ALTER TABLE positions DROP CHECK chk_positions_mode,
    ADD CONSTRAINT chk_positions_position_mode CHECK (position_mode IN ('one_way', 'hedged'));

-- +goose Down
ALTER TABLE positions DROP CHECK chk_positions_position_mode,
    ADD CONSTRAINT chk_positions_mode CHECK (position_mode IN ('one_way', 'hedged'));
ALTER TABLE positions DROP CHECK chk_positions_quantity_unit,
    ADD CONSTRAINT chk_positions_unit CHECK (quantity_unit IN ('contracts', 'base', 'quote'));
ALTER TABLE dual_positions DROP CHECK chk_dual_positions_rate_kind,
    ADD CONSTRAINT chk_dual_rate_kind CHECK (rate_kind IS NULL OR rate_kind IN ('apr', 'apy', 'annualized_yield'));
ALTER TABLE dual_positions DROP CHECK chk_dual_positions_status,
    ADD CONSTRAINT chk_dual_status CHECK (status IN ('pending', 'active', 'settling', 'redeeming', 'refunding'));
ALTER TABLE dual_positions DROP CHECK chk_dual_positions_direction,
    ADD CONSTRAINT chk_dual_direction CHECK (direction IS NULL OR direction IN ('buy_low', 'sell_high'));
ALTER TABLE dual_positions DROP CHECK chk_dual_positions_principal,
    ADD CONSTRAINT chk_dual_principal CHECK (principal > 0);
ALTER TABLE manual_balances DROP CHECK chk_manual_balances_quantity,
    ADD CONSTRAINT chk_manual_quantity CHECK (quantity <> 0);
ALTER TABLE platform_assets DROP CHECK chk_platform_assets_mapping_source,
    ADD CONSTRAINT chk_platform_assets_source CHECK (mapping_source IN ('auto', 'manual'));
ALTER TABLE platform_assets RENAME INDEX idx_platform_assets_asset_symbol TO idx_platform_assets_asset;
ALTER TABLE prices RENAME COLUMN quote_asset_symbol TO quote_symbol;
ALTER TABLE positions RENAME COLUMN margin_platform_asset_symbol TO margin_currency;
ALTER TABLE futures_contracts RENAME COLUMN settlement_platform_asset_symbol TO settlement_symbol;
ALTER TABLE futures_contracts RENAME COLUMN quote_platform_asset_symbol TO quote_symbol;
ALTER TABLE futures_contracts RENAME COLUMN base_platform_asset_symbol TO base_symbol;
ALTER TABLE dual_positions RENAME COLUMN invest_platform_asset_symbol TO invest_symbol;
ALTER TABLE dual_positions RENAME COLUMN quote_platform_asset_symbol TO quote_symbol;
ALTER TABLE dual_positions RENAME COLUMN base_platform_asset_symbol TO base_symbol;
ALTER TABLE manual_balances RENAME COLUMN platform_asset_symbol TO symbol;
