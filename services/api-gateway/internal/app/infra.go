package app

import (
	"context"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/circuitbreaker"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/featureflags"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	pkgredis "github.com/ekhodzitsky/go-ozon-marketplace/pkg/redis"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/clients"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/ws"
	"go.uber.org/zap"
	"github.com/redis/go-redis/v9"
)

func provideContext() context.Context {
	return context.Background()
}

func provideLogger(cfg *config.Config) (*zap.Logger, error) {
	return logger.New(cfg.LogLevel, cfg.LogFormat)
}

func provideCircuitBreaker() *circuitbreaker.CircuitBreaker {
	return circuitbreaker.New(5, 2, 30*time.Second)
}

func provideClientFactory(cfg *config.Config, cb *circuitbreaker.CircuitBreaker) *clients.Factory {
	return clients.NewFactory(cfg, cb)
}

func provideRedis(ctx context.Context, cfg *config.Config) (*redis.Client, func(), error) {
	client, err := pkgredis.NewClient(ctx, cfg.RedisAddr)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		_ = client.Close()
	}
	return client, cleanup, nil
}

func provideFeatureFlags(ctx context.Context, redisClient *redis.Client) (*featureflags.Engine, error) {
	engine := featureflags.NewEngine(redisClient)
	_ = engine.LoadFromRedis()
	engine.Register(&featureflags.Flag{Name: "new-checkout-flow", Enabled: false, Strategy: "default"})
	engine.Register(&featureflags.Flag{Name: "fast-search", Enabled: false, Strategy: "default"})
	engine.Register(&featureflags.Flag{Name: "discount-system", Enabled: false, Strategy: "default"})
	engine.Register(&featureflags.Flag{Name: "real-time-updates", Enabled: false, Strategy: "default"})
	_ = engine.SaveToRedis()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = engine.LoadFromRedis()
			case <-engine.Done():
				return
			}
		}
	}()
	return engine, nil
}

func provideRateLimiter(redisClient *redis.Client, cfg *config.Config) middleware.RateLimiter {
	return middleware.NewRoleRateLimiter(redisClient, cfg.RateLimitUserRPS, cfg.RateLimitAdminRPS, cfg.RateLimitWindow)
}

func provideHub(redisClient *redis.Client) *ws.Hub {
	hub := ws.NewHub()
	go hub.Run()
	go func() {
		ws.StartRedisPubSub(context.Background(), redisClient, hub)
	}()
	return hub
}
