-- +goose Up
CREATE TABLE assets (
    id BINARY(16) NOT NULL,
    code VARCHAR(64) NOT NULL,
    name VARCHAR(191) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    decimals SMALLINT UNSIGNED NULL,
    status VARCHAR(32) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_assets_code (code),
    CONSTRAINT chk_assets_kind CHECK (kind IN ('fiat', 'crypto', 'other')),
    CONSTRAINT chk_assets_status CHECK (status IN ('active', 'provisional', 'retired'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE platforms (
    id BINARY(16) NOT NULL,
    code VARCHAR(32) NOT NULL,
    name VARCHAR(120) NOT NULL,
    status VARCHAR(32) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_platforms_code (code),
    CONSTRAINT chk_platforms_status CHECK (status IN ('planned', 'enabled', 'disabled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO assets (id, code, name, kind, decimals, status, created_at, updated_at) VALUES
    (UNHEX('01A0BDFC1FA47A549FA124F4721A71AF'), 'USD', 'US Dollar', 'fiat', 2, 'active', '2026-01-01 00:00:00.000000', '2026-01-01 00:00:00.000000'),
    (UNHEX('01A0BDFC1FA47AF4BEDF65D968B9122B'), 'CNY', 'Chinese Yuan', 'fiat', 2, 'active', '2026-01-01 00:00:00.000000', '2026-01-01 00:00:00.000000');

INSERT INTO platforms (id, code, name, status, created_at, updated_at) VALUES
    (UNHEX('01A0BDFC1FA47B00B990A610A553E10C'), 'binance', 'Binance', 'enabled', '2026-01-01 00:00:00.000000', '2026-01-01 00:00:00.000000'),
    (UNHEX('01A0BDFC1FA47B08BC44A2A59E4665B8'), 'okx', 'OKX', 'enabled', '2026-01-01 00:00:00.000000', '2026-01-01 00:00:00.000000'),
    (UNHEX('01A0BDFC1FA47B10990C04AA9D8AC659'), 'bybit', 'Bybit', 'enabled', '2026-01-01 00:00:00.000000', '2026-01-01 00:00:00.000000');

-- +goose Down
DROP TABLE IF EXISTS platforms;
DROP TABLE IF EXISTS assets;
