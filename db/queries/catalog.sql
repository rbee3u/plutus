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
SELECT sqlc.embed(a), e.excluded, e.reason AS exclusion_reason, e.version AS exclusion_version, e.effective_at AS excluded_at
FROM assets a
LEFT JOIN asset_exclusion_versions e ON e.portfolio_id = a.portfolio_id AND e.asset_id = a.id
    AND e.version = (SELECT v.version FROM asset_exclusion_versions v WHERE v.portfolio_id = a.portfolio_id AND v.asset_id = a.id ORDER BY v.portfolio_id DESC, v.asset_id DESC, v.version DESC LIMIT 1)
WHERE a.portfolio_id = sqlc.arg(portfolio_id) AND (sqlc.arg(filter_platform) = 0 OR a.platform = sqlc.arg(platform))
  AND (sqlc.arg(filter_query) = 0 OR a.symbol LIKE sqlc.arg(query) OR a.canonical_asset_symbol LIKE sqlc.arg(query))
  AND (sqlc.arg(filter_status) = 0 OR COALESCE(e.excluded, FALSE) = (sqlc.arg(status) = 'excluded'))
ORDER BY FIND_IN_SET(a.platform, sqlc.arg(platform_order)) = 0,
         FIND_IN_SET(a.platform, sqlc.arg(platform_order)), a.platform, a.symbol, a.canonical_asset_symbol, a.id
LIMIT ? OFFSET ?;

-- name: CountAssets :one
SELECT COUNT(*) FROM assets a
LEFT JOIN asset_exclusion_versions e ON e.portfolio_id = a.portfolio_id AND e.asset_id = a.id
    AND e.version = (SELECT v.version FROM asset_exclusion_versions v WHERE v.portfolio_id = a.portfolio_id AND v.asset_id = a.id ORDER BY v.portfolio_id DESC, v.asset_id DESC, v.version DESC LIMIT 1)
WHERE a.portfolio_id = sqlc.arg(portfolio_id) AND (sqlc.arg(filter_platform) = 0 OR a.platform = sqlc.arg(platform))
  AND (sqlc.arg(filter_query) = 0 OR a.symbol LIKE sqlc.arg(query) OR a.canonical_asset_symbol LIKE sqlc.arg(query))
  AND (sqlc.arg(filter_status) = 0 OR COALESCE(e.excluded, FALSE) = (sqlc.arg(status) = 'excluded'));

-- Without a state filter, LIMIT bounds rule lookups rather than just output.
-- name: ListAssetsWithoutStatus :many
SELECT sqlc.embed(a), e.excluded, e.reason AS exclusion_reason, e.version AS exclusion_version, e.effective_at AS excluded_at
FROM (
    SELECT candidate.id FROM assets candidate
    WHERE candidate.portfolio_id = sqlc.arg(portfolio_id) AND (sqlc.arg(filter_platform) = 0 OR candidate.platform = sqlc.arg(platform))
      AND (sqlc.arg(filter_query) = 0 OR candidate.symbol LIKE sqlc.arg(query) OR candidate.canonical_asset_symbol LIKE sqlc.arg(query))
    ORDER BY FIND_IN_SET(candidate.platform, sqlc.arg(platform_order)) = 0,
             FIND_IN_SET(candidate.platform, sqlc.arg(platform_order)), candidate.platform, candidate.symbol, candidate.canonical_asset_symbol, candidate.id
    LIMIT ? OFFSET ?
) page
JOIN assets a ON a.portfolio_id = sqlc.arg(portfolio_id) AND a.id = page.id
LEFT JOIN asset_exclusion_versions e ON e.portfolio_id = a.portfolio_id AND e.asset_id = a.id
    AND e.version = (SELECT v.version FROM asset_exclusion_versions v WHERE v.portfolio_id = a.portfolio_id AND v.asset_id = a.id ORDER BY v.portfolio_id DESC, v.asset_id DESC, v.version DESC LIMIT 1)
ORDER BY FIND_IN_SET(a.platform, sqlc.arg(platform_order)) = 0,
         FIND_IN_SET(a.platform, sqlc.arg(platform_order)), a.platform, a.symbol, a.canonical_asset_symbol, a.id;

-- name: CountAssetsWithoutStatus :one
SELECT COUNT(*) FROM assets
WHERE portfolio_id = sqlc.arg(portfolio_id) AND (sqlc.arg(filter_platform) = 0 OR platform = sqlc.arg(platform))
  AND (sqlc.arg(filter_query) = 0 OR symbol LIKE sqlc.arg(query) OR canonical_asset_symbol LIKE sqlc.arg(query));

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
