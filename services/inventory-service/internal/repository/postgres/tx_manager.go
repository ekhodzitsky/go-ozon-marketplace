package postgres

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/txmanager"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type inventoryTxManager struct {
	tm *txmanager.Manager[repository.InventoryRepository]
}

// NewInventoryTxManager creates a transaction manager that runs callbacks with a repository bound to the transaction.
func NewInventoryTxManager(db *pgxpool.Pool, repo repository.InventoryRepository) repository.TxManager {
	tm := txmanager.New(db, func(tx pgx.Tx) repository.InventoryRepository {
		return repo.WithTx(tx)
	})
	return &inventoryTxManager{tm: tm}
}

func (tm *inventoryTxManager) Run(ctx context.Context, fn func(repo repository.InventoryRepository) error) error {
	return tm.tm.Run(ctx, fn)
}
