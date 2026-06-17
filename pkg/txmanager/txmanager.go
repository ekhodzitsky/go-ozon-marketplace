package txmanager

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Manager runs a function inside a database transaction.
type Manager[T any] struct {
	pool    *pgxpool.Pool
	factory func(pgx.Tx) T
}

// New creates a transaction manager that produces a transactional resource T for each transaction.
func New[T any](pool *pgxpool.Pool, factory func(pgx.Tx) T) *Manager[T] {
	return &Manager[T]{pool: pool, factory: factory}
}

// Run executes fn inside a transaction. The resource T is created from the transaction handle.
// If fn returns an error, the transaction is rolled back. Otherwise it is committed.
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

// RunTx is a lower-level variant that exposes the raw pgx transaction handle.
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
