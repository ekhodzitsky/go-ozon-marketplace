package featureflags

import (
	"context"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/redis/go-redis/v9"
)

// FeatureFlags — фасад над OpenFeature-клиентом на базе Redis.
type FeatureFlags struct {
	store  *RedisStore
	client *openfeature.Client
}

const clientDomain = "api-gateway"

// New создаёт фасад фиче-флагов поверх Redis.
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

// IsEnabled проверяет булевый флаг для заданного пользователя.
func (f *FeatureFlags) IsEnabled(ctx context.Context, name string, userID string) bool {
	evalCtx := openfeature.NewEvaluationContext(userID, nil)
	val, _ := f.client.BooleanValue(ctx, name, false, evalCtx)
	return val
}

// Register сохраняет определение флага.
func (f *FeatureFlags) Register(ctx context.Context, flag *Flag) error {
	return f.store.Set(ctx, flag)
}

// List возвращает все сохранённые флаги.
func (f *FeatureFlags) List(ctx context.Context) ([]*Flag, error) {
	return f.store.List(ctx)
}

// SetEnabled включает или выключает флаг.
func (f *FeatureFlags) SetEnabled(ctx context.Context, name string, enabled bool) error {
	return f.store.SetEnabled(ctx, name, enabled)
}

// SetPercentage задаёт процентный раскат для флага.
func (f *FeatureFlags) SetPercentage(ctx context.Context, name string, percentage int) error {
	return f.store.SetPercentage(ctx, name, percentage)
}
