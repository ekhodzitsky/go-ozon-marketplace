//go:build integration

package app_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/app"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func startPostgres(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("marketplace"),
		postgres.WithUsername("ozon"),
		postgres.WithPassword("ozonpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = container.Terminate(ctx)
	})

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	return dsn
}

func runMigrations(t *testing.T, dsn string) {
	t.Helper()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	migrationDir := filepath.Join("..", "..", "migrations")
	entries, err := os.ReadDir(migrationDir)
	require.NoError(t, err)

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	for _, file := range files {
		content, err := os.ReadFile(filepath.Join(migrationDir, file))
		require.NoError(t, err)

		tx, err := pool.Begin(ctx)
		require.NoError(t, err)

		_, err = tx.Exec(ctx, string(content))
		if err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("failed to execute migration %s: %v", file, err)
		}
		require.NoError(t, tx.Commit(ctx))
	}
}

func getFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

func waitForGRPCHealth(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			client := healthpb.NewHealthClient(conn)
			_, err = client.Check(ctx, &healthpb.HealthCheckRequest{})
			_ = conn.Close()
			if err == nil {
				cancel()
				return
			}
		}
		cancel()
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("gRPC health check failed at %s", addr)
}

func TestApp_StartStop(t *testing.T) {
	dsn := startPostgres(t)
	runMigrations(t, dsn)

	grpcPort := getFreePort(t)
	metricsPort := getFreePort(t)

	t.Setenv("JWT_SECRET", "this-is-a-very-long-secret-key-for-tests")
	t.Setenv("POSTGRES_DSN", dsn)
	t.Setenv("GRPC_PORT", strconv.Itoa(grpcPort))
	t.Setenv("METRICS_PORT", strconv.Itoa(metricsPort))
	t.Setenv("LOG_LEVEL", "error")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	application := app.New()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, application.Start(ctx))

	waitForGRPCHealth(t, "127.0.0.1:"+strconv.Itoa(grpcPort))

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()
	require.NoError(t, application.Stop(stopCtx))
}
