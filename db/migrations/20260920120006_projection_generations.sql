-- +goose Up
CREATE TABLE projection_generations (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    job_id BINARY(16) NOT NULL,
    status VARCHAR(32) NOT NULL,
    timezone VARCHAR(64) NOT NULL,
    display_asset_id BINARY(16) NOT NULL,
    source_watermark_at DATETIME(6) NOT NULL,
    started_at DATETIME(6) NOT NULL,
    completed_at DATETIME(6) NULL,
    activated_at DATETIME(6) NULL,
    failure_code VARCHAR(64) NULL,
    failure_detail VARCHAR(1024) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_projection_generations_portfolio_id (portfolio_id, id),
    CONSTRAINT chk_projection_generations_status CHECK (status IN ('building', 'ready', 'active', 'failed', 'retired')),
    CONSTRAINT fk_projection_generations_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT fk_projection_generations_job FOREIGN KEY (portfolio_id, job_id) REFERENCES jobs (portfolio_id, id) ON DELETE RESTRICT,
    CONSTRAINT fk_projection_generations_asset FOREIGN KEY (display_asset_id) REFERENCES assets (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE portfolios
    ADD CONSTRAINT fk_portfolios_active_generation
        FOREIGN KEY (id, active_projection_generation_id)
        REFERENCES projection_generations (portfolio_id, id) ON DELETE RESTRICT;

-- +goose Down
ALTER TABLE portfolios DROP FOREIGN KEY fk_portfolios_active_generation;
DROP TABLE IF EXISTS projection_generations;
