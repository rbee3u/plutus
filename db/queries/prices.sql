-- name: UpsertPrice :exec
INSERT INTO prices (platform, asset_symbol, observed_at, price, quote_symbol)
VALUES (sqlc.arg(platform), sqlc.arg(asset_symbol), sqlc.arg(observed_at), sqlc.arg(price), sqlc.arg(quote_symbol))
ON DUPLICATE KEY UPDATE price = VALUES(price), quote_symbol = VALUES(quote_symbol);

-- name: ListLatestPricesByPlatformAt :many
SELECT p.platform, p.asset_symbol, p.observed_at, p.price, p.quote_symbol
FROM prices p
INNER JOIN (
    SELECT x.asset_symbol, MAX(x.observed_at) AS observed_at
    FROM prices x
    WHERE x.platform = sqlc.arg(platform) AND x.observed_at <= sqlc.arg(at)
    GROUP BY x.asset_symbol
) latest ON latest.asset_symbol = p.asset_symbol AND latest.observed_at = p.observed_at
WHERE p.platform = sqlc.arg(platform);

-- name: GetNearestPriceAt :one
SELECT platform, asset_symbol, observed_at, price, quote_symbol
FROM prices
WHERE asset_symbol = sqlc.arg(asset_symbol)
  AND (sqlc.arg(filter_platform) = 0 OR platform = sqlc.arg(platform))
  AND observed_at <= sqlc.arg(at)
ORDER BY observed_at DESC
LIMIT 1;

-- name: ListPricesForAssetBetween :many
SELECT platform, asset_symbol, observed_at, price, quote_symbol
FROM prices
WHERE asset_symbol = sqlc.arg(asset_symbol) AND observed_at >= sqlc.arg(from_at) AND observed_at <= sqlc.arg(to_at)
ORDER BY observed_at ASC;
