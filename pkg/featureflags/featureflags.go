package featureflags

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Engine manages feature flags with Redis-backed storage.
type Engine struct {
	store  map[string]*Flag
	mu     sync.RWMutex
	client *redis.Client
	ctx    context.Context
	cancel context.CancelFunc
}

// Flag represents a single feature flag.
type Flag struct {
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
	Strategy   string `json:"strategy"`
	Percentage int    `json:"percentage"`
}

// NewEngine creates a new feature flags engine backed by Redis.
func NewEngine(client *redis.Client) *Engine {
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		store:  make(map[string]*Flag),
		client: client,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Done returns a channel that is closed when the engine is stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.ctx.Done()
}

// Stop stops the background polling.
func (e *Engine) Stop() {
	e.cancel()
}

// IsEnabled checks if a flag is enabled for a given user.
func (e *Engine) IsEnabled(name string, userID string) bool {
	e.mu.RLock()
	flag, ok := e.store[name]
	e.mu.RUnlock()
	if !ok {
		return false
	}
	if !flag.Enabled {
		return false
	}
	switch flag.Strategy {
	case "percentage":
		if userID == "" {
			return false
		}
		h := hashUserID(userID + name)
		return h%100 < uint32(flag.Percentage)
	case "user_id":
		return userID != ""
	default:
		return true
	}
}

// Register registers a flag in local memory.
func (e *Engine) Register(flag *Flag) {
	e.mu.Lock()
	e.store[flag.Name] = flag
	e.mu.Unlock()
}

// LoadFromRedis loads flags from Redis hash "featureflags".
func (e *Engine) LoadFromRedis() error {
	if e.client == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(e.ctx, 5*time.Second)
	defer cancel()
	data, err := e.client.HGetAll(ctx, "featureflags").Result()
	if err != nil {
		return fmt.Errorf("hgetall featureflags: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	for name, raw := range data {
		var flag Flag
		if err := json.Unmarshal([]byte(raw), &flag); err != nil {
			// Fallback to simple bool parsing for backward compatibility.
			enabled, _ := strconv.ParseBool(raw)
			flag = Flag{Name: name, Enabled: enabled, Strategy: "default"}
		}
		flag.Name = name
		e.store[name] = &flag
	}
	return nil
}

// SaveToRedis saves the current local flag state to Redis.
func (e *Engine) SaveToRedis() error {
	if e.client == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(e.ctx, 5*time.Second)
	defer cancel()

	e.mu.RLock()
	flags := make(map[string]interface{}, len(e.store))
	for name, flag := range e.store {
		b, err := json.Marshal(flag)
		if err != nil {
			e.mu.RUnlock()
			return fmt.Errorf("marshal flag %s: %w", name, err)
		}
		flags[name] = string(b)
	}
	e.mu.RUnlock()

	if err := e.client.HSet(ctx, "featureflags", flags).Err(); err != nil {
		return fmt.Errorf("hset featureflags: %w", err)
	}
	return nil
}

// List returns a copy of all registered flags.
func (e *Engine) List() []*Flag {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*Flag, 0, len(e.store))
	for _, f := range e.store {
		cp := *f
		out = append(out, &cp)
	}
	return out
}

// SetEnabled updates a flag's enabled state in local memory and Redis.
func (e *Engine) SetEnabled(name string, enabled bool) error {
	e.mu.Lock()
	flag, ok := e.store[name]
	if !ok {
		flag = &Flag{Name: name, Strategy: "default"}
		e.store[name] = flag
	}
	flag.Enabled = enabled
	cp := *flag
	e.mu.Unlock()
	return e.saveFlag(name, &cp)
}

// SetPercentage updates a flag's percentage strategy in local memory and Redis.
func (e *Engine) SetPercentage(name string, percentage int) error {
	if percentage < 0 || percentage > 100 {
		return fmt.Errorf("percentage must be 0-100")
	}
	e.mu.Lock()
	flag, ok := e.store[name]
	if !ok {
		flag = &Flag{Name: name, Strategy: "percentage"}
		e.store[name] = flag
	}
	flag.Strategy = "percentage"
	flag.Percentage = percentage
	flag.Enabled = true
	cp := *flag
	e.mu.Unlock()
	return e.saveFlag(name, &cp)
}

func (e *Engine) saveFlag(name string, flag *Flag) error {
	b, err := json.Marshal(flag)
	if err != nil {
		return err
	}
	if e.client == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(e.ctx, 5*time.Second)
	defer cancel()
	return e.client.HSet(ctx, "featureflags", name, string(b)).Err()
}

func hashUserID(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}
