-- +goose Up
UPDATE identities i
LEFT JOIN accounts a ON a.portfolio_id = i.portfolio_id AND a.identity_id = i.id AND a.account_type = 'manual'
SET i.version = i.version + 1, i.updated_at = UTC_TIMESTAMP(6)
WHERE a.id IS NULL;

INSERT INTO accounts (id, portfolio_id, identity_id, account_type, display_name, selected, version, created_at, updated_at)
SELECT UUID_TO_BIN(UUID()), i.portfolio_id, i.id, 'manual', 'Manual', TRUE, 1, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6)
FROM identities i
LEFT JOIN accounts a ON a.portfolio_id = i.portfolio_id AND a.identity_id = i.id AND a.account_type = 'manual'
WHERE a.id IS NULL;

CREATE TABLE manual_balances (
    portfolio_id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    symbol VARCHAR(64) NOT NULL,
    listing_id BINARY(16) NOT NULL,
    quantity DECIMAL(65,30) NOT NULL,
    modified_at DATETIME(6) NOT NULL,
    PRIMARY KEY (account_id, symbol),
    CONSTRAINT fk_manual_balances_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_manual_balances_listing FOREIGN KEY (listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT,
    CONSTRAINT chk_manual_quantity CHECK (quantity <> 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE manual_revisions (
    portfolio_id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    version BIGINT UNSIGNED NOT NULL,
    balances_json JSON NOT NULL,
    modified_at DATETIME(6) NOT NULL,
    PRIMARY KEY (account_id, version),
    CONSTRAINT fk_manual_revisions_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE manual_cycles (
    portfolio_id BINARY(16) NOT NULL,
    job_id BINARY(16) NOT NULL,
    captured_at DATETIME(6) NOT NULL,
    PRIMARY KEY (job_id),
    CONSTRAINT fk_manual_cycles_job FOREIGN KEY (portfolio_id, job_id) REFERENCES jobs (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE manual_cycle_accounts (
    portfolio_id BINARY(16) NOT NULL,
    job_id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    version BIGINT UNSIGNED NOT NULL,
    balances_json JSON NOT NULL,
    modified_at DATETIME(6) NULL,
    PRIMARY KEY (job_id, account_id),
    CONSTRAINT fk_manual_cycle_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_manual_cycle_job FOREIGN KEY (job_id) REFERENCES manual_cycles (job_id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE account_reports
    ADD COLUMN manual_state_json JSON NULL,
    ADD COLUMN manual_modified_at DATETIME(6) NULL;

-- +goose Down
-- Operational upgrades are append-only. An explicit downgrade removes the new
-- ledger tables but retains Identity Accounts and their versions.
DROP TABLE manual_cycle_accounts;
DROP TABLE manual_cycles;
DROP TABLE manual_revisions;
DROP TABLE manual_balances;
ALTER TABLE account_reports
    DROP COLUMN manual_state_json,
    DROP COLUMN manual_modified_at;
