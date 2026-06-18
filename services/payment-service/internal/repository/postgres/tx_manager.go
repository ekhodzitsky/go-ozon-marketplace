package postgres

import (
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/txmanager"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPaymentTxManager constructs a thin seam over pkg/txmanager.Manager.
// It leverages the generic manager so the payment module does not need
// its own wrapper type to adapt a transactional repository.
func NewPaymentTxManager(db *pgxpool.Pool, repo repository.PaymentRepository) repository.TxManager {
	return txmanager.New(db, repo.WithTx)
}

// Compile-time seam check: the generic manager is exactly the local TxManager interface.
var _ repository.TxManager = (*txmanager.Manager[repository.PaymentRepository])(nil)
