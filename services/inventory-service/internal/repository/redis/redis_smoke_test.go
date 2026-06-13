//go:build integration

package redis_test

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestRedisSmoke_BasicOperations(t *testing.T) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = container.Terminate(ctx)
	})

	endpoint, err := container.Endpoint(ctx, "")
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{Addr: endpoint})
	t.Cleanup(func() { _ = client.Close() })

	require.NoError(t, client.Ping(ctx).Err())

	key := "inventory-smoke-test"
	require.NoError(t, client.Set(ctx, key, "42", time.Minute).Err())

	val, err := client.Get(ctx, key).Result()
	require.NoError(t, err)
	require.Equal(t, "42", val)

	require.NoError(t, client.Del(ctx, key).Err())
}
