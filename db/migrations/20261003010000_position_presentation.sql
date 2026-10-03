-- +goose Up
ALTER TABLE account_reports ADD COLUMN margin_mode VARCHAR(32) NULL;
ALTER TABLE positions
    ADD COLUMN position_mode VARCHAR(16) NULL,
    ADD COLUMN margin_mode VARCHAR(16) NULL,
    ADD COLUMN margin_currency VARCHAR(64) NULL,
    ADD COLUMN margin_basis VARCHAR(16) NULL,
    ADD CONSTRAINT chk_positions_mode CHECK (position_mode IN ('one_way', 'hedged')),
    ADD CONSTRAINT chk_positions_margin_mode CHECK (margin_mode IN ('cross', 'isolated')),
    ADD CONSTRAINT chk_positions_margin_basis CHECK (margin_basis IN ('initial', 'allocated'));

-- Restore only exact, platform-specific references, including the stored Symbol.
-- BINARY prevents the database collation from accepting different spelling/case.
UPDATE positions p INNER JOIN instruments n ON n.id = p.instrument_id
SET p.position_mode = CASE
    WHEN n.platform = 'binance' AND (
        BINARY p.platform_position_ref = BINARY CONCAT('um:', n.symbol, ':BOTH') OR
        BINARY p.platform_position_ref = BINARY CONCAT('cm:', n.symbol, ':BOTH')) THEN 'one_way'
    WHEN n.platform = 'binance' AND (
        BINARY p.platform_position_ref = BINARY CONCAT('um:', n.symbol, ':', UPPER(p.side)) OR
        BINARY p.platform_position_ref = BINARY CONCAT('cm:', n.symbol, ':', UPPER(p.side))) THEN 'hedged'
    WHEN n.platform = 'okx' AND BINARY p.platform_position_ref = BINARY CONCAT(n.symbol, ':net') THEN 'one_way'
    WHEN n.platform = 'okx' AND BINARY p.platform_position_ref = BINARY CONCAT(n.symbol, ':', p.side) THEN 'hedged'
    WHEN n.platform = 'bybit' AND BINARY p.platform_position_ref = BINARY CONCAT(n.symbol, ':0') THEN 'one_way'
    WHEN n.platform = 'bybit' AND p.side = 'long' AND BINARY p.platform_position_ref = BINARY CONCAT(n.symbol, ':1') THEN 'hedged'
    WHEN n.platform = 'bybit' AND p.side = 'short' AND BINARY p.platform_position_ref = BINARY CONCAT(n.symbol, ':2') THEN 'hedged'
    WHEN n.platform = 'kraken' AND BINARY p.platform_position_ref = BINARY CONCAT(n.symbol, ':', p.side) THEN 'one_way'
    ELSE NULL
END;

-- Legacy Bybit Margin always stored positionIM, including isolated positions.
-- Preserve that source amount as an initial requirement; it is not allocation.
UPDATE positions p INNER JOIN instruments n ON n.id = p.instrument_id
SET p.margin_basis = 'initial', p.margin_currency = n.settlement_symbol
WHERE n.platform = 'bybit' AND p.margin IS NOT NULL;

-- Account Report modes and other missing provenance cannot be inferred from
-- the Identity's current mode, which may have changed since observation.

-- +goose Down
ALTER TABLE positions
    DROP CHECK chk_positions_mode,
    DROP CHECK chk_positions_margin_mode,
    DROP CHECK chk_positions_margin_basis,
    DROP COLUMN position_mode,
    DROP COLUMN margin_mode,
    DROP COLUMN margin_currency,
    DROP COLUMN margin_basis;
ALTER TABLE account_reports DROP COLUMN margin_mode;
