-- +goose Up
ALTER TABLE users
    ADD COLUMN password_hash VARCHAR(255) NULL AFTER status;

UPDATE users u
INNER JOIN user_credentials c ON c.user_id = u.id
SET u.password_hash = c.password_hash;

ALTER TABLE users
    MODIFY COLUMN password_hash VARCHAR(255) NOT NULL;

DROP TABLE user_credentials;

-- +goose Down
CREATE TABLE user_credentials (
    user_id BINARY(16) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    password_changed_at DATETIME(6) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    PRIMARY KEY (user_id),
    CONSTRAINT fk_user_credentials_user FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO user_credentials (user_id, password_hash, password_changed_at, created_at, updated_at)
SELECT id, password_hash, updated_at, created_at, updated_at FROM users;

ALTER TABLE users DROP COLUMN password_hash;
