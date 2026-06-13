package clickhouse

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/domain"
	"github.com/google/uuid"
)

type EventRepo struct {
	conn         driver.Conn
	callTimeout  time.Duration
	queryTimeout time.Duration
}

func NewEventRepo(addr, user, password string, callTimeout, queryTimeout time.Duration, migrationFS fs.FS) (*EventRepo, error) {
	opts := &clickhouse.Options{
		Addr: []string{addr},
	}
	if user != "" {
		opts.Auth = clickhouse.Auth{
			Database: "default",
			Username: user,
			Password: password,
		}
	}
	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("clickhouse open: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}

	if migrationFS != nil {
		if err := runMigrations(ctx, conn, migrationFS); err != nil {
			return nil, fmt.Errorf("run migrations: %w", err)
		}
	}

	return &EventRepo{conn: conn, callTimeout: callTimeout, queryTimeout: queryTimeout}, nil
}

func runMigrations(ctx context.Context, conn driver.Conn, fsys fs.FS) error {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return fmt.Errorf("read migration dir: %w", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	for _, f := range files {
		data, err := fs.ReadFile(fsys, f)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}
		if err := conn.Exec(ctx, string(data)); err != nil {
			return fmt.Errorf("execute migration %s: %w", f, err)
		}
	}
	return nil
}

func (r *EventRepo) BatchInsert(ctx context.Context, events []domain.Event) error {
	if r.callTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.callTimeout)
		defer cancel()
	}

	batch, err := r.conn.PrepareBatch(ctx, "INSERT INTO events (event_type, aggregate_id, payload, amount, currency, occurred_at, created_at, aggregation_key)")
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}

	for _, ev := range events {
		if err := batch.Append(
			string(ev.EventType),
			ev.AggregateID,
			ev.Payload,
			ev.Amount,
			ev.Currency,
			ev.OccurredAt,
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

func (r *EventRepo) TrackABTestEvent(ctx context.Context, event domain.ABTestEvent) error {
	if r.callTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.callTimeout)
		defer cancel()
	}

	batch, err := r.conn.PrepareBatch(ctx, "INSERT INTO ab_test_events")
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}

	if event.EventID == uuid.Nil {
		event.EventID = uuid.Must(uuid.NewV7())
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}

	if err := batch.Append(
		event.EventID,
		event.Experiment,
		event.Variation,
		event.UserID,
		event.Conversion,
		event.RevenueMinor,
		event.CreatedAt,
	); err != nil {
		return fmt.Errorf("append event: %w", err)
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("send batch: %w", err)
	}

	return nil
}

func (r *EventRepo) Close() error {
	if r.conn == nil {
		return nil
	}
	return r.conn.Close()
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
