package saga

import (
	"context"
	"fmt"
	"hash/fnv"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresAdvisoryLock implements Locker using PostgreSQL advisory locks.
type PostgresAdvisoryLock struct {
	pool *pgxpool.Pool
}

// NewPostgresAdvisoryLock creates a Locker adapter from a pgx connection pool.
func NewPostgresAdvisoryLock(pool *pgxpool.Pool) *PostgresAdvisoryLock {
	return &PostgresAdvisoryLock{pool: pool}
}

// TryLock attempts to acquire a PostgreSQL advisory lock for key.
func (l *PostgresAdvisoryLock) TryLock(ctx context.Context, key string) (bool, error) {
	lockID := advisoryLockID(key)
	var acquired bool
	if err := l.pool.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired); err != nil {
		return false, fmt.Errorf("pg_try_advisory_lock: %w", err)
	}
	return acquired, nil
}

// Unlock releases a PostgreSQL advisory lock for key.
func (l *PostgresAdvisoryLock) Unlock(ctx context.Context, key string) error {
	lockID := advisoryLockID(key)
	_, err := l.pool.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockID)
	if err != nil {
		return fmt.Errorf("pg_advisory_unlock: %w", err)
	}
	return nil
}

func advisoryLockID(key string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return int64(h.Sum64())
}
