package txmanager

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Manager выполняет функцию внутри транзакции БД.
type Manager[T any] struct {
	pool    *pgxpool.Pool
	factory func(pgx.Tx) T
}

// New создаёт менеджер транзакций, который для каждой транзакции строит ресурс T.
func New[T any](pool *pgxpool.Pool, factory func(pgx.Tx) T) *Manager[T] {
	return &Manager[T]{pool: pool, factory: factory}
}

// Run выполняет fn внутри транзакции. Ресурс T создаётся из хендла транзакции.
// Если fn вернула ошибку — транзакция откатывается, иначе коммитится.
func (m *Manager[T]) Run(ctx context.Context, fn func(T) error) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	resource := m.factory(tx)
	if err := fn(resource); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// RunTx — низкоуровневый вариант, отдающий сырой хендл pgx-транзакции.
func RunTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(tx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
