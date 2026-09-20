-- name: GetAssetListing :one
SELECT id, platform_id, external_ref, symbol, name, status, first_seen_at, last_seen_at, created_at, updated_at
FROM asset_listings
WHERE id = sqlc.arg(id);

-- name: GetInstrument :one
SELECT id, platform_id, external_ref, symbol, kind, base_listing_id, quote_listing_id, underlying_listing_id,
       settlement_listing_id, expiry_at, contract_size, status, first_seen_at, last_seen_at, created_at, updated_at
FROM instruments
WHERE id = sqlc.arg(id);

-- name: GetConfirmedMappingAsset :one
SELECT r.asset_id
FROM asset_listing_mappings m
INNER JOIN asset_listing_mapping_revisions r
    ON r.portfolio_id = m.portfolio_id AND r.id = m.current_revision_id
WHERE m.portfolio_id = sqlc.arg(portfolio_id)
  AND m.asset_listing_id = sqlc.arg(asset_listing_id)
  AND r.state = 'confirmed';

-- name: CountSearchAssetListings :one
SELECT COUNT(*)
FROM asset_listings l
LEFT JOIN asset_listing_mappings m
    ON m.portfolio_id = sqlc.arg(portfolio_id) AND m.asset_listing_id = l.id
LEFT JOIN asset_listing_mapping_revisions r
    ON r.portfolio_id = m.portfolio_id AND r.id = m.current_revision_id
WHERE (sqlc.arg(query) = '' OR INSTR(LOWER(l.symbol), LOWER(sqlc.arg(query))) > 0
    OR INSTR(LOWER(l.external_ref), LOWER(sqlc.arg(query))) > 0
    OR INSTR(LOWER(COALESCE(l.name, '')), LOWER(sqlc.arg(query))) > 0)
  AND (sqlc.arg(filter_platform) = 0 OR l.platform_id = sqlc.arg(platform_id))
  AND (
      sqlc.arg(mapping_state) = ''
      OR (sqlc.arg(mapping_state) = 'unmapped' AND r.state IS NULL)
      OR r.state = sqlc.arg(mapping_state)
  );

-- name: SearchAssetListings :many
SELECT
    l.id,
    l.platform_id,
    p.code AS platform_code,
    l.external_ref,
    l.symbol,
    l.name,
    l.status,
    r.asset_id AS mapped_asset_id,
    a.code AS mapped_asset_code,
    CASE
        WHEN r.state IS NULL THEN 'unmapped'
        ELSE r.state
    END AS mapping_state,
    COALESCE(m.version, 0) AS mapping_version,
    CASE WHEN pr.enabled = 1 THEN 1 ELSE 0 END AS pinned
FROM asset_listings l
INNER JOIN platforms p ON p.id = l.platform_id
LEFT JOIN asset_listing_mappings m
    ON m.portfolio_id = sqlc.arg(portfolio_id) AND m.asset_listing_id = l.id
LEFT JOIN asset_listing_mapping_revisions r
    ON r.portfolio_id = m.portfolio_id AND r.id = m.current_revision_id
LEFT JOIN assets a ON a.id = r.asset_id
LEFT JOIN catalog_pins pin
    ON pin.portfolio_id = sqlc.arg(portfolio_id) AND pin.asset_listing_id = l.id
LEFT JOIN catalog_pin_revisions pr
    ON pr.portfolio_id = pin.portfolio_id AND pr.id = pin.current_revision_id
WHERE (sqlc.arg(query) = '' OR INSTR(LOWER(l.symbol), LOWER(sqlc.arg(query))) > 0
    OR INSTR(LOWER(l.external_ref), LOWER(sqlc.arg(query))) > 0
    OR INSTR(LOWER(COALESCE(l.name, '')), LOWER(sqlc.arg(query))) > 0)
  AND (sqlc.arg(filter_platform) = 0 OR l.platform_id = sqlc.arg(platform_id))
  AND (
      sqlc.arg(mapping_state) = ''
      OR (sqlc.arg(mapping_state) = 'unmapped' AND r.state IS NULL)
      OR r.state = sqlc.arg(mapping_state)
  )
ORDER BY l.symbol ASC, l.id ASC
LIMIT ? OFFSET ?;

-- name: CountSearchInstruments :one
SELECT COUNT(*)
FROM instruments i
LEFT JOIN catalog_pins pin
    ON pin.portfolio_id = sqlc.arg(portfolio_id) AND pin.instrument_id = i.id
LEFT JOIN catalog_pin_revisions pr
    ON pr.portfolio_id = pin.portfolio_id AND pr.id = pin.current_revision_id
WHERE (sqlc.arg(query) = '' OR INSTR(LOWER(i.symbol), LOWER(sqlc.arg(query))) > 0
    OR INSTR(LOWER(i.external_ref), LOWER(sqlc.arg(query))) > 0)
  AND (sqlc.arg(filter_platform) = 0 OR i.platform_id = sqlc.arg(platform_id))
  AND (
      sqlc.arg(filter_pinned) = 0
      OR (sqlc.arg(pinned) = 1 AND pr.enabled = 1)
      OR (sqlc.arg(pinned) = 0 AND (pr.enabled IS NULL OR pr.enabled = 0))
  );

-- name: SearchInstruments :many
SELECT
    i.id,
    i.platform_id,
    p.code AS platform_code,
    i.external_ref,
    i.symbol,
    i.kind,
    i.status,
    i.expiry_at,
    CASE WHEN pr.enabled = 1 THEN 1 ELSE 0 END AS pinned
FROM instruments i
INNER JOIN platforms p ON p.id = i.platform_id
LEFT JOIN catalog_pins pin
    ON pin.portfolio_id = sqlc.arg(portfolio_id) AND pin.instrument_id = i.id
LEFT JOIN catalog_pin_revisions pr
    ON pr.portfolio_id = pin.portfolio_id AND pr.id = pin.current_revision_id
WHERE (sqlc.arg(query) = '' OR INSTR(LOWER(i.symbol), LOWER(sqlc.arg(query))) > 0
    OR INSTR(LOWER(i.external_ref), LOWER(sqlc.arg(query))) > 0)
  AND (sqlc.arg(filter_platform) = 0 OR i.platform_id = sqlc.arg(platform_id))
  AND (
      sqlc.arg(filter_pinned) = 0
      OR (sqlc.arg(pinned) = 1 AND pr.enabled = 1)
      OR (sqlc.arg(pinned) = 0 AND (pr.enabled IS NULL OR pr.enabled = 0))
  )
ORDER BY i.symbol ASC, i.id ASC
LIMIT ? OFFSET ?;

-- name: GetAssetListingMapping :one
SELECT id, portfolio_id, asset_listing_id, current_revision_id, version, created_at, updated_at
FROM asset_listing_mappings
WHERE portfolio_id = sqlc.arg(portfolio_id) AND asset_listing_id = sqlc.arg(asset_listing_id);

-- name: GetAssetListingMappingRevision :one
SELECT id, portfolio_id, mapping_id, asset_id, state, display_name_override, classification_json, reason,
       created_by_user_id, created_at
FROM asset_listing_mapping_revisions
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: InsertAssetListingMapping :exec
INSERT INTO asset_listing_mappings (
    id, portfolio_id, asset_listing_id, current_revision_id, version, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(asset_listing_id), sqlc.narg(current_revision_id), sqlc.arg(version),
    sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: InsertAssetListingMappingRevision :exec
INSERT INTO asset_listing_mapping_revisions (
    id, portfolio_id, mapping_id, asset_id, state, display_name_override, classification_json, reason,
    created_by_user_id, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(mapping_id), sqlc.arg(asset_id), sqlc.arg(state),
    sqlc.narg(display_name_override), sqlc.narg(classification_json), sqlc.narg(reason),
    sqlc.arg(created_by_user_id), sqlc.arg(created_at)
);

-- name: SetAssetListingMappingRevision :exec
UPDATE asset_listing_mappings
SET current_revision_id = sqlc.arg(current_revision_id),
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: UpdateAssetListingMappingPointer :execrows
UPDATE asset_listing_mappings
SET current_revision_id = sqlc.arg(current_revision_id),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version);

-- name: GetCatalogPinByListing :one
SELECT id, portfolio_id, asset_listing_id, instrument_id, current_revision_id, version, created_at, updated_at
FROM catalog_pins
WHERE portfolio_id = sqlc.arg(portfolio_id) AND asset_listing_id = sqlc.arg(asset_listing_id);

-- name: GetCatalogPinByInstrument :one
SELECT id, portfolio_id, asset_listing_id, instrument_id, current_revision_id, version, created_at, updated_at
FROM catalog_pins
WHERE portfolio_id = sqlc.arg(portfolio_id) AND instrument_id = sqlc.arg(instrument_id);

-- name: GetCatalogPinRevision :one
SELECT id, portfolio_id, catalog_pin_id, enabled, reason, created_by_user_id, created_at
FROM catalog_pin_revisions
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: InsertCatalogPin :exec
INSERT INTO catalog_pins (
    id, portfolio_id, asset_listing_id, instrument_id, current_revision_id, version, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.narg(asset_listing_id), sqlc.narg(instrument_id),
    sqlc.narg(current_revision_id), sqlc.arg(version), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: InsertCatalogPinRevision :exec
INSERT INTO catalog_pin_revisions (
    id, portfolio_id, catalog_pin_id, enabled, reason, created_by_user_id, created_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(catalog_pin_id), sqlc.arg(enabled), sqlc.narg(reason),
    sqlc.arg(created_by_user_id), sqlc.arg(created_at)
);

-- name: SetCatalogPinRevision :exec
UPDATE catalog_pins
SET current_revision_id = sqlc.arg(current_revision_id),
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: UpdateCatalogPinPointer :execrows
UPDATE catalog_pins
SET current_revision_id = sqlc.arg(current_revision_id),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version);
