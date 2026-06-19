package app

import (
	"os"
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_MissingJWTSecret(t *testing.T) {
	require.NoError(t, os.Unsetenv("JWT_SECRET"))

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestLoadConfig_InvalidConfig(t *testing.T) {
	t.Setenv("JWT_SECRET", "short")

	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}
