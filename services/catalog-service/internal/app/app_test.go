package app_test

import (
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/config"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_MissingPostgresDSN(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "")
	t.Setenv("JWT_SECRET", "")

	_, err := config.Load()
	require.Error(t, err)
}

func TestLoadConfig_InvalidDSN(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "bad-dsn")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")

	_, err := config.Load()
	require.Error(t, err)
}

func TestLoadConfig_ShortJWTSecret(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("JWT_SECRET", "short")

	_, err := config.Load()
	require.Error(t, err)
}
