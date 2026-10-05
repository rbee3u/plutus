-- +goose Up
ALTER TABLE account_reports ADD COLUMN earn_details_collected BOOLEAN NOT NULL DEFAULT FALSE;
CREATE TABLE earn_positions (
    report_id BINARY(16) NOT NULL,
    portfolio_id BINARY(16) NOT NULL,
    platform_position_ref VARCHAR(128) COLLATE utf8mb4_bin NOT NULL,
    observation_json JSON NOT NULL,
    PRIMARY KEY (report_id, platform_position_ref),
    CONSTRAINT fk_earn_positions_report FOREIGN KEY (portfolio_id, report_id)
        REFERENCES account_reports (portfolio_id, id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- +goose Down
DROP TABLE earn_positions;
ALTER TABLE account_reports DROP COLUMN earn_details_collected;
