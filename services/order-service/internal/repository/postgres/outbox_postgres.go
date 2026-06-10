package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type OutboxPostgres struct {
	db Querier
}

func NewOutboxPostgres(db Querier) *OutboxPostgres {
	return &OutboxPostgres{db: db}
}

func (r *OutboxPostgres) WithTx(tx pgx.Tx) *OutboxPostgres {
	return &OutboxPostgres{db: tx}
}

func (r *OutboxPostgres) Create(ctx context.Context, event *domain.OutboxEvent) error {
	query := `INSERT INTO outbox (id, aggregate_type, aggregate_id, event_type, payload, created_at) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.Exec(ctx, query, event.ID, event.AggregateType, event.AggregateID, event.EventType, event.Payload, event.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}

func (r *OutboxPostgres) GetUnprocessed(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	query := `SELECT id, aggregate_type, aggregate_id, event_type, payload, created_at, processed_at FROM outbox WHERE processed_at IS NULL ORDER BY created_at ASC LIMIT $1`
	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get unprocessed outbox: %w", err)
	}
	defer rows.Close()

	var events []domain.OutboxEvent
	for rows.Next() {
		var event domain.OutboxEvent
		var processedAt *time.Time
		if err := rows.Scan(&event.ID, &event.AggregateType, &event.AggregateID, &event.EventType, &event.Payload, &event.CreatedAt, &processedAt); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		event.ProcessedAt = processedAt
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
