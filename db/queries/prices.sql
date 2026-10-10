-- name: InsertPriceObservation :exec
INSERT INTO price_observations (portfolio_id, id, platform, requested_canonical_asset_symbol, source_market_ref, quote_canonical_asset_symbol, price, retrieved_at, source_observed_at)
VALUES (sqlc.arg(portfolio_id), sqlc.arg(id), sqlc.arg(platform), sqlc.arg(requested_canonical_asset_symbol), sqlc.arg(source_market_ref), sqlc.arg(quote_canonical_asset_symbol), sqlc.arg(price), sqlc.arg(retrieved_at), sqlc.narg(source_observed_at));

-- Rank only the 24-hour window inside the covering selection index, then read
-- the winners; a correlated per-row lookup rescans each symbol's history.
-- Bulk imports can leave stale statistics favoring a Portfolio-only scan, so
-- the window scan forces the selection index.
-- name: ListLatestPricesByPlatformAt :many
SELECT p.* FROM (
    SELECT ranked.portfolio_id, ranked.id FROM (
        SELECT o.portfolio_id, o.id, ROW_NUMBER() OVER (PARTITION BY o.requested_canonical_asset_symbol ORDER BY o.price_at DESC, o.retrieved_at DESC, o.id DESC) AS rn
        FROM price_observations o FORCE INDEX (idx_price_observations_selection)
        WHERE o.portfolio_id = sqlc.arg(portfolio_id) AND o.platform = sqlc.arg(platform)
          AND o.requested_canonical_asset_symbol IN (sqlc.slice('symbols'))
          AND o.price_at >= DATE_SUB(sqlc.arg(at), INTERVAL 24 HOUR)
          AND o.price_at <= sqlc.arg(price_cutoff) AND o.retrieved_at <= sqlc.arg(at)
    ) ranked WHERE ranked.rn = 1
) winners
JOIN price_observations p ON p.portfolio_id = winners.portfolio_id AND p.id = winners.id;

-- name: GetPriceObservation :one
SELECT * FROM price_observations WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: ListPriceObservationsByIDs :many
SELECT * FROM price_observations WHERE portfolio_id = ? AND id IN (sqlc.slice('ids'));
