-- +goose Up
-- Only missing accounts are added. Existing selections must never be overwritten.
UPDATE identities i
JOIN accounts e ON e.identity_id = i.id AND e.portfolio_id = i.portfolio_id AND e.account_type = 'earn'
LEFT JOIN accounts d ON d.identity_id = i.id AND d.portfolio_id = i.portfolio_id AND d.account_type = 'dual_investment'
SET i.version = i.version + 1, i.updated_at = UTC_TIMESTAMP(6)
WHERE i.platform IN ('binance', 'okx', 'bybit') AND d.id IS NULL;

INSERT INTO accounts (id, portfolio_id, identity_id, account_type, display_name, selected, version, created_at, updated_at)
SELECT UUID_TO_BIN(UUID()), i.portfolio_id, i.id, 'dual_investment', 'Dual Investment', e.selected, 1, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)
FROM identities i
JOIN accounts e ON e.identity_id = i.id AND e.portfolio_id = i.portfolio_id AND e.account_type = 'earn'
LEFT JOIN accounts d ON d.identity_id = i.id AND d.portfolio_id = i.portfolio_id AND d.account_type = 'dual_investment'
WHERE i.platform IN ('binance', 'okx', 'bybit') AND d.id IS NULL;

-- +goose Down
-- Configuration and collected history are intentionally retained.
SELECT 1;
