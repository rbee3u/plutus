-- +goose Up
ALTER TABLE account_reports ADD COLUMN scope_version SMALLINT UNSIGNED NOT NULL DEFAULT 1;
CREATE INDEX idx_account_reports_scope ON account_reports (portfolio_id, account_id, scope_version, status, created_at);

CREATE TABLE dual_positions (
    report_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    platform_position_ref VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    product_ref VARCHAR(128) NULL,
    order_ref VARCHAR(128) NULL,
    base_symbol VARCHAR(64) NULL,
    quote_symbol VARCHAR(64) NULL,
    invest_symbol VARCHAR(64) NOT NULL,
    principal DECIMAL(65,30) NOT NULL,
    direction VARCHAR(16) NULL,
    target_price DECIMAL(65,30) NULL,
    expiry_at DATETIME(6) NULL,
    annualized_rate DECIMAL(65,30) NULL,
    rate_kind VARCHAR(24) NULL,
    status VARCHAR(16) NOT NULL,
    PRIMARY KEY (report_id, platform_position_ref),
    CONSTRAINT fk_dual_positions_report FOREIGN KEY (portfolio_id, report_id) REFERENCES account_reports (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT chk_dual_principal CHECK (principal > 0),
    CONSTRAINT chk_dual_direction CHECK (direction IS NULL OR direction IN ('buy_low', 'sell_high')),
    CONSTRAINT chk_dual_status CHECK (status IN ('pending', 'active', 'settling', 'redeeming', 'refunding')),
    CONSTRAINT chk_dual_rate_kind CHECK (rate_kind IS NULL OR rate_kind IN ('apr', 'apy', 'annualized_yield'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE dual_positions;
DROP INDEX idx_account_reports_scope ON account_reports;
ALTER TABLE account_reports DROP COLUMN scope_version;
