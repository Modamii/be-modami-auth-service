package repository

import (
	"context"
	"net/netip"

	"be-modami-auth-service/internal/entity"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditLogRepository struct {
	pool *pgxpool.Pool
}

func NewAuditLogRepository(pool *pgxpool.Pool) *AuditLogRepository {
	return &AuditLogRepository{pool: pool}
}

type CreateAuditLogParams struct {
	UserID    pgtype.UUID
	Action    string
	Detail    []byte
	IPAddress *netip.Prefix
}

type ListAuditLogsByUserParams struct {
	UserID pgtype.UUID
	Limit  int32
	Offset int32
}

func (r *AuditLogRepository) Create(ctx context.Context, params CreateAuditLogParams) (entity.AuditLog, error) {
	const q = `
		INSERT INTO audit_logs (user_id, action, detail, ip_address)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, action, detail, ip_address, created_at`

	var a entity.AuditLog
	err := r.pool.QueryRow(ctx, q, params.UserID, params.Action, params.Detail, params.IPAddress).
		Scan(&a.ID, &a.UserID, &a.Action, &a.Detail, &a.IPAddress, &a.CreatedAt)
	return a, err
}

func (r *AuditLogRepository) ListByUser(ctx context.Context, params ListAuditLogsByUserParams) ([]entity.AuditLog, error) {
	const q = `
		SELECT id, user_id, action, detail, ip_address, created_at
		FROM audit_logs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, q, params.UserID, params.Limit, params.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []entity.AuditLog
	for rows.Next() {
		var a entity.AuditLog
		if err := rows.Scan(&a.ID, &a.UserID, &a.Action, &a.Detail, &a.IPAddress, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func (r *AuditLogRepository) CountByUser(ctx context.Context, userID pgtype.UUID) (int64, error) {
	const q = `SELECT COUNT(*) FROM audit_logs WHERE user_id = $1`

	var count int64
	err := r.pool.QueryRow(ctx, q, userID).Scan(&count)
	return count, err
}
