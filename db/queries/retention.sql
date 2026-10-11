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
DELETE FROM account_valuations
WHERE account_valuations.portfolio_id = sqlc.arg(portfolio_id) AND account_valuations.cycle_id = sqlc.arg(cycle_id)
  AND NOT EXISTS(SELECT 1 FROM balance_valuations b
      WHERE b.portfolio_id = account_valuations.portfolio_id AND b.cycle_id = account_valuations.cycle_id AND b.account_id = account_valuations.account_id)
LIMIT 1000;

-- name: ExpireEmptyPoint :exec
UPDATE valuation_points SET detail_state = 'expired'
WHERE valuation_points.portfolio_id = sqlc.arg(portfolio_id) AND valuation_points.cycle_id = sqlc.arg(cycle_id) AND valuation_points.detail_state = 'pruning'
  AND NOT EXISTS(SELECT 1 FROM account_valuations v WHERE v.portfolio_id = valuation_points.portfolio_id AND v.cycle_id = valuation_points.cycle_id);

-- name: ListReportRetentionCandidates :many
-- Seek the cursor range instead of rescanning the Portfolio's index prefix.
SELECT id, recorded_at AS at FROM account_reports FORCE INDEX (idx_account_reports_retention)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND recorded_at < sqlc.arg(before)
  AND (recorded_at > sqlc.arg(after_at) OR (recorded_at = sqlc.arg(after_at) AND id > sqlc.arg(after_id)))
ORDER BY recorded_at, id LIMIT 64;

-- A report stays while a retained valuation references it, as its own cycle's
-- report, or while it is the Account's last successful report.
-- The complete fallback-index order makes the correlated last-success lookup
-- a reverse one-row read instead of scanning and sorting the Account history.
-- name: ListPinnedReportIDs :many
SELECT r.id FROM account_reports r
WHERE r.portfolio_id = sqlc.arg(portfolio_id) AND r.id IN (sqlc.slice('ids')) AND (
    EXISTS(SELECT 1 FROM account_valuations v WHERE v.portfolio_id = r.portfolio_id AND v.account_id = r.account_id AND v.source_report_id = r.id)
    OR EXISTS(SELECT 1 FROM account_valuations v WHERE v.portfolio_id = r.portfolio_id AND v.cycle_id = r.cycle_id AND v.account_id = r.account_id)
    OR (r.status = 'succeeded' AND r.id = (
        SELECT n.id FROM account_reports n FORCE INDEX (idx_account_reports_fallback)
        WHERE n.portfolio_id = r.portfolio_id AND n.account_id = r.account_id AND n.status = 'succeeded'
        ORDER BY n.portfolio_id DESC, n.account_id DESC, n.status DESC, n.recorded_at DESC, n.id DESC LIMIT 1))
);

-- name: PruneReportBalances :execrows
DELETE FROM balances WHERE portfolio_id = sqlc.arg(portfolio_id) AND report_id IN (sqlc.slice('ids')) LIMIT 1000;

-- name: PruneReportFuturesPositions :execrows
DELETE FROM futures_positions WHERE portfolio_id = sqlc.arg(portfolio_id) AND report_id IN (sqlc.slice('ids')) LIMIT 1000;

-- name: PruneReportEarnPositions :execrows
DELETE FROM earn_positions WHERE portfolio_id = sqlc.arg(portfolio_id) AND report_id IN (sqlc.slice('ids')) LIMIT 1000;

-- name: PruneReportDualPositions :execrows
DELETE FROM dual_positions WHERE portfolio_id = sqlc.arg(portfolio_id) AND report_id IN (sqlc.slice('ids')) LIMIT 1000;

-- The page is already pin-checked under the Portfolio lock. A large report can
-- need several visits, so remove only parents whose children are all gone.
-- name: DeleteEmptyAccountReports :exec
DELETE FROM account_reports
WHERE account_reports.portfolio_id = sqlc.arg(portfolio_id) AND account_reports.id IN (sqlc.slice('ids'))
  AND NOT EXISTS(SELECT 1 FROM balances b WHERE b.portfolio_id = account_reports.portfolio_id AND b.report_id = account_reports.id)
  AND NOT EXISTS(SELECT 1 FROM futures_positions f WHERE f.portfolio_id = account_reports.portfolio_id AND f.report_id = account_reports.id)
  AND NOT EXISTS(SELECT 1 FROM earn_positions e WHERE e.portfolio_id = account_reports.portfolio_id AND e.report_id = account_reports.id)
  AND NOT EXISTS(SELECT 1 FROM dual_positions d WHERE d.portfolio_id = account_reports.portfolio_id AND d.report_id = account_reports.id);

-- name: ListRemainingReportIDs :many
SELECT id FROM account_reports WHERE portfolio_id = sqlc.arg(portfolio_id) AND id IN (sqlc.slice('ids'));

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

-- Seek each anchor class independently using the existing retention index;
-- the final merge sorts at most 128 candidates, not the whole price archive.
-- name: ListPriceRetentionCandidates :many
SELECT candidates.id, candidates.at FROM (
    (SELECT p.id, p.retrieved_at AS at FROM price_observations p FORCE INDEX (idx_price_observations_retention)
     WHERE p.portfolio_id = sqlc.arg(portfolio_id) AND p.daily_anchor = FALSE AND p.retrieved_at < sqlc.arg(before)
       AND (p.retrieved_at > sqlc.arg(after_at) OR (p.retrieved_at = sqlc.arg(after_at) AND p.id > sqlc.arg(after_id)))
     ORDER BY p.retrieved_at, p.id LIMIT 64)
    UNION ALL
    (SELECT p.id, p.retrieved_at AS at FROM price_observations p FORCE INDEX (idx_price_observations_retention)
     WHERE p.portfolio_id = sqlc.arg(portfolio_id) AND p.daily_anchor = TRUE AND p.retrieved_at < sqlc.arg(history_before)
       AND (p.retrieved_at > sqlc.arg(after_at) OR (p.retrieved_at = sqlc.arg(after_at) AND p.id > sqlc.arg(after_id)))
     ORDER BY p.retrieved_at, p.id LIMIT 64)
) candidates ORDER BY candidates.at, candidates.id LIMIT 64;

-- Preserve adopted prices and the source-time quote window of every Account's
-- last successful report, even outside the boundary: future stale valuation
-- can adopt a new mapping. Grouping by the full fallback-index prefix lets
-- MySQL skip-scan one entry per Account; a correlated latest-report lookup
-- rescans each Account's report history for every candidate.
-- name: DeleteUnpinnedPriceObservations :exec
DELETE FROM price_observations
WHERE price_observations.portfolio_id = sqlc.arg(portfolio_id) AND price_observations.id IN (sqlc.slice('ids')) AND (price_observations.daily_anchor = FALSE OR price_observations.retrieved_at < sqlc.arg(history_before))
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

-- The last point before the window supplies its opening value. Asset state
-- predecessors can live on still earlier points and remain as sparse evidence.
-- name: GetHistoryBoundary :one
SELECT p.cycle_id, p.evaluated_at, c.created_at FROM valuation_points p FORCE INDEX (idx_valuation_points_timeline)
JOIN collection_cycles c ON c.portfolio_id = p.portfolio_id AND c.id = p.cycle_id
WHERE p.portfolio_id = sqlc.arg(portfolio_id) AND p.evaluated_at < sqlc.arg(before)
ORDER BY p.evaluated_at DESC, p.cycle_id DESC LIMIT 1;

-- A retained Cycle can start before the window and finish inside it.
-- name: ListEarlierRetainedPolicyBoundaries :many
SELECT c.created_at FROM collection_cycles c
WHERE c.portfolio_id = sqlc.arg(portfolio_id) AND c.created_at < sqlc.arg(before)
  AND EXISTS (SELECT 1 FROM valuation_points p
    WHERE p.portfolio_id = c.portfolio_id AND p.cycle_id = c.id
      AND p.evaluated_at >= sqlc.arg(history_before))
ORDER BY c.created_at, c.id LIMIT 1;

-- name: ListExpiredPointCandidates :many
SELECT cycle_id AS id, evaluated_at AS at FROM valuation_points FORCE INDEX (idx_valuation_points_timeline)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND evaluated_at < sqlc.arg(before)
  AND (evaluated_at > sqlc.arg(after_at) OR (evaluated_at = sqlc.arg(after_at) AND cycle_id > sqlc.arg(after_id)))
ORDER BY evaluated_at, cycle_id LIMIT 64;

-- name: ListPinnedHistoricalPoints :many
SELECT p.cycle_id FROM valuation_points p
WHERE p.portfolio_id = sqlc.arg(portfolio_id) AND p.cycle_id IN (sqlc.slice('ids'))
  AND (p.cycle_id = sqlc.arg(boundary_cycle_id) OR EXISTS (
    SELECT 1 FROM calculated_asset_changes c
    WHERE c.portfolio_id = p.portfolio_id AND c.cycle_id = p.cycle_id
      AND c.cycle_id = (
        SELECT prior.cycle_id FROM calculated_asset_changes prior
        WHERE prior.portfolio_id = c.portfolio_id AND prior.asset_id = c.asset_id
          AND (prior.evaluated_at < sqlc.arg(boundary_at) OR (prior.evaluated_at = sqlc.arg(boundary_at) AND prior.cycle_id <= sqlc.arg(boundary_cycle_id)))
        ORDER BY prior.portfolio_id DESC, prior.asset_id DESC, prior.evaluated_at DESC, prior.cycle_id DESC LIMIT 1
      )
  ));

-- Even pinned pre-window summaries no longer need daily snapshot detail.
-- name: MarkHistoricalDetailsPruning :execrows
UPDATE valuation_points SET detail_state = 'pruning'
WHERE portfolio_id = sqlc.arg(portfolio_id) AND cycle_id IN (sqlc.slice('ids')) AND cycle_id <> sqlc.arg(boundary_cycle_id) AND detail_state IN ('full', 'daily');

-- name: DeleteExpiredManualChanges :execrows
DELETE FROM manual_balance_changes WHERE portfolio_id = sqlc.arg(portfolio_id) AND cycle_id IN (sqlc.slice('ids')) LIMIT 1000;

-- name: DeleteExpiredCalculatedChanges :execrows
DELETE FROM calculated_asset_changes WHERE portfolio_id = sqlc.arg(portfolio_id) AND cycle_id IN (sqlc.slice('ids')) LIMIT 1000;

-- name: DeleteExpiredBoundaryEvents :execrows
DELETE FROM boundary_events WHERE portfolio_id = sqlc.arg(portfolio_id) AND cycle_id IN (sqlc.slice('ids')) LIMIT 1000;

-- name: DeleteEmptyHistoricalPoints :execrows
DELETE FROM valuation_points
WHERE valuation_points.portfolio_id = sqlc.arg(portfolio_id) AND valuation_points.cycle_id IN (sqlc.slice('ids')) AND detail_state = 'expired'
  AND NOT EXISTS(SELECT 1 FROM manual_balance_changes m WHERE m.portfolio_id = valuation_points.portfolio_id AND m.cycle_id = valuation_points.cycle_id)
  AND NOT EXISTS(SELECT 1 FROM calculated_asset_changes c WHERE c.portfolio_id = valuation_points.portfolio_id AND c.cycle_id = valuation_points.cycle_id)
  AND NOT EXISTS(SELECT 1 FROM boundary_events e WHERE e.portfolio_id = valuation_points.portfolio_id AND e.cycle_id = valuation_points.cycle_id)
  AND NOT EXISTS(SELECT 1 FROM account_valuations v WHERE v.portfolio_id = valuation_points.portfolio_id AND v.cycle_id = valuation_points.cycle_id);

-- name: ListRemainingHistoricalPoints :many
SELECT cycle_id, detail_state FROM valuation_points WHERE portfolio_id = sqlc.arg(portfolio_id) AND cycle_id IN (sqlc.slice('ids'));

-- Flows after the opening point still contribute to the first retained interval.
-- name: DeleteExpiredCapitalFlows :execrows
DELETE FROM capital_flows WHERE portfolio_id = sqlc.arg(portfolio_id) AND effective_at < sqlc.arg(before) LIMIT 1000;

-- name: DeleteExpiredAuditEvents :execrows
DELETE FROM audit_events WHERE portfolio_id = sqlc.arg(portfolio_id) AND occurred_at < sqlc.arg(before) LIMIT 1000;

-- Version history keeps the opening state and every version used by a retained
-- audit. In particular, a restored rule must not reset its version counter.
-- name: ListExpiredExclusionVersions :many
SELECT e.asset_id, e.version FROM asset_exclusion_versions e
WHERE e.portfolio_id = sqlc.arg(portfolio_id) AND e.effective_at < sqlc.arg(before)
  AND EXISTS(SELECT 1 FROM asset_exclusion_versions n WHERE n.portfolio_id = e.portfolio_id AND n.asset_id = e.asset_id AND n.version > e.version AND n.effective_at < sqlc.arg(before))
  AND NOT EXISTS(SELECT 1 FROM audit_events a WHERE a.portfolio_id = e.portfolio_id AND a.resource_id = e.asset_id AND a.resource_type = 'asset'
    AND a.action IN ('catalog.asset.exclude', 'catalog.asset.restore')
    AND (CAST(JSON_UNQUOTE(JSON_EXTRACT(a.before_json, '$.version')) AS UNSIGNED) = e.version OR CAST(JSON_UNQUOTE(JSON_EXTRACT(a.after_json, '$.version')) AS UNSIGNED) = e.version))
ORDER BY e.asset_id, e.version LIMIT 64;

-- name: DeleteExpiredExclusionVersion :exec
DELETE FROM asset_exclusion_versions WHERE portfolio_id = ? AND asset_id = ? AND version = ?;

-- Cycles also remain while a stale report still needs its owning cycle.
-- name: ListExpiredCycleCandidates :many
SELECT id, created_at AS at FROM collection_cycles FORCE INDEX (idx_collection_cycles_timeline)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND created_at < sqlc.arg(before)
  AND (created_at > sqlc.arg(after_at) OR (created_at = sqlc.arg(after_at) AND id > sqlc.arg(after_id)))
ORDER BY created_at, id LIMIT 64;

-- name: DeleteUnreferencedHistoricalCycles :execrows
DELETE FROM collection_cycles
WHERE collection_cycles.portfolio_id = sqlc.arg(portfolio_id) AND collection_cycles.id IN (sqlc.slice('ids')) AND status IN ('succeeded', 'failed')
  AND NOT EXISTS(SELECT 1 FROM valuation_points p WHERE p.portfolio_id = collection_cycles.portfolio_id AND p.cycle_id = collection_cycles.id)
  AND NOT EXISTS(SELECT 1 FROM account_reports r WHERE r.portfolio_id = collection_cycles.portfolio_id AND r.cycle_id = collection_cycles.id);

-- name: DeleteUnusedHistoricalContracts :execrows
DELETE FROM futures_contracts
WHERE futures_contracts.portfolio_id = sqlc.arg(portfolio_id) AND last_observed_at < sqlc.arg(before)
  AND NOT EXISTS(SELECT 1 FROM futures_positions p WHERE p.portfolio_id = futures_contracts.portfolio_id AND p.futures_contract_id = futures_contracts.id)
LIMIT 64;

-- Default catalog entries may expire; explicit mappings and policy versions
-- are current configuration, even when the Asset is not currently held.
-- name: ListUnusedHistoricalAssets :many
SELECT a.id FROM assets a
WHERE a.portfolio_id = sqlc.arg(portfolio_id) AND a.last_observed_at < sqlc.arg(before) AND a.mapping_version = 1
  AND NOT EXISTS(SELECT 1 FROM balances b WHERE b.portfolio_id = a.portfolio_id AND b.asset_id = a.id)
  AND NOT EXISTS(SELECT 1 FROM manual_revision_balances m WHERE m.portfolio_id = a.portfolio_id AND m.asset_id = a.id)
  AND NOT EXISTS(SELECT 1 FROM asset_exclusion_versions e WHERE e.portfolio_id = a.portfolio_id AND e.asset_id = a.id)
  AND NOT EXISTS(SELECT 1 FROM calculated_asset_changes c WHERE c.portfolio_id = a.portfolio_id AND c.asset_id = a.id AND c.evaluated_at >= sqlc.arg(state_before))
  AND COALESCE((SELECT latest.treatment FROM calculated_asset_changes latest WHERE latest.portfolio_id = a.portfolio_id AND latest.asset_id = a.id
      ORDER BY latest.portfolio_id DESC, latest.asset_id DESC, latest.evaluated_at DESC, latest.cycle_id DESC LIMIT 1), 'absent') = 'absent'
ORDER BY a.id LIMIT 64;

-- name: DeleteUnusedAssetChanges :execrows
DELETE FROM calculated_asset_changes WHERE portfolio_id = sqlc.arg(portfolio_id) AND asset_id IN (sqlc.slice('ids')) LIMIT 1000;

-- name: DeleteUnusedHistoricalAssets :execrows
DELETE FROM assets WHERE assets.portfolio_id = sqlc.arg(portfolio_id) AND assets.id IN (sqlc.slice('ids'))
  AND NOT EXISTS(SELECT 1 FROM calculated_asset_changes c WHERE c.portfolio_id = assets.portfolio_id AND c.asset_id = assets.id);

-- The persisted boundary hides internal predecessors after cleanup starts and
-- never moves backward after timezone edits or a clock correction.
-- name: AdvanceHistoryBoundary :exec
UPDATE portfolios SET history_before = sqlc.arg(before)
WHERE id = sqlc.arg(id) AND (history_before IS NULL OR history_before < sqlc.arg(before));
