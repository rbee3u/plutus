-- +goose Up
-- Position base assets predate their registration as assets. Backfill one
-- asset per portfolio, platform, and normalized base symbol so historical
-- contract-only balance rows can resolve an asset identity. Collection now
-- registers new position bases through the catalog.
INSERT INTO assets (
    portfolio_id, id, platform, symbol, canonical_asset_symbol,
    mapping_version, first_observed_at, last_observed_at
)
SELECT candidate.portfolio_id,
       UUID_TO_BIN(UUID()),
       candidate.platform,
       candidate.symbol,
       candidate.symbol,
       1,
       candidate.first_observed_at,
       candidate.last_observed_at
FROM (
    SELECT fc.portfolio_id,
           fc.platform,
           UPPER(TRIM(fc.base_asset_symbol)) AS symbol,
           MIN(fc.first_observed_at) AS first_observed_at,
           MAX(fc.last_observed_at) AS last_observed_at
    FROM futures_contracts fc
    WHERE fc.base_asset_symbol IS NOT NULL
      AND TRIM(fc.base_asset_symbol) <> ''
    GROUP BY fc.portfolio_id, fc.platform, UPPER(TRIM(fc.base_asset_symbol))
) candidate
WHERE NOT EXISTS (
    SELECT 1
    FROM assets existing
    WHERE existing.portfolio_id = candidate.portfolio_id
      AND existing.platform = candidate.platform
      AND existing.symbol = candidate.symbol
);

-- +goose Down
-- Backfilled assets are retained: removing them could invalidate retained
-- contract-only rows and later catalog references.
