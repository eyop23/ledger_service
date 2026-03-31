-- +goose Up
CREATE TABLE accounts(
  id UUID PRIMARY KEY,   
  currency CHAR(3) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE accounts;