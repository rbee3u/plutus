-- name: GetNextRetentionPortfolioID :one
SELECT id FROM portfolios WHERE deletion_requested_at IS NULL AND id > sqlc.arg(after_id) ORDER BY id LIMIT 1;

-- name: HasActiveCollectionCycle :one
SELECT EXISTS(SELECT 1 FROM collection_cycles WHERE active_portfolio_id = sqlc.arg(portfolio_id)) AS active;

-- name: GetOldestFullPointBefore :one
SELECT evaluated_at FROM valuation_points
WHERE portfolio_id = sqlc.arg(portfolio_id) AND detail_state = 'full' AND evaluated_at < sqlc.arg(before)
ORDER BY evaluated_at, cycle_id LIMIT 1;

-- name: GetLatestFullPointBefore :one
SELECT cycle_id FROM valuation_points
WHERE portfolio_id = sqlc.arg(portfolio_id) AND detail_state = 'full' AND evaluated_at < sqlc.arg(before)
ORDER BY evaluated_at DESC, cycle_id DESC LIMIT 1;

-- name: SetPointDetailState :exec
UPDATE valuation_points SET detail_state = sqlc.arg(detail_state)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND cycle_id = sqlc.arg(cycle_id);

-- name: MarkFullPointsPruningBefore :exec
UPDATE valuation_points SET detail_state = 'pruning'
WHERE portfolio_id = sqlc.arg(portfolio_id) AND detail_state = 'full' AND evaluated_at < sqlc.arg(before);

-- name: GetNextPruningPoint :one
SELECT cycle_id FROM valuation_points
WHERE portfolio_id = sqlc.arg(portfolio_id) AND detail_state = 'pruning'
ORDER BY evaluated_at, cycle_id LIMIT 1;

-- name: PrunePointBalanceValuations :execrows
DELETE FROM balance_valuations WHERE portfolio_id = sqlc.arg(portfolio_id) AND cycle_id = sqlc.arg(cycle_id) LIMIT 1000;

-- name: PrunePointAccountValuations :execrows
DELETE FROM account_valuations WHERE portfolio_id = sqlc.arg(portfolio_id) AND cycle_id = sqlc.arg(cycle_id) LIMIT 1000;

-- name: ListReportRetentionCandidates :many
SELECT id, recorded_at AS at FROM account_reports
WHERE portfolio_id = sqlc.arg(portfolio_id) AND recorded_at < sqlc.arg(before)
  AND (recorded_at > sqlc.arg(after_at) OR (recorded_at = sqlc.arg(after_at) AND id > sqlc.arg(after_id)))
ORDER BY recorded_at, id LIMIT 64;

-- A report stays while a retained valuation references it, as its own cycle's
-- report, or while it is the Account's last successful report.
-- name: IsReportPinned :one
SELECT (
    EXISTS(SELECT 1 FROM account_valuations v WHERE v.portfolio_id = r.portfolio_id AND v.account_id = r.account_id AND v.source_report_id = r.id)
    OR EXISTS(SELECT 1 FROM account_valuations v WHERE v.portfolio_id = r.portfolio_id AND v.cycle_id = r.cycle_id AND v.account_id = r.account_id)
    OR (r.status = 'succeeded' AND NOT EXISTS(
        SELECT 1 FROM account_reports n
        WHERE n.portfolio_id = r.portfolio_id AND n.account_id = r.account_id AND n.status = 'succeeded'
          AND (n.recorded_at > r.recorded_at OR (n.recorded_at = r.recorded_at AND n.id > r.id))))
) AS pinned
FROM account_reports r WHERE r.portfolio_id = sqlc.arg(portfolio_id) AND r.id = sqlc.arg(id);

-- name: PruneReportBalances :execrows
DELETE FROM balances WHERE portfolio_id = sqlc.arg(portfolio_id) AND report_id = sqlc.arg(report_id) LIMIT 1000;

-- name: PruneReportFuturesPositions :execrows
DELETE FROM futures_positions WHERE portfolio_id = sqlc.arg(portfolio_id) AND report_id = sqlc.arg(report_id) LIMIT 1000;

-- name: PruneReportEarnPositions :execrows
DELETE FROM earn_positions WHERE portfolio_id = sqlc.arg(portfolio_id) AND report_id = sqlc.arg(report_id) LIMIT 1000;

-- name: PruneReportDualPositions :execrows
DELETE FROM dual_positions WHERE portfolio_id = sqlc.arg(portfolio_id) AND report_id = sqlc.arg(report_id) LIMIT 1000;

-- name: DeleteAccountReport :exec
DELETE FROM account_reports WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: GetOldestPriceRetrievedBetween :one
SELECT retrieved_at FROM price_observations
WHERE portfolio_id = sqlc.arg(portfolio_id) AND daily_anchor = FALSE AND retrieved_at >= sqlc.arg(from_at) AND retrieved_at < sqlc.arg(before)
ORDER BY retrieved_at, id LIMIT 1;

-- Keep the last observation per market key whose price time falls within the
-- staleness window of its retrieval; future-dated quotes never anchor a day.
-- name: MarkDailyPriceAnchors :exec
UPDATE price_observations p JOIN (
    SELECT ranked.portfolio_id, ranked.id FROM (
        SELECT o.portfolio_id, o.id, ROW_NUMBER() OVER (PARTITION BY o.platform, o.requested_canonical_asset_symbol ORDER BY o.price_at DESC, o.retrieved_at DESC, o.id DESC) AS rn
        FROM price_observations o
        WHERE o.portfolio_id = sqlc.arg(portfolio_id) AND o.daily_anchor = FALSE AND o.retrieved_at >= sqlc.arg(from_at) AND o.retrieved_at < sqlc.arg(before)
          AND o.price_at <= o.retrieved_at AND o.price_at >= DATE_SUB(o.retrieved_at, INTERVAL 24 HOUR)
    ) ranked WHERE ranked.rn = 1
) anchors ON p.portfolio_id = anchors.portfolio_id AND p.id = anchors.id
SET p.daily_anchor = TRUE;

-- name: SetPriceAnchorsBefore :exec
UPDATE portfolios SET price_anchors_before = sqlc.arg(before) WHERE id = sqlc.arg(id);

-- name: ListPriceRetentionCandidates :many
SELECT id, retrieved_at AS at FROM price_observations
WHERE portfolio_id = sqlc.arg(portfolio_id) AND daily_anchor = FALSE AND retrieved_at < sqlc.arg(before)
  AND (retrieved_at > sqlc.arg(after_at) OR (retrieved_at = sqlc.arg(after_at) AND id > sqlc.arg(after_id)))
ORDER BY retrieved_at, id LIMIT 64;

-- Preserve adopted prices and the source-time quote window of every Account's
-- last successful report, even outside the boundary: future stale valuation
-- can adopt a new mapping. Grouping by the full fallback-index prefix lets
-- MySQL skip-scan one entry per Account; a correlated latest-report lookup
-- rescans each Account's report history for every candidate.
-- name: DeleteUnpinnedPriceObservation :exec
DELETE FROM price_observations
WHERE price_observations.portfolio_id = sqlc.arg(portfolio_id) AND price_observations.id = sqlc.arg(id) AND price_observations.daily_anchor = FALSE
  AND NOT EXISTS(SELECT 1 FROM balance_valuations b WHERE b.portfolio_id = price_observations.portfolio_id AND b.price_observation_id = price_observations.id)
  AND NOT EXISTS(SELECT 1 FROM capital_flows f WHERE f.portfolio_id = price_observations.portfolio_id AND f.price_observation_id = price_observations.id)
  AND NOT EXISTS(
    SELECT 1 FROM (
        SELECT n.portfolio_id, n.account_id, MAX(n.recorded_at) AS recorded_at FROM account_reports n
        WHERE n.portfolio_id = sqlc.arg(portfolio_id) AND n.status = 'succeeded'
        GROUP BY n.portfolio_id, n.account_id
    ) latest
    JOIN accounts a ON a.portfolio_id = latest.portfolio_id AND a.id = latest.account_id
    JOIN identities i ON i.portfolio_id = a.portfolio_id AND i.id = a.identity_id
    WHERE i.platform = price_observations.platform
      AND price_observations.retrieved_at <= latest.recorded_at
      AND price_observations.price_at BETWEEN DATE_SUB(latest.recorded_at, INTERVAL 24 HOUR) AND latest.recorded_at);

-- A superseded revision is removable once no retained report references it.
-- name: GetPrunableManualRevision :one
SELECT r.account_id, r.revision_number FROM manual_revisions r
WHERE r.portfolio_id = sqlc.arg(portfolio_id) AND r.changed_at < sqlc.arg(before)
  AND NOT EXISTS(SELECT 1 FROM account_reports a WHERE a.portfolio_id = r.portfolio_id AND a.account_id = r.account_id AND a.manual_revision_number = r.revision_number)
  AND EXISTS(SELECT 1 FROM manual_revisions n WHERE n.portfolio_id = r.portfolio_id AND n.account_id = r.account_id AND n.revision_number > r.revision_number)
ORDER BY r.account_id, r.revision_number LIMIT 1;

-- name: PruneManualRevisionBalances :execrows
DELETE FROM manual_revision_balances
WHERE portfolio_id = sqlc.arg(portfolio_id) AND account_id = sqlc.arg(account_id) AND revision_number = sqlc.arg(revision_number) LIMIT 1000;

-- name: DeleteManualRevision :exec
DELETE FROM manual_revisions WHERE portfolio_id = sqlc.arg(portfolio_id) AND account_id = sqlc.arg(account_id) AND revision_number = sqlc.arg(revision_number);
