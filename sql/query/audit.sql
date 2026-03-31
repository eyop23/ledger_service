-- name: CreateAuditLog :one
INSERT INTO audit_log (id, actor, action, entity_type, entity_id, payload, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetAuditLogs :many
SELECT * FROM audit_log
WHERE
    ($1::text        IS NULL OR entity_type = $1)
    AND ($2::uuid        IS NULL OR entity_id   = $2)
    AND ($3::timestamptz IS NULL OR created_at  >= $3)
    AND ($4::timestamptz IS NULL OR created_at  <= $4)
    AND (created_at, id) < ($5, $6)
ORDER BY created_at DESC, id DESC
LIMIT $7;
