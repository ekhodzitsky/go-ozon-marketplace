package postgres

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository/postgres"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/unitofwork"
	"github.com/jackc/pgx/v5"
)

// UnitOfWork binds repository implementations to a single pgx transaction.
type UnitOfWork struct {
	productRepo repository.ProductRepository
	outboxRepo  repository.OutboxRepository
}

// NewUnitOfWork creates a UnitOfWork whose repositories operate on the provided tx.
func NewUnitOfWork(tx pgx.Tx) unitofwork.UnitOfWork {
	return &UnitOfWork{
		productRepo: postgres.NewProductPostgres(tx),
		outboxRepo:  postgres.NewOutboxPostgres(tx),
	}
}

func (u *UnitOfWork) ProductRepo() repository.ProductRepository { return u.productRepo }
func (u *UnitOfWork) OutboxRepo() repository.OutboxRepository   { return u.outboxRepo }
