package app

import (
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_ErrorsWithoutPostgresDSN(t *testing.T) {
	t.Setenv("JWT_SECRET", "super-secret-at-least-32-bytes-long")

	_, err := config.Load()
	assert.Error(t, err)
}

func TestLoadConfig_ErrorsWithoutJWTSecret(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/test?sslmode=disable")

	_, err := config.Load()
	assert.Error(t, err)
}

func TestNew_InvalidPostgresDSNReturnsError(t *testing.T) {
	// Valid format, but points to a non-listening host so connection fails inside fx.New.
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/test?sslmode=disable")
	t.Setenv("JWT_SECRET", "super-secret-at-least-32-bytes-long")

	cfg, err := config.Load()
	require.NoError(t, err)
	app := New(cfg)
	require.NotNil(t, app)
	assert.Error(t, app.Err())
}

func TestNew_InvalidRedisAddrReturnsError(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/test?sslmode=disable")
	t.Setenv("JWT_SECRET", "super-secret-at-least-32-bytes-long")
	t.Setenv("REDIS_ADDR", "::invalid")

	cfg, err := config.Load()
	require.NoError(t, err)
	app := New(cfg)
	require.NotNil(t, app)
	assert.Error(t, app.Err())
}
