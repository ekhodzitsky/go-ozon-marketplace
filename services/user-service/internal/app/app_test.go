package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_ReturnsErrorForMissingJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	t.Setenv("POSTGRES_DSN", "postgres://u:p@localhost/db?sslmode=disable")

	application := app.New()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = application.Stop(ctx)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := application.Start(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, config.ErrMissingJWTSecret), "expected missing JWT secret error, got %v", err)
	assert.ErrorIs(t, application.Err(), config.ErrMissingJWTSecret)
}

func TestNew_ReturnsErrorForShortJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "short")
	t.Setenv("POSTGRES_DSN", "postgres://u:p@localhost/db?sslmode=disable")

	application := app.New()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = application.Stop(ctx)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := application.Start(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, config.ErrJWTSecretTooShort), "expected short JWT secret error, got %v", err)
}

func TestNew_ReturnsErrorForMissingPostgresDSN(t *testing.T) {
	t.Setenv("JWT_SECRET", "this-is-a-very-long-secret-key-for-tests")
	t.Setenv("POSTGRES_DSN", "")

	application := app.New()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = application.Stop(ctx)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := application.Start(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, config.ErrMissingPostgresDSN), "expected missing postgres DSN error, got %v", err)
}

func TestNew_ReturnsErrorForInvalidPostgresDSN(t *testing.T) {
	t.Setenv("JWT_SECRET", "this-is-a-very-long-secret-key-for-tests")
	t.Setenv("POSTGRES_DSN", "not-a-valid-dsn")

	application := app.New()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = application.Stop(ctx)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := application.Start(ctx)
	require.Error(t, err)
	assert.True(t, errors.Is(err, config.ErrInvalidPostgresDSN), "expected invalid postgres DSN error, got %v", err)
}
