-- name: CreateAccount :one
INSERT INTO accounts (id, currency, created_at)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetAccount :one
SELECT * FROM accounts
WHERE id = $1;
