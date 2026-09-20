-- +goose Up
CREATE TABLE latest_balance_projections (
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    identity_id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    asset_listing_id BINARY(16) NOT NULL,
    asset_id BINARY(16) NULL,
    observation_id BINARY(16) NOT NULL,
    total_quantity DECIMAL(65, 30) NULL,
    available_quantity DECIMAL(65, 30) NULL,
    locked_quantity DECIMAL(65, 30) NULL,
    source_observed_at DATETIME(6) NOT NULL,
    freshness_state VARCHAR(32) NOT NULL,
    completeness VARCHAR(32) NOT NULL,
    PRIMARY KEY (generation_id, account_id, asset_listing_id),
    CONSTRAINT chk_lbp_freshness CHECK (freshness_state IN ('fresh', 'stale', 'unknown')),
    CONSTRAINT chk_lbp_completeness CHECK (completeness IN ('complete', 'partial', 'missing')),
    CONSTRAINT fk_lbp_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_lbp_identity FOREIGN KEY (portfolio_id, identity_id) REFERENCES identities (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_lbp_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_lbp_listing FOREIGN KEY (asset_listing_id) REFERENCES asset_listings (id) ON DELETE RESTRICT,
    CONSTRAINT fk_lbp_asset FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT,
    CONSTRAINT fk_lbp_observation FOREIGN KEY (portfolio_id, observation_id) REFERENCES observations (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE latest_position_projections (
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    identity_id BINARY(16) NOT NULL,
    account_id BINARY(16) NOT NULL,
    instrument_id BINARY(16) NOT NULL,
    platform_position_ref VARCHAR(191) NOT NULL,
    side VARCHAR(32) NOT NULL,
    quantity DECIMAL(65, 30) NOT NULL,
    selected_mark_price DECIMAL(65, 30) NULL,
    underlying_long_quantity DECIMAL(65, 30) NOT NULL,
    underlying_short_quantity DECIMAL(65, 30) NOT NULL,
    observation_id BINARY(16) NOT NULL,
    source_observed_at DATETIME(6) NOT NULL,
    freshness_state VARCHAR(32) NOT NULL,
    completeness VARCHAR(32) NOT NULL,
    PRIMARY KEY (generation_id, account_id, instrument_id, platform_position_ref),
    CONSTRAINT chk_lpp_side CHECK (side IN ('long', 'short', 'net')),
    CONSTRAINT chk_lpp_freshness CHECK (freshness_state IN ('fresh', 'stale', 'unknown')),
    CONSTRAINT chk_lpp_completeness CHECK (completeness IN ('complete', 'partial', 'missing')),
    CONSTRAINT fk_lpp_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_lpp_identity FOREIGN KEY (portfolio_id, identity_id) REFERENCES identities (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_lpp_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_lpp_instrument FOREIGN KEY (instrument_id) REFERENCES instruments (id) ON DELETE RESTRICT,
    CONSTRAINT fk_lpp_observation FOREIGN KEY (portfolio_id, observation_id) REFERENCES observations (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE portfolio_valuation_projections (
    id BINARY(16) NOT NULL,
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    evaluated_at DATETIME(6) NOT NULL,
    denomination_asset_id BINARY(16) NOT NULL,
    amount DECIMAL(65, 30) NULL,
    completeness VARCHAR(32) NOT NULL,
    coverage_fingerprint VARCHAR(128) NOT NULL,
    freshness_state VARCHAR(32) NOT NULL,
    source_cutoff_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_pvp_portfolio_id (portfolio_id, id),
    UNIQUE KEY uq_pvp_eval (generation_id, evaluated_at, denomination_asset_id),
    KEY idx_pvp_generation_time (generation_id, evaluated_at),
    CONSTRAINT chk_pvp_completeness CHECK (completeness IN ('complete', 'partial', 'missing')),
    CONSTRAINT chk_pvp_freshness CHECK (freshness_state IN ('fresh', 'stale', 'unknown')),
    CONSTRAINT fk_pvp_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_pvp_asset FOREIGN KEY (denomination_asset_id) REFERENCES assets (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE valuation_contribution_projections (
    id BINARY(16) NOT NULL,
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    portfolio_valuation_id BINARY(16) NOT NULL,
    contribution_kind VARCHAR(32) NOT NULL,
    identity_id BINARY(16) NULL,
    account_id BINARY(16) NULL,
    asset_id BINARY(16) NULL,
    amount DECIMAL(65, 30) NULL,
    completeness VARCHAR(32) NOT NULL,
    coverage_fingerprint VARCHAR(128) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_vcp_portfolio_id (portfolio_id, id),
    KEY idx_vcp_valuation (portfolio_id, portfolio_valuation_id),
    CONSTRAINT chk_vcp_kind CHECK (contribution_kind IN ('reported_equity', 'calculated')),
    CONSTRAINT chk_vcp_completeness CHECK (completeness IN ('complete', 'partial', 'missing')),
    CONSTRAINT fk_vcp_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_vcp_valuation FOREIGN KEY (portfolio_id, portfolio_valuation_id) REFERENCES portfolio_valuation_projections (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_vcp_identity FOREIGN KEY (portfolio_id, identity_id) REFERENCES identities (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_vcp_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_vcp_asset FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE valuation_contribution_sources (
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    valuation_contribution_id BINARY(16) NOT NULL,
    observation_id BINARY(16) NOT NULL,
    source_role VARCHAR(64) NOT NULL,
    PRIMARY KEY (valuation_contribution_id, observation_id, source_role),
    CONSTRAINT fk_vcs_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_vcs_contribution FOREIGN KEY (portfolio_id, valuation_contribution_id) REFERENCES valuation_contribution_projections (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_vcs_observation FOREIGN KEY (portfolio_id, observation_id) REFERENCES observations (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE exposure_projections (
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    evaluated_at DATETIME(6) NOT NULL,
    asset_id BINARY(16) NOT NULL,
    long_quantity DECIMAL(65, 30) NOT NULL,
    short_quantity DECIMAL(65, 30) NOT NULL,
    net_quantity DECIMAL(65, 30) NOT NULL,
    long_value DECIMAL(65, 30) NULL,
    short_value DECIMAL(65, 30) NULL,
    net_value DECIMAL(65, 30) NULL,
    denomination_asset_id BINARY(16) NOT NULL,
    completeness VARCHAR(32) NOT NULL,
    freshness_state VARCHAR(32) NOT NULL,
    PRIMARY KEY (generation_id, evaluated_at, asset_id),
    CONSTRAINT chk_exp_completeness CHECK (completeness IN ('complete', 'partial', 'missing')),
    CONSTRAINT chk_exp_freshness CHECK (freshness_state IN ('fresh', 'stale', 'unknown')),
    CONSTRAINT fk_exp_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_exp_asset FOREIGN KEY (asset_id) REFERENCES assets (id) ON DELETE RESTRICT,
    CONSTRAINT fk_exp_denomination FOREIGN KEY (denomination_asset_id) REFERENCES assets (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE contributed_capital_projections (
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    evaluated_at DATETIME(6) NOT NULL,
    denomination_asset_id BINARY(16) NOT NULL,
    amount DECIMAL(65, 30) NULL,
    completeness VARCHAR(32) NOT NULL,
    source_revision_watermark BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (generation_id, evaluated_at, denomination_asset_id),
    CONSTRAINT chk_ccp_completeness CHECK (completeness IN ('complete', 'partial', 'missing')),
    CONSTRAINT fk_ccp_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_ccp_asset FOREIGN KEY (denomination_asset_id) REFERENCES assets (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE value_change_projections (
    id BINARY(16) NOT NULL,
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    period_kind VARCHAR(32) NOT NULL,
    period_start_date DATE NOT NULL,
    period_end_date DATE NOT NULL,
    as_of_at DATETIME(6) NOT NULL,
    beginning_valuation_id BINARY(16) NOT NULL,
    ending_valuation_id BINARY(16) NOT NULL,
    contribution_amount DECIMAL(65, 30) NULL,
    withdrawal_amount DECIMAL(65, 30) NULL,
    amount DECIMAL(65, 30) NULL,
    state VARCHAR(32) NOT NULL,
    coverage_fingerprint VARCHAR(128) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_vch_period (generation_id, period_kind, period_start_date, period_end_date, as_of_at),
    CONSTRAINT chk_vch_period_kind CHECK (period_kind IN ('day', 'iso_week', 'month', 'quarter', 'year')),
    CONSTRAINT chk_vch_state CHECK (state IN ('complete', 'provisional', 'incomplete', 'incomparable')),
    CONSTRAINT fk_vch_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_vch_begin FOREIGN KEY (portfolio_id, beginning_valuation_id) REFERENCES portfolio_valuation_projections (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_vch_end FOREIGN KEY (portfolio_id, ending_valuation_id) REFERENCES portfolio_valuation_projections (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE period_summary_projections (
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    period_kind VARCHAR(32) NOT NULL,
    period_start_date DATE NOT NULL,
    period_end_date DATE NOT NULL,
    as_of_at DATETIME(6) NOT NULL,
    valuation_open DECIMAL(65, 30) NULL,
    valuation_close DECIMAL(65, 30) NULL,
    valuation_high DECIMAL(65, 30) NULL,
    valuation_low DECIMAL(65, 30) NULL,
    contributed_capital_close DECIMAL(65, 30) NULL,
    value_change DECIMAL(65, 30) NULL,
    freshness_state VARCHAR(32) NOT NULL,
    completeness VARCHAR(32) NOT NULL,
    PRIMARY KEY (generation_id, period_kind, period_start_date, period_end_date, as_of_at),
    CONSTRAINT chk_psp_period_kind CHECK (period_kind IN ('day', 'iso_week', 'month', 'quarter', 'year')),
    CONSTRAINT chk_psp_freshness CHECK (freshness_state IN ('fresh', 'stale', 'unknown')),
    CONSTRAINT chk_psp_completeness CHECK (completeness IN ('complete', 'partial', 'missing')),
    CONSTRAINT fk_psp_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE projection_issues (
    id BINARY(16) NOT NULL,
    generation_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    projection_kind VARCHAR(64) NOT NULL,
    projection_key VARCHAR(255) NOT NULL,
    severity VARCHAR(32) NOT NULL,
    code VARCHAR(64) NOT NULL,
    summary VARCHAR(1024) NOT NULL,
    details_json JSON NULL,
    identity_id BINARY(16) NULL,
    account_id BINARY(16) NULL,
    observation_id BINARY(16) NULL,
    first_effective_at DATETIME(6) NOT NULL,
    last_effective_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    KEY idx_projection_issues_generation (generation_id, projection_kind),
    CONSTRAINT chk_projection_issues_severity CHECK (severity IN ('info', 'warning', 'error')),
    CONSTRAINT fk_projection_issues_generation FOREIGN KEY (portfolio_id, generation_id) REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_projection_issues_identity FOREIGN KEY (portfolio_id, identity_id) REFERENCES identities (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_projection_issues_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_projection_issues_observation FOREIGN KEY (portfolio_id, observation_id) REFERENCES observations (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS projection_issues;
DROP TABLE IF EXISTS period_summary_projections;
DROP TABLE IF EXISTS value_change_projections;
DROP TABLE IF EXISTS contributed_capital_projections;
DROP TABLE IF EXISTS exposure_projections;
DROP TABLE IF EXISTS valuation_contribution_sources;
DROP TABLE IF EXISTS valuation_contribution_projections;
DROP TABLE IF EXISTS portfolio_valuation_projections;
DROP TABLE IF EXISTS latest_position_projections;
DROP TABLE IF EXISTS latest_balance_projections;
