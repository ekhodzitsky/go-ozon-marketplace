package clickhouse

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/domain"
)

type EventRepo struct {
	conn         driver.Conn
	mu           sync.Mutex
	// seen tracks aggregation keys for in-process deduplication.
	// TODO: add TTL or external store for cross-process idempotency.
	seen         map[string]struct{}
	callTimeout  time.Duration
	queryTimeout time.Duration
}

func NewEventRepo(addr string, callTimeout, queryTimeout time.Duration) (*EventRepo, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse open: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}

	if err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS events (
			event_type String,
			aggregate_id String,
			payload String,
			created_at DateTime,
			aggregation_key String
		) ENGINE = MergeTree()
		ORDER BY created_at
	`); err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}

	return &EventRepo{conn: conn, seen: make(map[string]struct{}), callTimeout: callTimeout, queryTimeout: queryTimeout}, nil
}

func (r *EventRepo) BatchInsert(ctx context.Context, events []domain.Event) error {
	if r.callTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.callTimeout)
		defer cancel()
	}

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
			ev.AggregationKey,
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
	r.mu.Lock()
	if event.AggregationKey != "" {
		if _, exists := r.seen[event.AggregationKey]; exists {
			r.mu.Unlock()
			return nil
		}
	}
	r.mu.Unlock()

	if event.AggregationKey != "" {
		var cnt uint64
		queryCtx := ctx
		if r.queryTimeout > 0 {
			var cancel context.CancelFunc
			queryCtx, cancel = context.WithTimeout(ctx, r.queryTimeout)
			defer cancel()
		}
		if err := r.conn.QueryRow(queryCtx, "SELECT count() FROM events WHERE aggregation_key = ?", event.AggregationKey).Scan(&cnt); err != nil {
			return fmt.Errorf("check aggregation key: %w", err)
		}
		if cnt > 0 {
			r.mu.Lock()
			r.seen[event.AggregationKey] = struct{}{}
			r.mu.Unlock()
			return nil
		}
	}

	if r.callTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.callTimeout)
		defer cancel()
	}
	if err := r.BatchInsert(ctx, []domain.Event{event}); err != nil {
		return err
	}

	if event.AggregationKey != "" {
		r.mu.Lock()
		r.seen[event.AggregationKey] = struct{}{}
		r.mu.Unlock()
	}
	return nil
}

func (r *EventRepo) GetDailyRevenue(ctx context.Context, date string) (float64, error) {
	if r.queryTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.queryTimeout)
		defer cancel()
	}

	var revenue float64
	row := r.conn.QueryRow(ctx, `
		SELECT SUM(amount)
		FROM events
		WHERE toDate(created_at) = ? AND event_type = 'payment_success'
	`, date)
	if err := row.Scan(&revenue); err != nil {
		return 0, fmt.Errorf("scan revenue: %w", err)
	}
	return revenue, nil
}
