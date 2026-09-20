-- name: ListAssetListings :many
SELECT id, platform_id, external_ref, symbol, name, status, first_seen_at, last_seen_at, created_at, updated_at
FROM asset_listings
ORDER BY id;

-- name: ListInstruments :many
SELECT id, platform_id, external_ref, symbol, kind, base_listing_id, quote_listing_id, underlying_listing_id,
       settlement_listing_id, expiry_at, contract_size, status, first_seen_at, last_seen_at, created_at, updated_at
FROM instruments
ORDER BY id;

-- name: ListCurrentAssetMappings :many
SELECT m.asset_listing_id, r.asset_id, r.state
FROM asset_listing_mappings m
INNER JOIN asset_listing_mapping_revisions r
    ON r.portfolio_id = m.portfolio_id AND r.id = m.current_revision_id
WHERE m.portfolio_id = sqlc.arg(portfolio_id);

-- name: ListAllAccountsByPortfolio :many
SELECT id, portfolio_id, identity_id, external_ref, display_name, account_type, status,
       first_seen_at, last_seen_at, created_at, updated_at
FROM accounts
WHERE portfolio_id = sqlc.arg(portfolio_id)
ORDER BY created_at ASC, id ASC;

-- name: ListBalanceFacts :many
SELECT o.id, o.identity_id, o.account_id, o.source_observed_at, o.collected_at, o.completeness,
       b.asset_listing_id, b.total_quantity, b.available_quantity, b.locked_quantity
FROM observations o
INNER JOIN balance_observations b
    ON b.portfolio_id = o.portfolio_id AND b.observation_id = o.id
WHERE o.portfolio_id = sqlc.arg(portfolio_id)
ORDER BY o.source_observed_at ASC, o.id ASC;

-- name: ListPositionFacts :many
SELECT o.id, o.identity_id, o.account_id, o.source_observed_at, o.collected_at, o.completeness,
       p.instrument_id, p.platform_position_ref, p.side, p.quantity, p.entry_price, p.mark_price, p.unrealized_value
FROM observations o
INNER JOIN position_observations p
    ON p.portfolio_id = o.portfolio_id AND p.observation_id = o.id
WHERE o.portfolio_id = sqlc.arg(portfolio_id)
ORDER BY o.source_observed_at ASC, o.id ASC;

-- name: ListPriceFacts :many
SELECT o.id, o.identity_id, o.source_observed_at, o.collected_at, o.completeness,
       p.instrument_id, p.base_listing_id, p.quote_listing_id, p.price_type, p.price
FROM observations o
INNER JOIN price_observations p
    ON p.portfolio_id = o.portfolio_id AND p.observation_id = o.id
WHERE o.portfolio_id = sqlc.arg(portfolio_id)
ORDER BY o.source_observed_at ASC, o.id ASC;

-- name: ListReportedEquityFacts :many
SELECT o.id, o.identity_id, o.account_id, o.source_observed_at, o.collected_at, o.completeness,
       e.denomination_listing_id, e.amount
FROM observations o
INNER JOIN reported_equity_observations e
    ON e.portfolio_id = o.portfolio_id AND e.observation_id = o.id
WHERE o.portfolio_id = sqlc.arg(portfolio_id)
ORDER BY o.source_observed_at ASC, o.id ASC;

-- name: ListReportedEquityCoverageIdentities :many
SELECT observation_id, identity_id
FROM reported_equity_covered_identities
WHERE portfolio_id = sqlc.arg(portfolio_id);

-- name: ListReportedEquityCoverageAccounts :many
SELECT observation_id, account_id
FROM reported_equity_covered_accounts
WHERE portfolio_id = sqlc.arg(portfolio_id);

-- name: ListCollectionFailureFacts :many
SELECT f.account_id, f.scope_kind, f.occurred_at, f.error_code, f.safe_detail, s.identity_id
FROM collection_failures f
INNER JOIN identity_snapshots s
    ON s.portfolio_id = f.portfolio_id AND s.id = f.identity_snapshot_id
WHERE f.portfolio_id = sqlc.arg(portfolio_id)
ORDER BY f.occurred_at ASC, f.id ASC;

-- name: ListCapitalFlowFacts :many
SELECT f.id, f.version, r.direction, r.quantity, r.asset_id, r.effective_at, r.state, r.id AS revision_id
FROM capital_flows f
INNER JOIN capital_flow_revisions r
    ON r.portfolio_id = f.portfolio_id AND r.id = f.current_revision_id
WHERE f.portfolio_id = sqlc.arg(portfolio_id)
ORDER BY r.effective_at ASC, f.id ASC;

-- name: InsertProjectionGeneration :exec
INSERT INTO projection_generations (
    id, portfolio_id, job_id, status, timezone, display_asset_id, source_watermark_at,
    started_at, completed_at, activated_at, failure_code, failure_detail
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(job_id), sqlc.arg(status), sqlc.arg(timezone),
    sqlc.arg(display_asset_id), sqlc.arg(source_watermark_at), sqlc.arg(started_at),
    sqlc.narg(completed_at), sqlc.narg(activated_at), sqlc.narg(failure_code), sqlc.narg(failure_detail)
);

-- name: GetProjectionGeneration :one
SELECT id, portfolio_id, job_id, status, timezone, display_asset_id, source_watermark_at,
       started_at, completed_at, activated_at, failure_code, failure_detail
FROM projection_generations
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: LockProjectionGeneration :one
SELECT id, portfolio_id, job_id, status, timezone, display_asset_id, source_watermark_at,
       started_at, completed_at, activated_at, failure_code, failure_detail
FROM projection_generations
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id)
FOR UPDATE;

-- name: LockPortfolioActivation :one
SELECT id, timezone, display_asset_id, active_projection_generation_id
FROM portfolios
WHERE id = sqlc.arg(id)
FOR UPDATE;

-- name: MarkProjectionGenerationReady :execrows
UPDATE projection_generations
SET status = 'ready',
    completed_at = sqlc.arg(completed_at),
    failure_code = NULL,
    failure_detail = NULL
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND status = 'building';

-- name: MarkProjectionGenerationFailed :execrows
UPDATE projection_generations
SET status = 'failed',
    completed_at = sqlc.arg(completed_at),
    failure_code = sqlc.arg(failure_code),
    failure_detail = sqlc.arg(failure_detail)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND status IN ('building', 'ready');

-- name: MarkProjectionGenerationActive :execrows
UPDATE projection_generations
SET status = 'active',
    activated_at = sqlc.arg(activated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND status = 'ready';

-- name: RetireActiveProjectionGeneration :exec
UPDATE projection_generations
SET status = 'retired'
WHERE portfolio_id = sqlc.arg(portfolio_id) AND status = 'active' AND id <> sqlc.arg(id);

-- name: SetActiveProjectionGeneration :execrows
UPDATE portfolios
SET active_projection_generation_id = sqlc.arg(active_projection_generation_id),
    updated_at = sqlc.arg(updated_at)
WHERE id = sqlc.arg(id);

-- name: InsertLatestBalanceProjection :exec
INSERT INTO latest_balance_projections (
    generation_id, portfolio_id, identity_id, account_id, asset_listing_id, asset_id, observation_id,
    total_quantity, available_quantity, locked_quantity, source_observed_at, freshness_state, completeness
) VALUES (
    sqlc.arg(generation_id), sqlc.arg(portfolio_id), sqlc.arg(identity_id), sqlc.arg(account_id),
    sqlc.arg(asset_listing_id), sqlc.narg(asset_id), sqlc.arg(observation_id), sqlc.narg(total_quantity),
    sqlc.narg(available_quantity), sqlc.narg(locked_quantity), sqlc.arg(source_observed_at),
    sqlc.arg(freshness_state), sqlc.arg(completeness)
);

-- name: InsertLatestPositionProjection :exec
INSERT INTO latest_position_projections (
    generation_id, portfolio_id, identity_id, account_id, instrument_id, platform_position_ref, side,
    quantity, selected_mark_price, underlying_long_quantity, underlying_short_quantity, observation_id,
    source_observed_at, freshness_state, completeness
) VALUES (
    sqlc.arg(generation_id), sqlc.arg(portfolio_id), sqlc.arg(identity_id), sqlc.arg(account_id),
    sqlc.arg(instrument_id), sqlc.arg(platform_position_ref), sqlc.arg(side), sqlc.arg(quantity),
    sqlc.narg(selected_mark_price), sqlc.arg(underlying_long_quantity), sqlc.arg(underlying_short_quantity),
    sqlc.arg(observation_id), sqlc.arg(source_observed_at), sqlc.arg(freshness_state), sqlc.arg(completeness)
);

-- name: InsertPortfolioValuationProjection :exec
INSERT INTO portfolio_valuation_projections (
    id, generation_id, portfolio_id, evaluated_at, denomination_asset_id, amount, completeness,
    coverage_fingerprint, freshness_state, source_cutoff_at
) VALUES (
    sqlc.arg(id), sqlc.arg(generation_id), sqlc.arg(portfolio_id), sqlc.arg(evaluated_at),
    sqlc.arg(denomination_asset_id), sqlc.narg(amount), sqlc.arg(completeness),
    sqlc.arg(coverage_fingerprint), sqlc.arg(freshness_state), sqlc.arg(source_cutoff_at)
);

-- name: InsertValuationContributionProjection :exec
INSERT INTO valuation_contribution_projections (
    id, generation_id, portfolio_id, portfolio_valuation_id, contribution_kind, identity_id, account_id,
    asset_id, amount, completeness, coverage_fingerprint
) VALUES (
    sqlc.arg(id), sqlc.arg(generation_id), sqlc.arg(portfolio_id), sqlc.arg(portfolio_valuation_id),
    sqlc.arg(contribution_kind), sqlc.narg(identity_id), sqlc.narg(account_id), sqlc.narg(asset_id),
    sqlc.narg(amount), sqlc.arg(completeness), sqlc.arg(coverage_fingerprint)
);

-- name: InsertValuationContributionSource :exec
INSERT INTO valuation_contribution_sources (
    generation_id, portfolio_id, valuation_contribution_id, observation_id, source_role
) VALUES (
    sqlc.arg(generation_id), sqlc.arg(portfolio_id), sqlc.arg(valuation_contribution_id),
    sqlc.arg(observation_id), sqlc.arg(source_role)
);

-- name: InsertExposureProjection :exec
INSERT INTO exposure_projections (
    generation_id, portfolio_id, evaluated_at, asset_id, long_quantity, short_quantity, net_quantity,
    long_value, short_value, net_value, denomination_asset_id, completeness, freshness_state
) VALUES (
    sqlc.arg(generation_id), sqlc.arg(portfolio_id), sqlc.arg(evaluated_at), sqlc.arg(asset_id),
    sqlc.arg(long_quantity), sqlc.arg(short_quantity), sqlc.arg(net_quantity), sqlc.narg(long_value),
    sqlc.narg(short_value), sqlc.narg(net_value), sqlc.arg(denomination_asset_id), sqlc.arg(completeness),
    sqlc.arg(freshness_state)
);

-- name: InsertContributedCapitalProjection :exec
INSERT INTO contributed_capital_projections (
    generation_id, portfolio_id, evaluated_at, denomination_asset_id, amount, completeness, source_revision_watermark
) VALUES (
    sqlc.arg(generation_id), sqlc.arg(portfolio_id), sqlc.arg(evaluated_at), sqlc.arg(denomination_asset_id),
    sqlc.narg(amount), sqlc.arg(completeness), sqlc.arg(source_revision_watermark)
);

-- name: InsertValueChangeProjection :exec
INSERT INTO value_change_projections (
    id, generation_id, portfolio_id, period_kind, period_start_date, period_end_date, as_of_at,
    beginning_valuation_id, ending_valuation_id, contribution_amount, withdrawal_amount, amount, state,
    coverage_fingerprint
) VALUES (
    sqlc.arg(id), sqlc.arg(generation_id), sqlc.arg(portfolio_id), sqlc.arg(period_kind),
    sqlc.arg(period_start_date), sqlc.arg(period_end_date), sqlc.arg(as_of_at),
    sqlc.arg(beginning_valuation_id), sqlc.arg(ending_valuation_id), sqlc.narg(contribution_amount),
    sqlc.narg(withdrawal_amount), sqlc.narg(amount), sqlc.arg(state), sqlc.arg(coverage_fingerprint)
);

-- name: InsertPeriodSummaryProjection :exec
INSERT INTO period_summary_projections (
    generation_id, portfolio_id, period_kind, period_start_date, period_end_date, as_of_at,
    valuation_open, valuation_close, valuation_high, valuation_low, contributed_capital_close,
    value_change, freshness_state, completeness
) VALUES (
    sqlc.arg(generation_id), sqlc.arg(portfolio_id), sqlc.arg(period_kind), sqlc.arg(period_start_date),
    sqlc.arg(period_end_date), sqlc.arg(as_of_at), sqlc.narg(valuation_open), sqlc.narg(valuation_close),
    sqlc.narg(valuation_high), sqlc.narg(valuation_low), sqlc.narg(contributed_capital_close),
    sqlc.narg(value_change), sqlc.arg(freshness_state), sqlc.arg(completeness)
);

-- name: InsertProjectionIssue :exec
INSERT INTO projection_issues (
    id, generation_id, portfolio_id, projection_kind, projection_key, severity, code, summary,
    details_json, identity_id, account_id, observation_id, first_effective_at, last_effective_at
) VALUES (
    sqlc.arg(id), sqlc.arg(generation_id), sqlc.arg(portfolio_id), sqlc.arg(projection_kind),
    sqlc.arg(projection_key), sqlc.arg(severity), sqlc.arg(code), sqlc.arg(summary),
    sqlc.narg(details_json), sqlc.narg(identity_id), sqlc.narg(account_id), sqlc.narg(observation_id),
    sqlc.arg(first_effective_at), sqlc.arg(last_effective_at)
);

-- name: ListLatestBalanceProjections :many
SELECT generation_id, portfolio_id, identity_id, account_id, asset_listing_id, asset_id, observation_id,
       total_quantity, available_quantity, locked_quantity, source_observed_at, freshness_state, completeness
FROM latest_balance_projections
WHERE generation_id = sqlc.arg(generation_id)
ORDER BY account_id ASC, asset_listing_id ASC;

-- name: ListLatestPositionProjections :many
SELECT generation_id, portfolio_id, identity_id, account_id, instrument_id, platform_position_ref, side,
       quantity, selected_mark_price, underlying_long_quantity, underlying_short_quantity, observation_id,
       source_observed_at, freshness_state, completeness
FROM latest_position_projections
WHERE generation_id = sqlc.arg(generation_id)
ORDER BY account_id ASC, instrument_id ASC, platform_position_ref ASC;

-- name: ListPortfolioValuationProjections :many
SELECT id, generation_id, portfolio_id, evaluated_at, denomination_asset_id, amount, completeness,
       coverage_fingerprint, freshness_state, source_cutoff_at
FROM portfolio_valuation_projections
WHERE generation_id = sqlc.arg(generation_id)
ORDER BY evaluated_at ASC, id ASC;

-- name: ListExposureProjections :many
SELECT generation_id, portfolio_id, evaluated_at, asset_id, long_quantity, short_quantity, net_quantity,
       long_value, short_value, net_value, denomination_asset_id, completeness, freshness_state
FROM exposure_projections
WHERE generation_id = sqlc.arg(generation_id)
ORDER BY evaluated_at ASC, asset_id ASC;

-- name: ListContributedCapitalProjections :many
SELECT generation_id, portfolio_id, evaluated_at, denomination_asset_id, amount, completeness, source_revision_watermark
FROM contributed_capital_projections
WHERE generation_id = sqlc.arg(generation_id)
ORDER BY evaluated_at ASC;

-- name: ListValueChangeProjections :many
SELECT id, generation_id, portfolio_id, period_kind, period_start_date, period_end_date, as_of_at,
       beginning_valuation_id, ending_valuation_id, contribution_amount, withdrawal_amount, amount, state,
       coverage_fingerprint
FROM value_change_projections
WHERE generation_id = sqlc.arg(generation_id)
ORDER BY period_kind ASC, period_start_date ASC, period_end_date ASC;

-- name: ListPeriodSummaryProjections :many
SELECT generation_id, portfolio_id, period_kind, period_start_date, period_end_date, as_of_at,
       valuation_open, valuation_close, valuation_high, valuation_low, contributed_capital_close,
       value_change, freshness_state, completeness
FROM period_summary_projections
WHERE generation_id = sqlc.arg(generation_id)
ORDER BY period_kind ASC, period_start_date ASC, period_end_date ASC;

-- name: ListProjectionIssues :many
SELECT id, generation_id, portfolio_id, projection_kind, projection_key, severity, code, summary,
       details_json, identity_id, account_id, observation_id, first_effective_at, last_effective_at
FROM projection_issues
WHERE generation_id = sqlc.arg(generation_id)
ORDER BY code ASC, projection_key ASC, id ASC;
