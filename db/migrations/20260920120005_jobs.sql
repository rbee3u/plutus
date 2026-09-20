-- +goose Up
CREATE TABLE jobs (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    priority SMALLINT NOT NULL,
    dedupe_key VARCHAR(255) NOT NULL,
    active_dedupe_key VARCHAR(255) AS (IF(status IN ('queued', 'running'), dedupe_key, NULL)) STORED,
    requested_by_user_id BINARY(16) NULL,
    available_at DATETIME(6) NOT NULL,
    lease_owner VARCHAR(191) NULL,
    lease_expires_at DATETIME(6) NULL,
    attempt_count INT UNSIGNED NOT NULL,
    max_attempts INT UNSIGNED NOT NULL,
    last_error_code VARCHAR(64) NULL,
    last_error_detail VARCHAR(1024) NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    started_at DATETIME(6) NULL,
    finished_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_jobs_portfolio_id (portfolio_id, id),
    UNIQUE KEY uq_jobs_active_dedupe (portfolio_id, active_dedupe_key),
    KEY idx_jobs_claim (status, available_at, priority, lease_expires_at),
    CONSTRAINT chk_jobs_kind CHECK (kind IN ('collection', 'projection')),
    CONSTRAINT chk_jobs_status CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
    CONSTRAINT fk_jobs_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT fk_jobs_requested_by FOREIGN KEY (requested_by_user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE job_attempts (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    job_id BINARY(16) NOT NULL,
    attempt_number INT UNSIGNED NOT NULL,
    worker_id VARCHAR(191) NOT NULL,
    started_at DATETIME(6) NOT NULL,
    finished_at DATETIME(6) NULL,
    outcome VARCHAR(32) NOT NULL,
    error_class VARCHAR(64) NULL,
    error_code VARCHAR(64) NULL,
    safe_detail VARCHAR(1024) NULL,
    retry_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_job_attempts_portfolio_id (portfolio_id, id),
    UNIQUE KEY uq_job_attempts_number (portfolio_id, job_id, attempt_number),
    KEY idx_job_attempts_finished (finished_at),
    CONSTRAINT chk_job_attempts_outcome CHECK (outcome IN ('succeeded', 'failed', 'cancelled')),
    CONSTRAINT fk_job_attempts_job FOREIGN KEY (portfolio_id, job_id) REFERENCES jobs (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE projection_job_details (
    portfolio_id BINARY(16) NOT NULL,
    job_id BINARY(16) NOT NULL,
    invalid_from DATETIME(6) NULL,
    invalid_through DATETIME(6) NULL,
    reason VARCHAR(1024) NOT NULL,
    PRIMARY KEY (portfolio_id, job_id),
    CONSTRAINT fk_projection_job_details_job FOREIGN KEY (portfolio_id, job_id) REFERENCES jobs (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE projection_job_kinds (
    portfolio_id BINARY(16) NOT NULL,
    job_id BINARY(16) NOT NULL,
    projection_kind VARCHAR(64) NOT NULL,
    PRIMARY KEY (portfolio_id, job_id, projection_kind),
    CONSTRAINT fk_projection_job_kinds_details FOREIGN KEY (portfolio_id, job_id) REFERENCES projection_job_details (portfolio_id, job_id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS projection_job_kinds;
DROP TABLE IF EXISTS projection_job_details;
DROP TABLE IF EXISTS job_attempts;
DROP TABLE IF EXISTS jobs;
