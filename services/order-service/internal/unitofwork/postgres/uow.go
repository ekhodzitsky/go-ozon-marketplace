package postgres

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/unitofwork"
	"github.com/jackc/pgx/v5"
)

type UnitOfWork struct {
	orderRepo  repository.OrderRepository
	outboxRepo repository.OutboxRepository
}

func NewUnitOfWork(tx pgx.Tx) unitofwork.UnitOfWork {
	return &UnitOfWork{
		orderRepo:  postgres.NewOrderPostgres(tx),
		outboxRepo: postgres.NewOutboxPostgres(tx),
	}
}

func (u *UnitOfWork) OrderRepo() repository.OrderRepository   { return u.orderRepo }
func (u *UnitOfWork) OutboxRepo() repository.OutboxRepository { return u.outboxRepo }
