CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users
(
    id       SERIAL PRIMARY KEY,
    username VARCHAR(20) UNIQUE NOT NULL,
    password TEXT NOT NULL -- Stores the hashed, salted passwords
);

INSERT INTO users (username, password)
VALUES
    ('alice', crypt('alice', gen_salt('bf'))),
    ('bob', crypt('bob', gen_salt('bf'))),
    ('carol', crypt('carol', gen_salt('bf'))),
    ('dave', crypt('dave', gen_salt('bf')));

-- Composite index for fast lookups by username and password for auth case
CREATE INDEX IF NOT EXISTS idx_users_username_password ON users (username, password);

CREATE TABLE IF NOT EXISTS transactions
(
    tx_hash          VARCHAR(66) PRIMARY KEY,
    tx_status        INT         NOT NULL,
    block_hash       VARCHAR(66) NOT NULL,
    block_number     NUMERIC     NOT NULL,
    from_address     VARCHAR(42) NOT NULL,
    to_address       VARCHAR(42),
    contract_address VARCHAR(42),
    logs_count       BIGINT      NOT NULL,
    input            TEXT        NOT NULL,
    value            TEXT        NOT NULL,
    user_id          INT,
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- Index on user_id for fast lookups by user_id
CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON transactions (user_id);
