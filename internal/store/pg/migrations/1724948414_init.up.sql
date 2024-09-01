CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users
(
    id       SERIAL PRIMARY KEY,
    username VARCHAR(20) UNIQUE NOT NULL,
    password TEXT NOT NULL -- stores the hashed, salted passwords
);

INSERT INTO users (username, password)
VALUES
    ('alice', crypt('alice', gen_salt('bf'))),
    ('bob', crypt('bob', gen_salt('bf'))),
    ('carol', crypt('carol', gen_salt('bf'))),
    ('dave', crypt('dave', gen_salt('bf')));

-- composite index for fast lookups by username and password for auth case
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
    value            TEXT        NOT NULL
);

CREATE TABLE IF NOT EXISTS user_transactions
(
    user_id  INT NOT NULL,
    tx_hash  VARCHAR(66) NOT NULL,
    PRIMARY KEY (user_id, tx_hash),
    FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    FOREIGN KEY (tx_hash) REFERENCES transactions (tx_hash) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_transactions_user_id ON user_transactions (user_id);
