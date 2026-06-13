package featureflags

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestEngine(t *testing.T) (*Engine, *miniredis.Miniredis) {
	t.Helper()
	s := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewEngine(client), s
}

func TestEngine_IsEnabled(t *testing.T) {
	t.Parallel()

	e, _ := newTestEngine(t)
	e.Register(&Flag{Name: "always_on", Enabled: true, Strategy: "default"})
	e.Register(&Flag{Name: "always_off", Enabled: false, Strategy: "default"})
	e.Register(&Flag{Name: "pct_50", Enabled: true, Strategy: "percentage", Percentage: 50})
	e.Register(&Flag{Name: "by_user", Enabled: true, Strategy: "user_id"})

	assert.True(t, e.IsEnabled("always_on", "user1"))
	assert.False(t, e.IsEnabled("always_off", "user1"))
	assert.False(t, e.IsEnabled("missing", "user1"))
	assert.False(t, e.IsEnabled("pct_50", ""))
	assert.False(t, e.IsEnabled("by_user", ""))
	assert.True(t, e.IsEnabled("by_user", "user1"))
}

func TestEngine_SetEnabled(t *testing.T) {
	t.Parallel()

	e, s := newTestEngine(t)
	require.NoError(t, e.SetEnabled("feat", true))
	assert.True(t, e.IsEnabled("feat", ""))

	flag := s.HGet("featureflags", "feat")
	require.NotEmpty(t, flag)
	assert.Contains(t, flag, `"enabled":true`)

	require.NoError(t, e.SetEnabled("feat", false))
	assert.False(t, e.IsEnabled("feat", ""))
}

func TestEngine_SetPercentage(t *testing.T) {
	t.Parallel()

	e, s := newTestEngine(t)
	require.NoError(t, e.SetPercentage("rollout", 42))
	assert.True(t, e.IsEnabled("rollout", "user"))

	flag := s.HGet("featureflags", "rollout")
	require.NotEmpty(t, flag)
	assert.Contains(t, flag, `"percentage":42`)
	assert.Contains(t, flag, `"strategy":"percentage"`)

	require.Error(t, e.SetPercentage("rollout", 101))
}

func TestEngine_List(t *testing.T) {
	t.Parallel()

	e, _ := newTestEngine(t)
	e.Register(&Flag{Name: "a", Enabled: true, Strategy: "default"})
	flags := e.List()
	require.Len(t, flags, 1)
	assert.Equal(t, "a", flags[0].Name)

	// List must return copies, not shared pointers.
	flags[0].Name = "mutated"
	assert.Equal(t, "a", e.List()[0].Name)
}

func TestEngine_LoadFromRedis(t *testing.T) {
	t.Parallel()

	e, s := newTestEngine(t)
	s.HSet("featureflags", "remote", `{"name":"remote","enabled":true,"strategy":"default"}`)

	require.NoError(t, e.LoadFromRedis())
	assert.True(t, e.IsEnabled("remote", ""))
}

func TestEngine_SaveToRedis(t *testing.T) {
	t.Parallel()

	e, s := newTestEngine(t)
	e.Register(&Flag{Name: "local", Enabled: true, Strategy: "default"})
	require.NoError(t, e.SaveToRedis())

	flag := s.HGet("featureflags", "local")
	require.NotEmpty(t, flag)
	assert.Contains(t, flag, `"name":"local"`)
}

func TestEngine_SetRace(t *testing.T) {
	t.Parallel()

	e, _ := newTestEngine(t)

	for i := 0; i < 100; i++ {
		go func() {
			_ = e.SetEnabled("race", true)
		}()
		go func() {
			_ = e.SetPercentage("race", 50)
		}()
	}
}
