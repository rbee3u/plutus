-- +goose Up
ALTER TABLE portfolios
    ADD COLUMN protected BOOLEAN NOT NULL DEFAULT FALSE AFTER status;

UPDATE portfolios p
INNER JOIN (
    SELECT user_id, MIN(id) AS id
    FROM portfolios
    GROUP BY user_id
) first_row ON p.id = first_row.id
SET p.protected = TRUE;

-- +goose Down
ALTER TABLE portfolios DROP COLUMN protected;
