-- +goose Up
CREATE TABLE collection_job_details (
    portfolio_id BINARY(16) NOT NULL,
    job_id BINARY(16) NOT NULL,
    connection_id BINARY(16) NOT NULL,
    `trigger` VARCHAR(32) NOT NULL,
    scope VARCHAR(64) NOT NULL,
    PRIMARY KEY (portfolio_id, job_id),
    CONSTRAINT chk_collection_job_trigger CHECK (`trigger` IN ('schedule', 'manual', 'retry')),
    CONSTRAINT fk_collection_job_details_job FOREIGN KEY (portfolio_id, job_id) REFERENCES jobs (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_collection_job_details_connection FOREIGN KEY (portfolio_id, connection_id) REFERENCES connections (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE collection_runs (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    job_id BINARY(16) NOT NULL,
    connection_id BINARY(16) NOT NULL,
    identity_id BINARY(16) NOT NULL,
    adapter_name VARCHAR(64) NOT NULL,
    adapter_version VARCHAR(64) NOT NULL,
    started_at DATETIME(6) NOT NULL,
    finished_at DATETIME(6) NULL,
    status VARCHAR(32) NOT NULL,
    successful_scope_count INT UNSIGNED NOT NULL,
    failed_scope_count INT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_collection_runs_portfolio_id (portfolio_id, id),
    KEY idx_collection_runs_identity_started (portfolio_id, identity_id, started_at),
    CONSTRAINT chk_collection_runs_status CHECK (status IN ('running', 'succeeded', 'partial', 'failed')),
    CONSTRAINT fk_collection_runs_job FOREIGN KEY (portfolio_id, job_id) REFERENCES jobs (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_collection_runs_connection FOREIGN KEY (portfolio_id, connection_id) REFERENCES connections (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_collection_runs_identity FOREIGN KEY (portfolio_id, identity_id) REFERENCES identities (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE identity_snapshots (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    collection_run_id BINARY(16) NOT NULL,
    identity_id BINARY(16) NOT NULL,
    started_at DATETIME(6) NOT NULL,
    completed_at DATETIME(6) NULL,
    status VARCHAR(32) NOT NULL,
    completeness VARCHAR(32) NOT NULL,
    expected_scope_count INT UNSIGNED NOT NULL,
    successful_scope_count INT UNSIGNED NOT NULL,
    failed_scope_count INT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_identity_snapshots_portfolio_id (portfolio_id, id),
    KEY idx_identity_snapshots_identity (portfolio_id, identity_id, started_at),
    CONSTRAINT chk_identity_snapshots_status CHECK (status IN ('complete', 'partial', 'failed')),
    CONSTRAINT chk_identity_snapshots_completeness CHECK (completeness IN ('complete', 'partial')),
    CONSTRAINT fk_identity_snapshots_run FOREIGN KEY (portfolio_id, collection_run_id) REFERENCES collection_runs (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_identity_snapshots_identity FOREIGN KEY (portfolio_id, identity_id) REFERENCES identities (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE collection_failures (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    collection_run_id BINARY(16) NOT NULL,
    identity_snapshot_id BINARY(16) NOT NULL,
    account_id BINARY(16) NULL,
    scope_kind VARCHAR(64) NOT NULL,
    scope_ref VARCHAR(191) NULL,
    phase VARCHAR(64) NOT NULL,
    error_class VARCHAR(64) NOT NULL,
    error_code VARCHAR(64) NOT NULL,
    safe_detail VARCHAR(1024) NOT NULL,
    retriable BOOLEAN NOT NULL,
    occurred_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_collection_failures_portfolio_id (portfolio_id, id),
    KEY idx_collection_failures_run (portfolio_id, collection_run_id),
    CONSTRAINT fk_collection_failures_run FOREIGN KEY (portfolio_id, collection_run_id) REFERENCES collection_runs (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_collection_failures_snapshot FOREIGN KEY (portfolio_id, identity_snapshot_id) REFERENCES identity_snapshots (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_collection_failures_account FOREIGN KEY (portfolio_id, account_id) REFERENCES accounts (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS collection_failures;
DROP TABLE IF EXISTS identity_snapshots;
DROP TABLE IF EXISTS collection_runs;
DROP TABLE IF EXISTS collection_job_details;
