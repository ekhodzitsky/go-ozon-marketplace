package clickhouse

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/domain"
	"github.com/google/uuid"
)

// mockRow is a test double for driver.Row.
type mockRow struct {
	scanFunc func(dest ...any) error
}

func (m *mockRow) Err() error                { return nil }
func (m *mockRow) Scan(dest ...any) error    { return m.scanFunc(dest...) }
func (m *mockRow) ScanStruct(dest any) error { return nil }

// mockBatch is a test double for driver.Batch.
type mockBatch struct {
	appendFunc func(v ...any) error
	sendFunc   func() error
}

func (m *mockBatch) Append(v ...any) error           { return m.appendFunc(v...) }
func (m *mockBatch) AppendStruct(v any) error        { return nil }
func (m *mockBatch) Column(i int) driver.BatchColumn { return nil }
func (m *mockBatch) Send() error                     { return m.sendFunc() }
func (m *mockBatch) Abort() error                    { return nil }
func (m *mockBatch) Flush() error                    { return nil }
func (m *mockBatch) IsSent() bool                    { return false }
func (m *mockBatch) Rows() int                       { return 0 }
func (m *mockBatch) Columns() []column.Interface     { return nil }

// mockConn is a test double for driver.Conn that captures operations.
type mockConn struct {
	queryRowQuery     string
	queryRowArgs      []any
	row               driver.Row
	batch             driver.Batch
	prepareBatchQuery string
}

func (m *mockConn) Contributors() []string                        { return nil }
func (m *mockConn) ServerVersion() (*driver.ServerVersion, error) { return nil, nil }
func (m *mockConn) Select(ctx context.Context, dest any, query string, args ...any) error {
	return nil
}
func (m *mockConn) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	return nil, nil
}
func (m *mockConn) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	m.queryRowQuery = query
	m.queryRowArgs = args
	return m.row
}
func (m *mockConn) PrepareBatch(ctx context.Context, query string, opts ...driver.PrepareBatchOption) (driver.Batch, error) {
	m.prepareBatchQuery = query
	return m.batch, nil
}
func (m *mockConn) Exec(ctx context.Context, query string, args ...any) error { return nil }
func (m *mockConn) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	return nil
}
func (m *mockConn) Ping(context.Context) error { return nil }
func (m *mockConn) Stats() driver.Stats        { return driver.Stats{} }
func (m *mockConn) Close() error               { return nil }

func TestEventRepo_GetDailyRevenue_SQL(t *testing.T) {
	t.Parallel()

	row := &mockRow{
		scanFunc: func(dest ...any) error {
			*dest[0].(*float64) = 1500.50
			return nil
		},
	}
	conn := &mockConn{row: row}
	repo := &EventRepo{conn: conn}

	revenue, err := repo.GetDailyRevenue(context.Background(), "2024-01-15")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if revenue != 1500.50 {
		t.Errorf("expected revenue 1500.50, got %v", revenue)
	}

	query := strings.ToLower(conn.queryRowQuery)
	if !strings.Contains(query, "sum(amount)") {
		t.Errorf("expected query to contain SUM(amount), got: %s", conn.queryRowQuery)
	}
	if !strings.Contains(query, "payment_success") {
		t.Errorf("expected query to contain payment_success, got: %s", conn.queryRowQuery)
	}
	if len(conn.queryRowArgs) != 1 || conn.queryRowArgs[0] != "2024-01-15" {
		t.Errorf("expected args [2024-01-15], got %v", conn.queryRowArgs)
	}
}

func TestEventRepo_GetDailyRevenue_ScanError(t *testing.T) {
	t.Parallel()

	row := &mockRow{
		scanFunc: func(dest ...any) error {
			return errors.New("scan failed")
		},
	}
	conn := &mockConn{row: row}
	repo := &EventRepo{conn: conn}

	_, err := repo.GetDailyRevenue(context.Background(), "2024-01-15")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "scan revenue") {
		t.Errorf("expected error to contain 'scan revenue', got: %v", err)
	}
}

func TestEventRepo_BatchInsert(t *testing.T) {
	t.Parallel()

	batch := &mockBatch{
		appendFunc: func(v ...any) error { return nil },
		sendFunc:   func() error { return nil },
	}
	conn := &mockConn{batch: batch}
	repo := &EventRepo{conn: conn, callTimeout: time.Second}

	events := []domain.Event{
		{EventType: domain.EventTypePaymentSuccess, AggregateID: "order-1", Amount: 10.0, CreatedAt: time.Now().UTC()},
	}

	if err := repo.BatchInsert(context.Background(), events); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(conn.prepareBatchQuery, "INSERT INTO events") {
		t.Errorf("expected INSERT INTO events query, got: %s", conn.prepareBatchQuery)
	}
}

func TestEventRepo_BatchInsert_SendError(t *testing.T) {
	t.Parallel()

	batch := &mockBatch{
		appendFunc: func(v ...any) error { return nil },
		sendFunc:   func() error { return errors.New("send failed") },
	}
	conn := &mockConn{batch: batch}
	repo := &EventRepo{conn: conn, callTimeout: time.Second}

	events := []domain.Event{
		{EventType: domain.EventTypePaymentSuccess, AggregateID: "order-1", Amount: 10.0, CreatedAt: time.Now().UTC()},
	}

	err := repo.BatchInsert(context.Background(), events)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "send batch") {
		t.Errorf("expected error to contain 'send batch', got: %v", err)
	}
}

func TestEventRepo_TrackABTestEvent(t *testing.T) {
	t.Parallel()

	batch := &mockBatch{
		appendFunc: func(v ...any) error { return nil },
		sendFunc:   func() error { return nil },
	}
	conn := &mockConn{batch: batch}
	repo := &EventRepo{conn: conn, callTimeout: time.Second}

	event := domain.ABTestEvent{
		EventID:      uuid.Nil,
		Experiment:   "exp-1",
		Variation:    "var-a",
		UserID:       uuid.Must(uuid.NewV7()),
		Conversion:   true,
		RevenueMinor: 100,
	}

	if err := repo.TrackABTestEvent(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(conn.prepareBatchQuery, "ab_test_events") {
		t.Errorf("expected ab_test_events query, got: %s", conn.prepareBatchQuery)
	}
}

func TestEventRepo_TrackABTestEvent_AppendError(t *testing.T) {
	t.Parallel()

	batch := &mockBatch{
		appendFunc: func(v ...any) error { return errors.New("append failed") },
		sendFunc:   func() error { return nil },
	}
	conn := &mockConn{batch: batch}
	repo := &EventRepo{conn: conn, callTimeout: time.Second}

	err := repo.TrackABTestEvent(context.Background(), domain.ABTestEvent{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "append event") {
		t.Errorf("expected error to contain 'append event', got: %v", err)
	}
}

func TestEventRepo_Close(t *testing.T) {
	t.Parallel()

	conn := &mockConn{}
	repo := &EventRepo{conn: conn}
	if err := repo.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEventRepo_NewEventRepo_PingFailure(t *testing.T) {
	t.Parallel()

	_, err := NewEventRepo("127.0.0.1:1", "", "", time.Second, time.Second, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "clickhouse ping") {
		t.Errorf("expected error to contain 'clickhouse ping', got: %v", err)
	}
}
