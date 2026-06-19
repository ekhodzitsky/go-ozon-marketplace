package featureflags

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/open-feature/go-sdk/openfeature"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) (*RedisStore, *miniredis.Miniredis) {
	t.Helper()
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisStore(client), s
}

func TestRedisStore_GetSet(t *testing.T) {
	t.Parallel()
	store, _ := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Set(ctx, &Flag{Name: "feat", Enabled: true, Strategy: "default"}))
	flag, err := store.Get(ctx, "feat")
	require.NoError(t, err)
	require.NotNil(t, flag)
	assert.True(t, flag.Enabled)
}

func TestRedisStore_GetMissing(t *testing.T) {
	t.Parallel()
	store, _ := newTestStore(t)
	ctx := context.Background()

	flag, err := store.Get(ctx, "missing")
	require.NoError(t, err)
	assert.Nil(t, flag)
}

func TestRedisStore_List(t *testing.T) {
	t.Parallel()
	store, _ := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.Set(ctx, &Flag{Name: "a", Enabled: true, Strategy: "default"}))
	flags, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, flags, 1)
	assert.Equal(t, "a", flags[0].Name)
}

func TestRedisStore_SetEnabled(t *testing.T) {
	t.Parallel()
	store, _ := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.SetEnabled(ctx, "feat", true))
	flag, err := store.Get(ctx, "feat")
	require.NoError(t, err)
	assert.True(t, flag.Enabled)

	require.NoError(t, store.SetEnabled(ctx, "feat", false))
	flag, err = store.Get(ctx, "feat")
	require.NoError(t, err)
	assert.False(t, flag.Enabled)
}

func TestRedisStore_SetPercentage(t *testing.T) {
	t.Parallel()
	store, _ := newTestStore(t)
	ctx := context.Background()

	require.NoError(t, store.SetPercentage(ctx, "rollout", 42))
	flag, err := store.Get(ctx, "rollout")
	require.NoError(t, err)
	assert.Equal(t, "percentage", flag.Strategy)
	assert.Equal(t, 42, flag.Percentage)
	assert.True(t, flag.Enabled)

	require.Error(t, store.SetPercentage(ctx, "rollout", 101))
}

func TestProvider_BooleanEvaluation(t *testing.T) {
	t.Parallel()
	store, _ := newTestStore(t)
	ctx := context.Background()

	provider := NewProvider(store)

	cases := []struct {
		name         string
		flagKey      string
		flag         *Flag
		userID       string
		defaultValue bool
		want         bool
	}{
		{"always_on", "always_on", &Flag{Name: "always_on", Enabled: true, Strategy: "default"}, "user1", false, true},
		{"always_off", "always_off", &Flag{Name: "always_off", Enabled: false, Strategy: "default"}, "user1", true, true},
		{"missing", "missing", nil, "user1", false, false},
		{"percentage_no_user", "pct", &Flag{Name: "pct", Enabled: true, Strategy: "percentage", Percentage: 50}, "", false, false},
		{"by_user_empty", "by_user", &Flag{Name: "by_user", Enabled: true, Strategy: "user_id"}, "", false, false},
		{"by_user_present", "by_user", &Flag{Name: "by_user", Enabled: true, Strategy: "user_id"}, "user1", false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.flag != nil {
				require.NoError(t, store.Set(ctx, tc.flag))
			}
			detail := provider.BooleanEvaluation(ctx, tc.flagKey, tc.defaultValue, map[string]any{"targetingKey": tc.userID})
			assert.Equal(t, tc.want, detail.Value)
		})
	}
}

func TestFeatureFlags_IsEnabled(t *testing.T) {
	t.Parallel()
	store, _ := newTestStore(t)
	ctx := context.Background()

	provider := NewProvider(store)
	require.NoError(t, openfeature.SetProvider(provider))
	ff := &FeatureFlags{store: store, client: openfeature.NewDefaultClient()}

	require.NoError(t, store.Set(ctx, &Flag{Name: "feat", Enabled: true, Strategy: "default"}))
	assert.True(t, ff.IsEnabled(ctx, "feat", "user-1"))
}
