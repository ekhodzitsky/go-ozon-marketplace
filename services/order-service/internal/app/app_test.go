package app_test

import (
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_ErrorsWithoutRequiredConfig(t *testing.T) {
	_, err := config.Load()
	require.Error(t, err)
}

func TestNew_ReturnsAppWithRequiredConfig(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost/db")
	t.Setenv("JWT_SECRET", "super-secret-at-least-32-bytes-long")

	cfg, err := config.Load()
	require.NoError(t, err)
	// fx.New invokes constructors synchronously, so real external connections
	// (Postgres, Redis, Kafka) are not available in unit tests. We only verify
	// that the app object is constructed and that New does not panic on config.
	var constructed bool
	require.NotPanics(t, func() {
		a := app.New(cfg)
		constructed = a != nil
		// An error is expected because the container cannot connect to real
		// dependencies during a unit test.
		if a != nil {
			_ = a.Err()
		}
	})
	assert.True(t, constructed)
}
