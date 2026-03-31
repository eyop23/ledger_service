-- +goose Up
CREATE TABLE transactions (
    id               UUID PRIMARY KEY,
    idempotency_key  TEXT NOT NULL UNIQUE,
    amount           BIGINT NOT NULL CHECK (amount > 0),
    currency         CHAR(3) NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('posted', 'rejected')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE transactions;
