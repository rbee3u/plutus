-- name: UpsertAssetListing :exec
INSERT INTO asset_listings (
    id, platform, symbol, asset_symbol, name, mapping_source, first_seen_at, last_seen_at, version, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(platform), sqlc.arg(symbol), sqlc.arg(asset_symbol), sqlc.narg(name), 'auto',
    sqlc.arg(seen_at), sqlc.arg(seen_at), 1, sqlc.arg(seen_at), sqlc.arg(seen_at)
)
ON DUPLICATE KEY UPDATE
    last_seen_at = VALUES(last_seen_at),
    name = COALESCE(VALUES(name), name),
    updated_at = VALUES(updated_at);

-- name: GetAssetListingByPlatformSymbol :one
SELECT id, platform, symbol, asset_symbol, name, mapping_source, first_seen_at, last_seen_at, version, created_at, updated_at
FROM asset_listings
WHERE platform = sqlc.arg(platform) AND symbol = sqlc.arg(symbol);

-- name: GetAssetListing :one
SELECT id, platform, symbol, asset_symbol, name, mapping_source, first_seen_at, last_seen_at, version, created_at, updated_at
FROM asset_listings
WHERE id = sqlc.arg(id);

-- name: ListAssetListings :many
SELECT id, platform, symbol, asset_symbol, name, mapping_source, first_seen_at, last_seen_at, version, created_at, updated_at
FROM asset_listings
WHERE (sqlc.arg(filter_platform) = 0 OR platform = sqlc.arg(platform))
  AND (sqlc.arg(filter_query) = 0 OR symbol LIKE sqlc.arg(query) OR asset_symbol LIKE sqlc.arg(query))
ORDER BY platform ASC, symbol ASC
LIMIT ? OFFSET ?;

-- name: CountAssetListings :one
SELECT COUNT(*)
FROM asset_listings
WHERE (sqlc.arg(filter_platform) = 0 OR platform = sqlc.arg(platform))
  AND (sqlc.arg(filter_query) = 0 OR symbol LIKE sqlc.arg(query) OR asset_symbol LIKE sqlc.arg(query));

-- name: UpdateAssetListingMapping :execrows
UPDATE asset_listings
SET asset_symbol = sqlc.arg(asset_symbol),
    mapping_source = sqlc.arg(mapping_source),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND version = sqlc.arg(version);

-- name: ListPortfoliosHoldingListing :many
SELECT DISTINCT r.portfolio_id
FROM balances b
INNER JOIN account_reports r ON r.portfolio_id = b.portfolio_id AND r.id = b.report_id
WHERE b.listing_id = sqlc.arg(listing_id);

-- name: UpsertInstrument :exec
INSERT INTO instruments (
    id, platform, symbol, kind, base_symbol, quote_symbol, settlement_symbol, expiry_at, contract_size,
    first_seen_at, last_seen_at, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(platform), sqlc.arg(symbol), sqlc.arg(kind), sqlc.narg(base_symbol), sqlc.narg(quote_symbol),
    sqlc.narg(settlement_symbol), sqlc.narg(expiry_at), sqlc.narg(contract_size),
    sqlc.arg(seen_at), sqlc.arg(seen_at), sqlc.arg(seen_at), sqlc.arg(seen_at)
)
ON DUPLICATE KEY UPDATE
    last_seen_at = VALUES(last_seen_at),
    updated_at = VALUES(updated_at);

-- name: GetInstrumentByPlatformSymbol :one
SELECT id, platform, symbol, kind, base_symbol, quote_symbol, settlement_symbol, expiry_at, contract_size,
       first_seen_at, last_seen_at, created_at, updated_at
FROM instruments
WHERE platform = sqlc.arg(platform) AND symbol = sqlc.arg(symbol);

-- name: ListInstruments :many
SELECT id, platform, symbol, kind, base_symbol, quote_symbol, settlement_symbol, expiry_at, contract_size,
       first_seen_at, last_seen_at, created_at, updated_at
FROM instruments
WHERE (sqlc.arg(filter_platform) = 0 OR platform = sqlc.arg(platform))
  AND (sqlc.arg(filter_query) = 0 OR symbol LIKE sqlc.arg(query))
ORDER BY platform ASC, symbol ASC
LIMIT ? OFFSET ?;

-- name: CountInstruments :one
SELECT COUNT(*)
FROM instruments
WHERE (sqlc.arg(filter_platform) = 0 OR platform = sqlc.arg(platform))
  AND (sqlc.arg(filter_query) = 0 OR symbol LIKE sqlc.arg(query));
