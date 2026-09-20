-- name: CountIdentityObservations :one
SELECT COUNT(*) FROM observations
WHERE portfolio_id = sqlc.arg(portfolio_id) AND identity_id = sqlc.arg(identity_id);

-- name: GetIdentitySnapshot :one
SELECT id, portfolio_id, collection_run_id, identity_id, started_at, completed_at, status,
       completeness, expected_scope_count, successful_scope_count, failed_scope_count,
       created_at, updated_at
FROM identity_snapshots
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: InsertIdentitySnapshot :exec
INSERT INTO identity_snapshots (
    id, portfolio_id, collection_run_id, identity_id, started_at, completed_at, status,
    completeness, expected_scope_count, successful_scope_count, failed_scope_count,
    created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(collection_run_id), sqlc.arg(identity_id),
    sqlc.arg(started_at), sqlc.narg(completed_at), sqlc.arg(status), sqlc.arg(completeness),
    sqlc.arg(expected_scope_count), sqlc.arg(successful_scope_count), sqlc.arg(failed_scope_count),
    sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: InsertCollectionRun :exec
INSERT INTO collection_runs (
    id, portfolio_id, job_id, connection_id, identity_id, adapter_name, adapter_version,
    started_at, finished_at, status, successful_scope_count, failed_scope_count, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(job_id), sqlc.arg(connection_id), sqlc.arg(identity_id),
    sqlc.arg(adapter_name), sqlc.arg(adapter_version), sqlc.arg(started_at), sqlc.narg(finished_at),
    sqlc.arg(status), sqlc.arg(successful_scope_count), sqlc.arg(failed_scope_count),
    sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: InsertCollectionJobDetail :exec
INSERT INTO collection_job_details (
    portfolio_id, job_id, connection_id, `trigger`, scope
) VALUES (
    sqlc.arg(portfolio_id), sqlc.arg(job_id), sqlc.arg(connection_id), sqlc.arg(job_trigger), sqlc.arg(scope)
);

-- name: InsertCollectionFailure :exec
INSERT INTO collection_failures (
    id, portfolio_id, collection_run_id, identity_snapshot_id, account_id, scope_kind, scope_ref,
    phase, error_class, error_code, safe_detail, retriable, occurred_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(collection_run_id), sqlc.arg(identity_snapshot_id),
    sqlc.narg(account_id), sqlc.arg(scope_kind), sqlc.narg(scope_ref), sqlc.arg(phase),
    sqlc.arg(error_class), sqlc.arg(error_code), sqlc.arg(safe_detail), sqlc.arg(retriable),
    sqlc.arg(occurred_at)
);

-- name: GetObservation :one
SELECT id, portfolio_id, identity_snapshot_id, identity_id, account_id, kind,
       source_observed_at, collected_at, completeness, adapter_name, adapter_version,
       schema_version, version, created_at, updated_at
FROM observations
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id);

-- name: InsertObservation :exec
INSERT INTO observations (
    id, portfolio_id, identity_snapshot_id, identity_id, account_id, kind,
    source_observed_at, collected_at, completeness, adapter_name, adapter_version,
    schema_version, version, created_at, updated_at
) VALUES (
    sqlc.arg(id), sqlc.arg(portfolio_id), sqlc.arg(identity_snapshot_id), sqlc.arg(identity_id),
    sqlc.narg(account_id), sqlc.arg(kind), sqlc.arg(source_observed_at), sqlc.arg(collected_at),
    sqlc.arg(completeness), sqlc.arg(adapter_name), sqlc.arg(adapter_version),
    sqlc.arg(schema_version), sqlc.arg(version), sqlc.arg(created_at), sqlc.arg(updated_at)
);

-- name: UpdateObservationHeader :execrows
UPDATE observations
SET completeness = sqlc.arg(completeness),
    version = version + 1,
    updated_at = sqlc.arg(updated_at)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND id = sqlc.arg(id) AND version = sqlc.arg(version);

-- name: GetBalanceObservation :one
SELECT portfolio_id, observation_id, asset_listing_id, total_quantity, available_quantity, locked_quantity
FROM balance_observations
WHERE portfolio_id = sqlc.arg(portfolio_id) AND observation_id = sqlc.arg(observation_id);

-- name: InsertBalanceObservation :exec
INSERT INTO balance_observations (
    portfolio_id, observation_id, asset_listing_id, total_quantity, available_quantity, locked_quantity
) VALUES (
    sqlc.arg(portfolio_id), sqlc.arg(observation_id), sqlc.arg(asset_listing_id),
    sqlc.narg(total_quantity), sqlc.narg(available_quantity), sqlc.narg(locked_quantity)
);

-- name: UpdateBalanceObservation :exec
UPDATE balance_observations
SET total_quantity = sqlc.narg(total_quantity),
    available_quantity = sqlc.narg(available_quantity),
    locked_quantity = sqlc.narg(locked_quantity)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND observation_id = sqlc.arg(observation_id);

-- name: GetPositionObservation :one
SELECT portfolio_id, observation_id, instrument_id, platform_position_ref, side, quantity,
       entry_price, mark_price, unrealized_value, leverage
FROM position_observations
WHERE portfolio_id = sqlc.arg(portfolio_id) AND observation_id = sqlc.arg(observation_id);

-- name: InsertPositionObservation :exec
INSERT INTO position_observations (
    portfolio_id, observation_id, instrument_id, platform_position_ref, side, quantity,
    entry_price, mark_price, unrealized_value, leverage
) VALUES (
    sqlc.arg(portfolio_id), sqlc.arg(observation_id), sqlc.arg(instrument_id),
    sqlc.arg(platform_position_ref), sqlc.arg(side), sqlc.arg(quantity),
    sqlc.narg(entry_price), sqlc.narg(mark_price), sqlc.narg(unrealized_value), sqlc.narg(leverage)
);

-- name: UpdatePositionObservation :exec
UPDATE position_observations
SET side = sqlc.arg(side),
    quantity = sqlc.arg(quantity),
    entry_price = sqlc.narg(entry_price),
    mark_price = sqlc.narg(mark_price),
    unrealized_value = sqlc.narg(unrealized_value),
    leverage = sqlc.narg(leverage)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND observation_id = sqlc.arg(observation_id);

-- name: GetReportedEquityObservation :one
SELECT portfolio_id, observation_id, denomination_listing_id, amount
FROM reported_equity_observations
WHERE portfolio_id = sqlc.arg(portfolio_id) AND observation_id = sqlc.arg(observation_id);

-- name: InsertReportedEquityObservation :exec
INSERT INTO reported_equity_observations (
    portfolio_id, observation_id, denomination_listing_id, amount
) VALUES (
    sqlc.arg(portfolio_id), sqlc.arg(observation_id), sqlc.arg(denomination_listing_id), sqlc.arg(amount)
);

-- name: UpdateReportedEquityObservation :exec
UPDATE reported_equity_observations
SET amount = sqlc.arg(amount)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND observation_id = sqlc.arg(observation_id);

-- name: ListReportedEquityCoveredIdentities :many
SELECT identity_id
FROM reported_equity_covered_identities
WHERE portfolio_id = sqlc.arg(portfolio_id) AND observation_id = sqlc.arg(observation_id)
ORDER BY identity_id;

-- name: InsertReportedEquityCoveredIdentity :exec
INSERT INTO reported_equity_covered_identities (portfolio_id, observation_id, identity_id)
VALUES (sqlc.arg(portfolio_id), sqlc.arg(observation_id), sqlc.arg(identity_id));

-- name: ListReportedEquityCoveredAccounts :many
SELECT account_id
FROM reported_equity_covered_accounts
WHERE portfolio_id = sqlc.arg(portfolio_id) AND observation_id = sqlc.arg(observation_id)
ORDER BY account_id;

-- name: InsertReportedEquityCoveredAccount :exec
INSERT INTO reported_equity_covered_accounts (portfolio_id, observation_id, account_id)
VALUES (sqlc.arg(portfolio_id), sqlc.arg(observation_id), sqlc.arg(account_id));

-- name: GetPriceObservation :one
SELECT portfolio_id, observation_id, instrument_id, base_listing_id, quote_listing_id, price_type, price
FROM price_observations
WHERE portfolio_id = sqlc.arg(portfolio_id) AND observation_id = sqlc.arg(observation_id);

-- name: InsertPriceObservation :exec
INSERT INTO price_observations (
    portfolio_id, observation_id, instrument_id, base_listing_id, quote_listing_id, price_type, price
) VALUES (
    sqlc.arg(portfolio_id), sqlc.arg(observation_id), sqlc.narg(instrument_id), sqlc.narg(base_listing_id),
    sqlc.arg(quote_listing_id), sqlc.arg(price_type), sqlc.arg(price)
);

-- name: UpdatePriceObservation :exec
UPDATE price_observations
SET price = sqlc.arg(price)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND observation_id = sqlc.arg(observation_id);

-- name: GetAccountMetricObservation :one
SELECT portfolio_id, observation_id, metric_code, unit, decimal_value, text_value, semantics_version
FROM account_metric_observations
WHERE portfolio_id = sqlc.arg(portfolio_id) AND observation_id = sqlc.arg(observation_id);

-- name: InsertAccountMetricObservation :exec
INSERT INTO account_metric_observations (
    portfolio_id, observation_id, metric_code, unit, decimal_value, text_value, semantics_version
) VALUES (
    sqlc.arg(portfolio_id), sqlc.arg(observation_id), sqlc.arg(metric_code), sqlc.arg(unit),
    sqlc.narg(decimal_value), sqlc.narg(text_value), sqlc.arg(semantics_version)
);

-- name: UpdateAccountMetricObservation :exec
UPDATE account_metric_observations
SET decimal_value = sqlc.narg(decimal_value),
    text_value = sqlc.narg(text_value)
WHERE portfolio_id = sqlc.arg(portfolio_id) AND observation_id = sqlc.arg(observation_id);
