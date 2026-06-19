package unitofwork

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository"
)

// UnitOfWork is a transactional aggregate of repositories.
// The transaction boundary is managed by txmanager.Manager; implementations are
// constructed for a specific pgx.Tx and only expose repository accessors.
type UnitOfWork interface {
	ProductRepo() repository.ProductRepository
	OutboxRepo() repository.OutboxRepository
}

// Manager runs a business callback inside a transaction and passes it a UnitOfWork
// bound to that transaction. It is satisfied by txmanager.Manager[UnitOfWork].
type Manager interface {
	Run(ctx context.Context, fn func(UnitOfWork) error) error
}
