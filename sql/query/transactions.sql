-- name: CreateTransaction :one
INSERT INTO transactions (id, idempotency_key, amount, currency, status, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetTransaction :one
SELECT * FROM transactions
WHERE id = $1;

-- name: GetTransactionByIdempotencyKey :one
SELECT * FROM transactions
WHERE idempotency_key = $1;
