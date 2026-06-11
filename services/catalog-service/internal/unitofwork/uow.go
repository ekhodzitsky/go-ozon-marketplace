package unitofwork

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository"
)

type UnitOfWork interface {
	Begin(ctx context.Context) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	ProductRepo() repository.ProductRepository
	OutboxRepo() repository.OutboxRepository
}
