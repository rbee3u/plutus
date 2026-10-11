-- Seed initialization owns an empty Portfolio and loads its finite source
-- history in one transaction. Runtime collection uses bounded point reads.
-- name: ListSeedHistoryPrices :many
SELECT * FROM price_observations
WHERE portfolio_id = ?
ORDER BY platform, requested_canonical_asset_symbol, price_at DESC, retrieved_at DESC, id DESC;

-- name: ListSeedHistoryExclusions :many
SELECT * FROM asset_exclusion_versions
WHERE portfolio_id = ?
ORDER BY asset_id, version DESC;
