package postgres

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/txmanager"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type paymentTxManager struct {
	tm *txmanager.Manager[repository.PaymentRepository]
}

// NewPaymentTxManager creates a transaction manager that runs callbacks with a repository bound to the transaction.
func NewPaymentTxManager(db *pgxpool.Pool, repo repository.PaymentRepository) repository.TxManager {
	tm := txmanager.New(db, func(tx pgx.Tx) repository.PaymentRepository {
		return repo.WithTx(tx)
	})
	return &paymentTxManager{tm: tm}
}

func (tm *paymentTxManager) Run(ctx context.Context, fn func(repo repository.PaymentRepository) error) error {
	return tm.tm.Run(ctx, fn)
}
