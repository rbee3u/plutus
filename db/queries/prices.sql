-- name: UpsertPrice :exec
INSERT INTO prices (platform, canonical_asset_symbol, observed_at, price, quote_canonical_asset_symbol)
VALUES (sqlc.arg(platform), sqlc.arg(canonical_asset_symbol), sqlc.arg(observed_at), sqlc.arg(price), sqlc.arg(quote_canonical_asset_symbol))
ON DUPLICATE KEY UPDATE price = VALUES(price), quote_canonical_asset_symbol = VALUES(quote_canonical_asset_symbol);

-- name: ListLatestPricesByPlatformAt :many
SELECT p.platform, p.canonical_asset_symbol, p.observed_at, p.price, p.quote_canonical_asset_symbol
FROM prices p
INNER JOIN (
    SELECT x.canonical_asset_symbol, MAX(x.observed_at) AS observed_at
    FROM prices x
    WHERE x.platform = sqlc.arg(platform) AND x.observed_at <= sqlc.arg(at)
    GROUP BY x.canonical_asset_symbol
) latest ON latest.canonical_asset_symbol = p.canonical_asset_symbol AND latest.observed_at = p.observed_at
WHERE p.platform = sqlc.arg(platform);

-- name: GetNearestPriceAt :one
SELECT platform, canonical_asset_symbol, observed_at, price, quote_canonical_asset_symbol
FROM prices
WHERE canonical_asset_symbol = sqlc.arg(canonical_asset_symbol)
  AND (sqlc.arg(filter_platform) = 0 OR platform = sqlc.arg(platform))
  AND observed_at <= sqlc.arg(at)
ORDER BY observed_at DESC
LIMIT 1;

-- name: ListPricesForCanonicalAssetBetween :many
SELECT platform, canonical_asset_symbol, observed_at, price, quote_canonical_asset_symbol
FROM prices
WHERE canonical_asset_symbol = sqlc.arg(canonical_asset_symbol) AND observed_at >= sqlc.arg(from_at) AND observed_at <= sqlc.arg(to_at)
ORDER BY observed_at ASC;
