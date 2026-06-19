package app_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func getFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

func startPostgres(t *testing.T) string {
	t.Helper()
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

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	return connStr
}

func setRequiredEnv(t *testing.T, dsn string) (grpcPort, metricsPort int) {
	t.Helper()
	grpcPort = getFreePort(t)
	metricsPort = getFreePort(t)
	require.NoError(t, os.Setenv("GRPC_PORT", itoa(grpcPort)))
	require.NoError(t, os.Setenv("METRICS_PORT", itoa(metricsPort)))
	require.NoError(t, os.Setenv("POSTGRES_DSN", dsn))
	require.NoError(t, os.Setenv("JWT_SECRET", "test-secret-for-payment-service"))
	t.Cleanup(func() {
		_ = os.Unsetenv("GRPC_PORT")
		_ = os.Unsetenv("METRICS_PORT")
		_ = os.Unsetenv("POSTGRES_DSN")
		_ = os.Unsetenv("JWT_SECRET")
	})
	return grpcPort, metricsPort
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func waitForTCP(t *testing.T, addr string) {
	t.Helper()
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 50 * time.Millisecond
	b.MaxInterval = 200 * time.Millisecond
	b.MaxElapsedTime = 5 * time.Second

	err := backoff.Retry(func() error {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err != nil {
			return err
		}
		_ = conn.Close()
		return nil
	}, b)
	require.NoError(t, err)
}

func TestLoadConfig_InvalidConfig(t *testing.T) {
	require.NoError(t, os.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/db?sslmode=disable"))
	require.NoError(t, os.Setenv("JWT_SECRET", "short"))
	t.Cleanup(func() {
		_ = os.Unsetenv("POSTGRES_DSN")
		_ = os.Unsetenv("JWT_SECRET")
	})

	_, err := config.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "jwt secret must be at least")
}

func TestNew_DIWiring(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration-style DI wiring test in short mode")
	}

	dsn := startPostgres(t)
	grpcPort, metricsPort := setRequiredEnv(t, dsn)

	cfg, err := config.Load()
	require.NoError(t, err)

	application := app.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	require.NoError(t, application.Start(ctx))
	waitForTCP(t, fmt.Sprintf("127.0.0.1:%d", grpcPort))
	waitForTCP(t, fmt.Sprintf("127.0.0.1:%d", metricsPort))

	t.Cleanup(func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		_ = application.Stop(stopCtx)
	})
}
