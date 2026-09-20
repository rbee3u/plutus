-- name: GetAssetListing :one
SELECT id, platform_id, external_ref, symbol, name, status, first_seen_at, last_seen_at, created_at, updated_at
FROM asset_listings
WHERE id = sqlc.arg(id);

-- name: GetConfirmedMappingAsset :one
SELECT r.asset_id
FROM asset_listing_mappings m
INNER JOIN asset_listing_mapping_revisions r
    ON r.portfolio_id = m.portfolio_id AND r.id = m.current_revision_id
WHERE m.portfolio_id = sqlc.arg(portfolio_id)
  AND m.asset_listing_id = sqlc.arg(asset_listing_id)
  AND r.state = 'confirmed';
