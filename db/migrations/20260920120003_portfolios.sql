-- +goose Up
CREATE TABLE portfolios (
    id BINARY(16) NOT NULL,
    user_id BINARY(16) NOT NULL,
    name VARCHAR(120) NOT NULL,
    timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Shanghai',
    display_asset_id BINARY(16) NOT NULL,
    status VARCHAR(32) NOT NULL,
    active_projection_generation_id BINARY(16) NULL,
    version BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    archived_at DATETIME(6) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_portfolios_user_id (user_id, id),
    UNIQUE KEY uq_portfolios_user_name (user_id, name),
    KEY idx_portfolios_user (user_id),
    KEY idx_portfolios_status (status),
    CONSTRAINT chk_portfolios_status CHECK (status IN ('active', 'archived')),
    CONSTRAINT fk_portfolios_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT fk_portfolios_display_asset FOREIGN KEY (display_asset_id) REFERENCES assets (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS portfolios;
