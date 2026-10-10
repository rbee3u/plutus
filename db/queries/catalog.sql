-- name: UpsertAsset :exec
INSERT INTO assets (
    id, platform, symbol, canonical_symbol, name, mapping_source, first_seen_at, last_seen_at, version, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(platform), sqlc.arg(symbol), sqlc.arg(canonical_symbol), sqlc.narg(name), 'auto',
    sqlc.arg(seen_at), sqlc.arg(seen_at), 1, sqlc.arg(seen_at), sqlc.arg(seen_at)
)
ON DUPLICATE KEY UPDATE
    last_seen_at = VALUES(last_seen_at),
    name = COALESCE(VALUES(name), name),
    updated_at = VALUES(updated_at);

-- name: GetAssetByPlatformSymbol :one
SELECT id, platform, symbol, canonical_symbol, name, mapping_source, first_seen_at, last_seen_at, version, created_at, updated_at
FROM assets
WHERE platform = sqlc.arg(platform) AND symbol = sqlc.arg(symbol);

-- name: GetAsset :one
SELECT id, platform, symbol, canonical_symbol, name, mapping_source, first_seen_at, last_seen_at, version, created_at, updated_at
FROM assets
WHERE id = sqlc.arg(id);

-- name: ListAssets :many
SELECT id, platform, symbol, canonical_symbol, name, mapping_source, first_seen_at, last_seen_at, version, created_at, updated_at
FROM assets
WHERE (sqlc.arg(filter_platform) = 0 OR platform = sqlc.arg(platform))
  AND (sqlc.arg(filter_query) = 0 OR symbol LIKE sqlc.arg(query) OR canonical_symbol LIKE sqlc.arg(query))
  AND (sqlc.arg(ignored_only) = 0 OR EXISTS (
      SELECT 1 FROM portfolio_asset_exclusions e
      WHERE e.portfolio_id = sqlc.arg(portfolio_id) AND e.asset_id = assets.id AND e.ignored = TRUE
  ))
ORDER BY FIND_IN_SET(platform, sqlc.arg(platform_order)) = 0,
         FIND_IN_SET(platform, sqlc.arg(platform_order)), platform, symbol, canonical_symbol, id
LIMIT ? OFFSET ?;

-- name: CountAssets :one
SELECT COUNT(*)
FROM assets
WHERE (sqlc.arg(filter_platform) = 0 OR platform = sqlc.arg(platform))
  AND (sqlc.arg(filter_query) = 0 OR symbol LIKE sqlc.arg(query) OR canonical_symbol LIKE sqlc.arg(query))
  AND (sqlc.arg(ignored_only) = 0 OR EXISTS (
      SELECT 1 FROM portfolio_asset_exclusions e
      WHERE e.portfolio_id = sqlc.arg(portfolio_id) AND e.asset_id = assets.id AND e.ignored = TRUE
  ));

-- name: UpdateAssetMapping :execrows
UPDATE assets
SET canonical_symbol = sqlc.arg(canonical_symbol),
    mapping_source = sqlc.arg(mapping_source),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id) AND version = sqlc.arg(version);

-- name: ListPortfoliosHoldingAsset :many
SELECT DISTINCT r.portfolio_id
FROM balances b
INNER JOIN account_reports r ON r.portfolio_id = b.portfolio_id AND r.id = b.report_id
WHERE b.asset_id = sqlc.arg(asset_id);

-- name: UpsertFuturesContract :exec
INSERT INTO futures_contracts (
    id, platform, symbol, kind, base_asset_symbol, quote_asset_symbol, settlement_asset_symbol, expiry_at, contract_size,
    first_seen_at, last_seen_at, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(platform), sqlc.arg(symbol), sqlc.arg(kind), sqlc.narg(base_asset_symbol), sqlc.narg(quote_asset_symbol),
    sqlc.narg(settlement_asset_symbol), sqlc.narg(expiry_at), sqlc.narg(contract_size),
    sqlc.arg(seen_at), sqlc.arg(seen_at), sqlc.arg(seen_at), sqlc.arg(seen_at)
)
ON DUPLICATE KEY UPDATE
    last_seen_at = VALUES(last_seen_at),
    updated_at = VALUES(updated_at);

-- name: GetFuturesContractByPlatformSymbol :one
SELECT id, platform, symbol, kind, base_asset_symbol, quote_asset_symbol, settlement_asset_symbol, expiry_at, contract_size,
       first_seen_at, last_seen_at, created_at, updated_at
FROM futures_contracts
WHERE platform = sqlc.arg(platform) AND symbol = sqlc.arg(symbol);

-- name: ListFuturesContracts :many
SELECT id, platform, symbol, kind, base_asset_symbol, quote_asset_symbol, settlement_asset_symbol, expiry_at, contract_size,
       first_seen_at, last_seen_at, created_at, updated_at
FROM futures_contracts
WHERE (sqlc.arg(filter_platform) = 0 OR platform = sqlc.arg(platform))
  AND (sqlc.arg(filter_query) = 0 OR symbol LIKE sqlc.arg(query))
ORDER BY FIND_IN_SET(platform, sqlc.arg(platform_order)) = 0,
         FIND_IN_SET(platform, sqlc.arg(platform_order)), platform, symbol, id
LIMIT ? OFFSET ?;

-- name: CountFuturesContracts :one
SELECT COUNT(*)
FROM futures_contracts
WHERE (sqlc.arg(filter_platform) = 0 OR platform = sqlc.arg(platform))
  AND (sqlc.arg(filter_query) = 0 OR symbol LIKE sqlc.arg(query));
