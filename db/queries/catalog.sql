-- name: UpsertAsset :exec
INSERT INTO assets (
    portfolio_id, id, platform, symbol, canonical_asset_symbol, mapping_version, first_observed_at, last_observed_at
) VALUES (
    sqlc.arg(portfolio_id), sqlc.arg(id), sqlc.arg(platform), sqlc.arg(symbol), sqlc.arg(canonical_asset_symbol), 1, sqlc.arg(observed_at), sqlc.arg(observed_at)
)
ON DUPLICATE KEY UPDATE
    first_observed_at = LEAST(first_observed_at, VALUES(first_observed_at)),
    last_observed_at = GREATEST(last_observed_at, VALUES(last_observed_at));

-- name: GetAssetByPlatformSymbol :one
SELECT portfolio_id, id, platform, symbol, canonical_asset_symbol, mapping_version, first_observed_at, last_observed_at
FROM assets
WHERE assets.portfolio_id = sqlc.arg(portfolio_id) AND platform = sqlc.arg(platform) AND symbol = sqlc.arg(symbol);

-- name: GetAsset :one
SELECT portfolio_id, id, platform, symbol, canonical_asset_symbol, mapping_version, first_observed_at, last_observed_at
FROM assets
WHERE assets.portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: ListAssets :many
SELECT portfolio_id, id, platform, symbol, canonical_asset_symbol, mapping_version, first_observed_at, last_observed_at
FROM assets
WHERE assets.portfolio_id = sqlc.arg(portfolio_id) AND (sqlc.arg(filter_platform) = 0 OR platform = sqlc.arg(platform))
  AND (sqlc.arg(filter_query) = 0 OR symbol LIKE sqlc.arg(query) OR canonical_asset_symbol LIKE sqlc.arg(query))
  AND (sqlc.arg(excluded_only) = 0 OR EXISTS (
      SELECT 1 FROM asset_exclusion_versions e
      WHERE e.portfolio_id = sqlc.arg(portfolio_id) AND e.asset_id = assets.id AND e.excluded = TRUE AND e.version = (SELECT MAX(v.version) FROM asset_exclusion_versions v WHERE v.portfolio_id = e.portfolio_id AND v.asset_id = e.asset_id)
  ))
ORDER BY FIND_IN_SET(platform, sqlc.arg(platform_order)) = 0,
         FIND_IN_SET(platform, sqlc.arg(platform_order)), platform, symbol, canonical_asset_symbol, id
LIMIT ? OFFSET ?;

-- name: CountAssets :one
SELECT COUNT(*)
FROM assets
WHERE assets.portfolio_id = sqlc.arg(portfolio_id) AND (sqlc.arg(filter_platform) = 0 OR platform = sqlc.arg(platform))
  AND (sqlc.arg(filter_query) = 0 OR symbol LIKE sqlc.arg(query) OR canonical_asset_symbol LIKE sqlc.arg(query))
  AND (sqlc.arg(excluded_only) = 0 OR EXISTS (
      SELECT 1 FROM asset_exclusion_versions e
      WHERE e.portfolio_id = sqlc.arg(portfolio_id) AND e.asset_id = assets.id AND e.excluded = TRUE AND e.version = (SELECT MAX(v.version) FROM asset_exclusion_versions v WHERE v.portfolio_id = e.portfolio_id AND v.asset_id = e.asset_id)
  ));

-- name: UpdateAssetMapping :execrows
UPDATE assets
SET canonical_asset_symbol = sqlc.arg(canonical_asset_symbol),
    mapping_version = mapping_version + 1
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND mapping_version = sqlc.arg(mapping_version);

-- name: UpsertFuturesContract :exec
INSERT INTO futures_contracts
    (portfolio_id, id, platform, symbol, terms_hash, kind, base_asset_symbol, quote_asset_symbol,
     settlement_asset_symbol, expiry_at, contract_size, first_observed_at, last_observed_at)
VALUES (sqlc.arg(portfolio_id), sqlc.arg(id), sqlc.arg(platform), sqlc.arg(symbol), sqlc.arg(terms_hash), sqlc.arg(kind),
        sqlc.narg(base_asset_symbol), sqlc.narg(quote_asset_symbol), sqlc.narg(settlement_asset_symbol),
        sqlc.narg(expiry_at), sqlc.narg(contract_size), sqlc.arg(observed_at), sqlc.arg(observed_at))
ON DUPLICATE KEY UPDATE first_observed_at = LEAST(first_observed_at, VALUES(first_observed_at)),
                        last_observed_at = GREATEST(last_observed_at, VALUES(last_observed_at));

-- name: GetFuturesContractByTerms :one
SELECT id, kind, base_asset_symbol, quote_asset_symbol, settlement_asset_symbol, expiry_at, contract_size FROM futures_contracts
WHERE portfolio_id = sqlc.arg(portfolio_id) AND platform = sqlc.arg(platform) AND symbol = sqlc.arg(symbol) AND terms_hash = sqlc.arg(terms_hash);

-- name: ListFuturesContracts :many
SELECT id, platform, symbol, kind, base_asset_symbol, quote_asset_symbol,
       settlement_asset_symbol, expiry_at, contract_size, first_observed_at, last_observed_at
FROM futures_contracts
WHERE portfolio_id = sqlc.arg(portfolio_id) AND (sqlc.arg(filter_platform) = 0 OR platform = sqlc.arg(platform))
  AND (sqlc.arg(filter_query) = 0 OR symbol LIKE sqlc.arg(query))
ORDER BY FIND_IN_SET(platform, sqlc.arg(platform_order)) = 0,
         FIND_IN_SET(platform, sqlc.arg(platform_order)), platform, symbol, last_observed_at DESC, id
LIMIT ? OFFSET ?;

-- name: CountFuturesContracts :one
SELECT COUNT(*) FROM futures_contracts
WHERE portfolio_id = sqlc.arg(portfolio_id) AND (sqlc.arg(filter_platform) = 0 OR platform = sqlc.arg(platform))
  AND (sqlc.arg(filter_query) = 0 OR symbol LIKE sqlc.arg(query));

-- name: ListMappingsForSymbols :many
SELECT symbol, canonical_asset_symbol FROM assets
WHERE portfolio_id = ? AND platform = ? AND symbol IN (sqlc.slice('symbols'));
