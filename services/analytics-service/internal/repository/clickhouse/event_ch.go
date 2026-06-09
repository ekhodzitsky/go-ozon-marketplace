package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/domain"
)

type EventRepo struct {
	conn driver.Conn
}

func NewEventRepo(addr string) (*EventRepo, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse open: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}

	if err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS events (
			event_type String,
			aggregate_id String,
			payload String,
			created_at DateTime
		) ENGINE = MergeTree()
		ORDER BY created_at
	`); err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}

	return &EventRepo{conn: conn}, nil
}

func (r *EventRepo) BatchInsert(ctx context.Context, events []domain.Event) error {
	batch, err := r.conn.PrepareBatch(ctx, "INSERT INTO events")
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}

	for _, ev := range events {
		if err := batch.Append(
			ev.EventType,
			ev.AggregateID,
			ev.Payload,
			ev.CreatedAt,
		); err != nil {
			return fmt.Errorf("append event: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("send batch: %w", err)
	}

	return nil
}

func (r *EventRepo) Insert(ctx context.Context, event domain.Event) error {
	return r.BatchInsert(ctx, []domain.Event{event})
}

func (r *EventRepo) GetDailyRevenue(ctx context.Context, date string) (float64, error) {
	var revenue float64
	row := r.conn.QueryRow(ctx, `
		SELECT COUNT() * 100.0
		FROM events
		WHERE toDate(created_at) = ?
	`, date)
	if err := row.Scan(&revenue); err != nil {
		return 0, fmt.Errorf("scan revenue: %w", err)
	}
	return revenue, nil
}
