-- +goose Up
CREATE TABLE audit_events (
    id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NULL,
    actor_user_id BINARY(16) NULL,
    action VARCHAR(64) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id BINARY(16) NOT NULL,
    request_id VARCHAR(64) NOT NULL,
    before_json JSON NULL,
    after_json JSON NULL,
    metadata_json JSON NULL,
    occurred_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    KEY idx_audit_events_portfolio_time (portfolio_id, occurred_at),
    KEY idx_audit_events_actor_time (actor_user_id, occurred_at),
    KEY idx_audit_events_resource (resource_type, resource_id),
    CONSTRAINT fk_audit_events_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT,
    CONSTRAINT fk_audit_events_actor FOREIGN KEY (actor_user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE idempotency_records (
    id BINARY(16) NOT NULL,
    user_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NULL,
    `key` VARCHAR(255) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    method VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    path VARCHAR(512) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
    request_hash BINARY(32) NOT NULL,
    response_status SMALLINT UNSIGNED NOT NULL,
    response_headers_json JSON NULL,
    response_body_json JSON NULL,
    created_at DATETIME(6) NOT NULL,
    expires_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_idempotency_user_request (user_id, method, path, `key`),
    KEY idx_idempotency_expires (expires_at),
    CONSTRAINT fk_idempotency_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT fk_idempotency_portfolio FOREIGN KEY (portfolio_id) REFERENCES portfolios (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE IF EXISTS idempotency_records;
DROP TABLE IF EXISTS audit_events;
