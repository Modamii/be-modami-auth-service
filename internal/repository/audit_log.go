package repository

import (
	"context"
	"fmt"

	"be-modami-auth-service/pkg/db/sqlc"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditLogRepository struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func NewAuditLogRepository(pool *pgxpool.Pool) *AuditLogRepository {
	return &AuditLogRepository{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

func (r *AuditLogRepository) WithTx(ctx context.Context, fn func(q *sqlc.Queries) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := fn(sqlc.New(tx)); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *AuditLogRepository) Create(ctx context.Context, params sqlc.CreateAuditLogParams) (sqlc.AuditLog, error) {
	return r.queries.CreateAuditLog(ctx, params)
}

func (r *AuditLogRepository) ListByUser(ctx context.Context, params sqlc.ListAuditLogsByUserParams) ([]sqlc.AuditLog, error) {
	return r.queries.ListAuditLogsByUser(ctx, params)
}

func (r *AuditLogRepository) CountByUser(ctx context.Context, userID interface{}) (int64, error) {
	return r.queries.CountAuditLogsByUser(ctx, userID)
}
