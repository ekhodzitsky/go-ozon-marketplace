package app_test

import (
	"testing"

	catalogapp "github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/app"
	"github.com/stretchr/testify/require"
)

func TestNew_MissingConfig(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "")
	t.Setenv("JWT_SECRET", "")

	require.Panics(t, func() { _ = catalogapp.New() })
}

func TestNew_InvalidDSN(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "bad-dsn")
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")

	require.Panics(t, func() { _ = catalogapp.New() })
}

func TestNew_ShortJWTSecret(t *testing.T) {
	t.Setenv("POSTGRES_DSN", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	t.Setenv("JWT_SECRET", "short")

	require.Panics(t, func() { _ = catalogapp.New() })
}
