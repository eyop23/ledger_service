-- name: CreateEntry :one
INSERT INTO entries (id, transaction_id, account_id, direction, amount, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetEntriesByTransaction :many
SELECT * FROM entries
WHERE transaction_id = $1
ORDER BY created_at ASC;

-- name: GetBalance :one
SELECT (
    COALESCE(SUM(CASE WHEN direction = 'CREDIT' THEN amount ELSE 0 END), 0::BIGINT) -
    COALESCE(SUM(CASE WHEN direction = 'DEBIT'  THEN amount ELSE 0 END), 0::BIGINT)
)::BIGINT AS balance
FROM entries
WHERE account_id = $1;

-- name: GetEntriesByAccount :many
SELECT * FROM entries
WHERE account_id = $1
  AND (created_at, id) < ($2::timestamptz, $3::uuid)
ORDER BY created_at DESC, id DESC
LIMIT $4;
