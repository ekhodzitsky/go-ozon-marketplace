package app

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_MissingJWTSecret(t *testing.T) {
	require.NoError(t, os.Unsetenv("JWT_SECRET"))

	app := New()
	require.NotNil(t, app)

	err := app.Err()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestNew_InvalidConfig(t *testing.T) {
	t.Setenv("JWT_SECRET", "short")

	app := New()
	require.NotNil(t, app)

	err := app.Err()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}
