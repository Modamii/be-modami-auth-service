-- name: CreateAuditLog :one
INSERT INTO audit_logs (user_id, action, detail, ip_address)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, action, detail, ip_address, created_at;

-- name: ListAuditLogsByUser :many
SELECT id, user_id, action, detail, ip_address, created_at
FROM audit_logs
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountAuditLogsByUser :one
SELECT COUNT(*) FROM audit_logs WHERE user_id = $1;
