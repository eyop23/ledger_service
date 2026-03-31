-- +goose Up
CREATE TABLE entries (
    id              UUID PRIMARY KEY,
    transaction_id  UUID NOT NULL REFERENCES transactions(id),
    account_id      UUID NOT NULL REFERENCES accounts(id),
    direction       TEXT NOT NULL CHECK (direction IN ('DEBIT', 'CREDIT')),
    amount          BIGINT NOT NULL CHECK (amount > 0),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_entries_account_id ON entries(account_id, created_at DESC);
CREATE INDEX idx_entries_transaction_id ON entries(transaction_id);

-- +goose Down
DROP TABLE entries;
