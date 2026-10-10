-- name: DeletePortfolioAccountValuations :execrows
DELETE FROM account_valuations WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioBalances :execrows
DELETE FROM balances WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioPositions :execrows
DELETE FROM futures_positions WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioEarnPositions :execrows
DELETE FROM earn_positions WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioDualPositions :execrows
DELETE FROM dual_positions WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioManualRevisions :execrows
DELETE FROM manual_revisions WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioCapitalFlows :execrows
DELETE FROM capital_flows WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioValuationPoints :execrows
DELETE FROM valuation_points WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioAuditEvents :execrows
DELETE FROM audit_events WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioAssetExclusionVersions :execrows
DELETE FROM asset_exclusion_versions WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioAccountReports :execrows
DELETE FROM account_reports WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioAccounts :execrows
DELETE FROM accounts WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioCollectionCycles :execrows
DELETE FROM collection_cycles WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioIdentities :execrows
DELETE FROM identities WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioBoundaryEvents :execrows
DELETE FROM boundary_events WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioBalanceValuations :execrows
DELETE FROM balance_valuations WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioManualRevisionBalances :execrows
DELETE FROM manual_revision_balances WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioFuturesContracts :execrows
DELETE FROM futures_contracts WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioAssets :execrows
DELETE FROM assets WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioPriceObservations :execrows
DELETE FROM price_observations WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioManualAdjustments :execrows
DELETE FROM manual_adjustments WHERE portfolio_id = ? LIMIT 1000;

-- name: DeletePortfolioCalculatedAssetChanges :execrows
DELETE FROM calculated_asset_changes WHERE portfolio_id = ? LIMIT 1000;
