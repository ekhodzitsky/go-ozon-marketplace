package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type OutboxPostgres struct {
	db   Querier
	pool Querier
	tx   pgx.Tx
}

func NewOutboxPostgres(db Querier) repository.OutboxRepository {
	return &OutboxPostgres{db: db, pool: db}
}

func (r *OutboxPostgres) WithTx(tx pgx.Tx) *OutboxPostgres {
	return &OutboxPostgres{db: tx, pool: tx, tx: tx}
}

func (r *OutboxPostgres) Begin(ctx context.Context) error {
	if r.tx != nil {
		return nil
	}
	beginner, ok := r.pool.(interface {
		Begin(ctx context.Context) (pgx.Tx, error)
	})
	if !ok {
		return fmt.Errorf("outbox postgres: cannot begin transaction: db does not support Begin")
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin outbox tx: %w", err)
	}
	r.tx = tx
	r.db = tx
	return nil
}

func (r *OutboxPostgres) Commit(ctx context.Context) error {
	if r.tx == nil {
		return fmt.Errorf("no active outbox transaction")
	}
	err := r.tx.Commit(ctx)
	r.tx = nil
	r.db = r.pool
	return err
}

func (r *OutboxPostgres) Rollback(ctx context.Context) error {
	if r.tx == nil {
		return nil
	}
	err := r.tx.Rollback(ctx)
	r.tx = nil
	r.db = r.pool
	return err
}

func (r *OutboxPostgres) Create(ctx context.Context, event *domain.OutboxEvent) error {
	if event.NextRetryAt.IsZero() {
		event.NextRetryAt = time.Now().UTC()
	}
	query := `INSERT INTO outbox (id, aggregate_type, aggregate_id, event_type, payload, created_at, retry_count, next_retry_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	_, err := r.db.Exec(ctx, query, event.ID, event.AggregateType, event.AggregateID, event.EventType, event.Payload, event.CreatedAt, event.RetryCount, event.NextRetryAt)
	if err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}

func (r *OutboxPostgres) GetUnprocessed(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	query := `SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at, processed_at, retry_count, last_error, next_retry_at
		FROM outbox
		WHERE processed_at IS NULL AND next_retry_at <= NOW()
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED`
	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get unprocessed outbox: %w", err)
	}
	defer rows.Close()

	var events []domain.OutboxEvent
	for rows.Next() {
		var event domain.OutboxEvent
		var processedAt *time.Time
		var lastError *string
		if err := rows.Scan(&event.ID, &event.AggregateType, &event.AggregateID, &event.EventType, &event.Payload, &event.CreatedAt, &processedAt, &event.RetryCount, &lastError, &event.NextRetryAt); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		event.ProcessedAt = processedAt
		event.LastError = lastError
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}
	return events, nil
}

func (r *OutboxPostgres) MarkProcessed(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE outbox SET processed_at=NOW() WHERE id=$1`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("mark outbox processed: %w", err)
	}
	return nil
}

func (r *OutboxPostgres) BatchMarkProcessed(ctx context.Context, ids []uuid.UUID) error {
	query := `UPDATE outbox SET processed_at = NOW() WHERE id = ANY($1)`
	_, err := r.db.Exec(ctx, query, ids)
	if err != nil {
		return fmt.Errorf("batch mark outbox processed: %w", err)
	}
	return nil
}

func (r *OutboxPostgres) IncrementRetryAndSetError(ctx context.Context, id uuid.UUID, lastError string, nextRetryAt time.Time) error {
	query := `UPDATE outbox SET retry_count = retry_count + 1, last_error = $2, next_retry_at = $3 WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id, lastError, nextRetryAt)
	if err != nil {
		return fmt.Errorf("increment outbox retry: %w", err)
	}
	return nil
}

func (r *OutboxPostgres) MoveToDLQ(ctx context.Context, event *domain.OutboxEvent, failedAt time.Time, lastError string) error {
	insertQuery := `INSERT INTO outbox_dlq (id, aggregate_type, aggregate_id, event_type, payload, created_at, failed_at, last_error, retry_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO NOTHING`
	_, err := r.db.Exec(ctx, insertQuery, event.ID, event.AggregateType, event.AggregateID, event.EventType, event.Payload, event.CreatedAt, failedAt, lastError, event.RetryCount+1)
	if err != nil {
		return fmt.Errorf("insert outbox dlq: %w", err)
	}

	deleteQuery := `DELETE FROM outbox WHERE id = $1`
	_, err = r.db.Exec(ctx, deleteQuery, event.ID)
	if err != nil {
		return fmt.Errorf("delete outbox event: %w", err)
	}
	return nil
}
