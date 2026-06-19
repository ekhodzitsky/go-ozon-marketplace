package app_test

import (
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_MissingJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	t.Setenv("POSTGRES_DSN", "postgres://u:p@localhost/db?sslmode=disable")

	_, err := config.Load()
	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrMissingJWTSecret)
}

func TestLoadConfig_ShortJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "short")
	t.Setenv("POSTGRES_DSN", "postgres://u:p@localhost/db?sslmode=disable")

	_, err := config.Load()
	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrJWTSecretTooShort)
}

func TestLoadConfig_MissingPostgresDSN(t *testing.T) {
	t.Setenv("JWT_SECRET", "this-is-a-very-long-secret-key-for-tests")
	t.Setenv("POSTGRES_DSN", "")

	_, err := config.Load()
	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrMissingPostgresDSN)
}

func TestLoadConfig_InvalidPostgresDSN(t *testing.T) {
	t.Setenv("JWT_SECRET", "this-is-a-very-long-secret-key-for-tests")
	t.Setenv("POSTGRES_DSN", "not-a-valid-dsn")

	_, err := config.Load()
	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrInvalidPostgresDSN)
}
