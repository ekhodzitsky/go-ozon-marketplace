package ratelimit

import (
	"context"
	"sync"
	"time"
)

// RateLimiter режет частоту запросов по ключу.
type RateLimiter interface {
	Allow(ctx context.Context, key string) bool
}

// NoopLimiter пропускает всё. Используем, когда ограничение не настроено.
type NoopLimiter struct{}

func (n *NoopLimiter) Allow(ctx context.Context, key string) bool { return true }

// MemoryRateLimiter — простой in-memory rate limiter с скользящим окном.
type MemoryRateLimiter struct {
	limit  int
	window time.Duration
	mu     sync.Mutex
	hits   map[string][]time.Time
}

// NewMemoryRateLimiter создаёт лимитер: limit вызовов за window.
func NewMemoryRateLimiter(limit int, window time.Duration) *MemoryRateLimiter {
	if limit <= 0 {
		limit = 10
	}
	if window <= 0 {
		window = time.Minute
	}
	return &MemoryRateLimiter{
		limit:  limit,
		window: window,
		hits:   make(map[string][]time.Time),
	}
}

// Allow возвращает true, если вызов по ключу укладывается в лимит.
func (r *MemoryRateLimiter) Allow(ctx context.Context, key string) bool {
	now := time.Now()
	cutoff := now.Add(-r.window)

	r.mu.Lock()
	defer r.mu.Unlock()

	var recent []time.Time
	for _, t := range r.hits[key] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= r.limit {
		r.hits[key] = recent
		return false
	}
	r.hits[key] = append(recent, now)
	return true
}
