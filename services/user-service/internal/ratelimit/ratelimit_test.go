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

	for i := 0; i < 10; i++ {
		assert.True(t, rl.Allow(context.Background(), "key"))
	}
	assert.False(t, rl.Allow(context.Background(), "key"))
}

func TestNoopLimiter_AlwaysAllows(t *testing.T) {
	t.Parallel()
	var rl NoopLimiter
	assert.True(t, rl.Allow(context.Background(), "any"))
}
