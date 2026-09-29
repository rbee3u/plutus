-- +goose Up
ALTER TABLE dual_positions ADD COLUMN subscribed_at DATETIME(6) NULL;

-- +goose Down
ALTER TABLE dual_positions DROP COLUMN subscribed_at;
