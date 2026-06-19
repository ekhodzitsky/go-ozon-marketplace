package featureflags

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
)

const redisKey = "featureflags"

// Flag — один фиче-флаг.
type Flag struct {
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
	Strategy   string `json:"strategy"`
	Percentage int    `json:"percentage"`
}

// RedisStore хранит фиче-флаги в Redis-хеше.
// Если Redis-клиент не передан, работает на in-memory мапе.
type RedisStore struct {
	client *redis.Client
	mu     sync.RWMutex
	local  map[string]*Flag
}

// NewRedisStore создаёт стор поверх Redis.
func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client, local: make(map[string]*Flag)}
}

// Get загружает флаг по имени. nil означает, что флага нет.
func (s *RedisStore) Get(ctx context.Context, name string) (*Flag, error) {
	if s.client == nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		if f, ok := s.local[name]; ok {
			cp := *f
			return &cp, nil
		}
		return nil, nil
	}
	raw, err := s.client.HGet(ctx, redisKey, name).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("hget featureflags/%s: %w", name, err)
	}
	var f Flag
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		return nil, fmt.Errorf("unmarshal flag %s: %w", name, err)
	}
	f.Name = name
	return &f, nil
}

// Set сохраняет или обновляет флаг.
func (s *RedisStore) Set(ctx context.Context, flag *Flag) error {
	if s.client == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		cp := *flag
		s.local[flag.Name] = &cp
		return nil
	}
	b, err := json.Marshal(flag)
	if err != nil {
		return fmt.Errorf("marshal flag %s: %w", flag.Name, err)
	}
	if err := s.client.HSet(ctx, redisKey, flag.Name, string(b)).Err(); err != nil {
		return fmt.Errorf("hset featureflags/%s: %w", flag.Name, err)
	}
	return nil
}

// List returns all stored flags.
func (s *RedisStore) List(ctx context.Context) ([]*Flag, error) {
	if s.client == nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		flags := make([]*Flag, 0, len(s.local))
		for _, f := range s.local {
			cp := *f
			flags = append(flags, &cp)
		}
		return flags, nil
	}
	data, err := s.client.HGetAll(ctx, redisKey).Result()
	if err != nil {
		return nil, fmt.Errorf("hgetall featureflags: %w", err)
	}
	flags := make([]*Flag, 0, len(data))
	for name, raw := range data {
		var f Flag
		if err := json.Unmarshal([]byte(raw), &f); err != nil {
			return nil, fmt.Errorf("unmarshal flag %s: %w", name, err)
		}
		f.Name = name
		flags = append(flags, &f)
	}
	return flags, nil
}

// SetEnabled меняет состояние включения флага.
func (s *RedisStore) SetEnabled(ctx context.Context, name string, enabled bool) error {
	flag, err := s.Get(ctx, name)
	if err != nil {
		return err
	}
	if flag == nil {
		flag = &Flag{Name: name, Strategy: "default"}
	}
	flag.Enabled = enabled
	return s.Set(ctx, flag)
}

// SetPercentage меняет процент раската флага.
func (s *RedisStore) SetPercentage(ctx context.Context, name string, percentage int) error {
	if percentage < 0 || percentage > 100 {
		return fmt.Errorf("percentage must be 0-100")
	}
	flag, err := s.Get(ctx, name)
	if err != nil {
		return err
	}
	if flag == nil {
		flag = &Flag{Name: name}
	}
	flag.Strategy = "percentage"
	flag.Percentage = percentage
	flag.Enabled = true
	return s.Set(ctx, flag)
}
