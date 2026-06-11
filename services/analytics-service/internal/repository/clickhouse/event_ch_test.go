package clickhouse

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// mockRow is a test double for driver.Row.
type mockRow struct {
	scanFunc func(dest ...any) error
}

func (m *mockRow) Err() error              { return nil }
func (m *mockRow) Scan(dest ...any) error  { return m.scanFunc(dest...) }
func (m *mockRow) ScanStruct(dest any) error { return nil }

// mockConn is a test double for driver.Conn that captures QueryRow arguments.
type mockConn struct {
	queryRowQuery string
	queryRowArgs  []any
	row           driver.Row
}

func (m *mockConn) Contributors() []string                       { return nil }
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
	return nil, nil
}
func (m *mockConn) Exec(ctx context.Context, query string, args ...any) error { return nil }
func (m *mockConn) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	return nil
}
func (m *mockConn) Ping(context.Context) error     { return nil }
func (m *mockConn) Stats() driver.Stats            { return driver.Stats{} }
func (m *mockConn) Close() error                   { return nil }

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
