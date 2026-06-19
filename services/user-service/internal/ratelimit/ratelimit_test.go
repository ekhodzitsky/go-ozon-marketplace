package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMemoryRateLimiter_Defaults(t *testing.T) {
	t.Parallel()

	rl := NewMemoryRateLimiter(0, 0)
	assert.Equal(t, 10, rl.limit)
	assert.Equal(t, time.Minute, rl.window)

	// Должен пропустить 10 запросов с дефолтным лимитом.
	for i := 0; i < 10; i++ {
		assert.True(t, rl.Allow(context.Background(), "key"))
	}
	assert.False(t, rl.Allow(context.Background(), "key"))
}

func TestMemoryRateLimiter_DifferentKeysDoNotInterfere(t *testing.T) {
	t.Parallel()

	rl := NewMemoryRateLimiter(1, time.Hour)

	assert.True(t, rl.Allow(context.Background(), "a"))
	assert.True(t, rl.Allow(context.Background(), "b"))
	assert.False(t, rl.Allow(context.Background(), "a"))
}

func TestNoopLimiter_AlwaysAllows(t *testing.T) {
	t.Parallel()

	rl := NewNoopLimiter()
	for i := 0; i < 100; i++ {
		assert.True(t, rl.Allow(context.Background(), "key"))
	}
}
