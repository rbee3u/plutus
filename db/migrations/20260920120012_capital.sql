-- +goose Up
CREATE TABLE capital_baselines (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    current_revision_id BINARY(16) NULL,
    version BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_capital_baselines_portfolio_id (portfolio_id, id),
    UNIQUE KEY uq_capital_baselines_portfolio (portfolio_id),
    CONSTRAINT fk_capital_baselines_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE capital_baseline_revisions (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    capital_baseline_id BINARY(16) NOT NULL,
    effective_at DATETIME(6) NOT NULL,
    asset_id BINARY(16) NOT NULL,
    quantity DECIMAL(65, 30) NOT NULL,
    note VARCHAR(1024) NULL,
    created_by_user_id BINARY(16) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_cbr_portfolio_id (portfolio_id, id),
    KEY idx_cbr_baseline (portfolio_id, capital_baseline_id),
    CONSTRAINT fk_cbr_baseline FOREIGN KEY (portfolio_id, capital_baseline_id) REFERENCES capital_baselines (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_cbr_asset FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT,
    CONSTRAINT fk_cbr_user FOREIGN KEY (created_by_user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE capital_baselines
    ADD CONSTRAINT fk_capital_baselines_current_revision
        FOREIGN KEY (portfolio_id, current_revision_id)
        REFERENCES capital_baseline_revisions (portfolio_id, id) ON DELETE RESTRICT;

CREATE TABLE capital_flows (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    current_revision_id BINARY(16) NULL,
    version BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_capital_flows_portfolio_id (portfolio_id, id),
    CONSTRAINT fk_capital_flows_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE capital_flow_revisions (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    capital_flow_id BINARY(16) NOT NULL,
    direction VARCHAR(32) NOT NULL,
    quantity DECIMAL(65, 30) NOT NULL,
    asset_id BINARY(16) NOT NULL,
    effective_at DATETIME(6) NOT NULL,
    account_id BINARY(16) NULL,
    note VARCHAR(1024) NULL,
    state VARCHAR(32) NOT NULL,
    created_by_user_id BINARY(16) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_cfr_portfolio_id (portfolio_id, id),
    KEY idx_cfr_flow (portfolio_id, capital_flow_id),
    CONSTRAINT chk_cfr_direction CHECK (direction IN ('contribution', 'withdrawal')),
    CONSTRAINT chk_cfr_quantity CHECK (quantity > 0),
    CONSTRAINT chk_cfr_state CHECK (state IN ('active', 'deleted')),
    CONSTRAINT fk_cfr_flow FOREIGN KEY (portfolio_id, capital_flow_id) REFERENCES capital_flows (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_cfr_asset FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT,
    CONSTRAINT fk_cfr_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_cfr_user FOREIGN KEY (created_by_user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE capital_flows
    ADD CONSTRAINT fk_capital_flows_current_revision
        FOREIGN KEY (portfolio_id, current_revision_id)
        REFERENCES capital_flow_revisions (portfolio_id, id) ON DELETE RESTRICT;

-- +goose Down
ALTER TABLE capital_flows DROP FOREIGN KEY fk_capital_flows_current_revision;
ALTER TABLE capital_baselines DROP FOREIGN KEY fk_capital_baselines_current_revision;
DROP TABLE IF EXISTS capital_flow_revisions;
DROP TABLE IF EXISTS capital_flows;
DROP TABLE IF EXISTS capital_baseline_revisions;
DROP TABLE IF EXISTS capital_baselines;
