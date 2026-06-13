package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_PanicsWithoutPostgresDSN(t *testing.T) {
	t.Setenv("JWT_SECRET", "super-secret-at-least-32-bytes-long")

	assert.Panics(t, func() { New() })
}

func TestNew_PanicsWithoutJWTSecret(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/test?sslmode=disable")

	assert.Panics(t, func() { New() })
}

func TestNew_InvalidPostgresDSNReturnsError(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "invalid-dsn")
	t.Setenv("JWT_SECRET", "super-secret-at-least-32-bytes-long")

	app := New()
	require.NotNil(t, app)
	assert.Error(t, app.Err())
}

func TestNew_InvalidRedisAddrReturnsError(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/test?sslmode=disable")
	t.Setenv("JWT_SECRET", "super-secret-at-least-32-bytes-long")
	t.Setenv("REDIS_ADDR", "::invalid")

	app := New()
	require.NotNil(t, app)
	assert.Error(t, app.Err())
}
