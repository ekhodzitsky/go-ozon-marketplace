package app

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/featureflags"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/ws"
	"github.com/olahol/melody"
	"github.com/redis/go-redis/v9"
)

func provideFeatureFlags(redisClient *redis.Client) (*featureflags.FeatureFlags, error) {
	ctx := context.Background()
	flags, err := featureflags.New(redisClient)
	if err != nil {
		return nil, err
	}
	defaults := []*featureflags.Flag{
		{Name: "new-checkout-flow", Enabled: false, Strategy: "default"},
		{Name: "fast-search", Enabled: false, Strategy: "default"},
		{Name: "discount-system", Enabled: false, Strategy: "default"},
		{Name: "real-time-updates", Enabled: false, Strategy: "default"},
	}
	for _, f := range defaults {
		if _, err := flags.List(ctx); err != nil {
			return nil, err
		}
		if err := flags.Register(ctx, f); err != nil {
			return nil, err
		}
	}
	return flags, nil
}

func provideRateLimiter(redisClient *redis.Client, cfg *config.Config) middleware.RateLimiter {
	return middleware.NewRoleRateLimiter(redisClient, cfg.RateLimitUserRPS, cfg.RateLimitAdminRPS, cfg.RateLimitWindow)
}

func provideHub(redisClient *redis.Client) *melody.Melody {
	m := ws.NewHub()
	go func() {
		ws.StartRedisPubSub(context.Background(), redisClient, m)
	}()
	return m
}
