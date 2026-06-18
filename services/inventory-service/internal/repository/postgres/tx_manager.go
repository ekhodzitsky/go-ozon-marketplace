package postgres

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/txmanager"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewInventoryTxManager constructs a thin seam over pkg/txmanager.Manager.
// It leverages the generic manager so the inventory module does not need
// its own wrapper type to adapt a transactional repository.
func NewInventoryTxManager(db *pgxpool.Pool, repo repository.InventoryRepository) repository.TxManager {
	return txmanager.New(db, repo.WithTx)
}

// Compile-time seam check: the generic manager is exactly the local TxManager interface.
var _ repository.TxManager = (*txmanager.Manager[repository.InventoryRepository])(nil)
