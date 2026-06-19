package featureflags

import (
	"context"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/redis/go-redis/v9"
)

// FeatureFlags is a facade over an OpenFeature client backed by Redis.
type FeatureFlags struct {
	store  *RedisStore
	client *openfeature.Client
}

const clientDomain = "api-gateway"

// New creates a feature-flag facade backed by Redis.
func New(client *redis.Client) (*FeatureFlags, error) {
	store := NewRedisStore(client)
	provider := NewProvider(store)
	if err := openfeature.SetNamedProviderAndWait(clientDomain, provider); err != nil {
		return nil, err
	}
	return &FeatureFlags{
		store:  store,
		client: openfeature.NewClient(clientDomain),
	}, nil
}

// IsEnabled evaluates a boolean flag for the given user.
func (f *FeatureFlags) IsEnabled(ctx context.Context, name string, userID string) bool {
	evalCtx := openfeature.NewEvaluationContext(userID, nil)
	val, _ := f.client.BooleanValue(ctx, name, false, evalCtx)
	return val
}

// Register stores a flag definition.
func (f *FeatureFlags) Register(ctx context.Context, flag *Flag) error {
	return f.store.Set(ctx, flag)
}

// List returns all stored flags.
func (f *FeatureFlags) List(ctx context.Context) ([]*Flag, error) {
	return f.store.List(ctx)
}

// SetEnabled enables or disables a flag.
func (f *FeatureFlags) SetEnabled(ctx context.Context, name string, enabled bool) error {
	return f.store.SetEnabled(ctx, name, enabled)
}

// SetPercentage sets a percentage rollout strategy for a flag.
func (f *FeatureFlags) SetPercentage(ctx context.Context, name string, percentage int) error {
	return f.store.SetPercentage(ctx, name, percentage)
}
