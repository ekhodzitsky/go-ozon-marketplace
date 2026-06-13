package postgres

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/unitofwork"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UnitOfWork struct {
	pool        *pgxpool.Pool
	tx          pgx.Tx
	productRepo repository.ProductRepository
	outboxRepo  repository.OutboxRepository
}

func NewUnitOfWork(pool *pgxpool.Pool) unitofwork.UnitOfWork {
	return &UnitOfWork{pool: pool}
}

func (u *UnitOfWork) Begin(ctx context.Context) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return err
	}
	u.tx = tx
	u.productRepo = postgres.NewProductPostgres(tx)
	u.outboxRepo = postgres.NewOutboxPostgres(tx)
	return nil
}

func (u *UnitOfWork) Commit(ctx context.Context) error {
	return u.tx.Commit(ctx)
}

func (u *UnitOfWork) Rollback(ctx context.Context) error {
	if u.tx == nil {
		return nil
	}
	return u.tx.Rollback(ctx)
}

func (u *UnitOfWork) ProductRepo() repository.ProductRepository { return u.productRepo }
func (u *UnitOfWork) OutboxRepo() repository.OutboxRepository   { return u.outboxRepo }
