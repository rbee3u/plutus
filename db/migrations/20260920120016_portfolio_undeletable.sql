-- +goose Up
ALTER TABLE portfolios
    RENAME COLUMN protected TO undeletable;

-- +goose Down
ALTER TABLE portfolios
    RENAME COLUMN undeletable TO protected;
