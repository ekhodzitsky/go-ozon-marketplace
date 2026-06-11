package postgres

import (
	"context"
	"fmt"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PaymentTxManager struct {
	db   *pgxpool.Pool
	repo repository.PaymentRepository
}

func NewPaymentTxManager(db *pgxpool.Pool, repo repository.PaymentRepository) repository.TxManager {
	return &PaymentTxManager{db: db, repo: repo}
}

func (tm *PaymentTxManager) Run(ctx context.Context, fn func(repo repository.PaymentRepository) error) error {
	tx, err := tm.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	repoTx := tm.repo.WithTx(tx)
	if err := fn(repoTx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
