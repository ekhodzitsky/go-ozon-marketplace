package postgres

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/unitofwork"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UnitOfWork struct {
	pool       *pgxpool.Pool
	tx         pgx.Tx
	orderRepo  repository.OrderRepository
	outboxRepo repository.OutboxRepository
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
	u.orderRepo = postgres.NewOrderPostgres(tx)
	u.outboxRepo = postgres.NewOutboxPostgres(tx)
	return nil
}

func (u *UnitOfWork) Commit(ctx context.Context) error {
	return u.tx.Commit(ctx)
}

func (u *UnitOfWork) Rollback(ctx context.Context) error {
	return u.tx.Rollback(ctx)
}

func (u *UnitOfWork) OrderRepo() repository.OrderRepository  { return u.orderRepo }
func (u *UnitOfWork) OutboxRepo() repository.OutboxRepository { return u.outboxRepo }
