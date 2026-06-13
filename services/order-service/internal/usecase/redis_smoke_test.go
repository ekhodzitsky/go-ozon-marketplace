//go:build integration

package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestRedisSmoke_PublishSubscribe(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

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

	pubsub := client.Subscribe(ctx, "order-events")
	t.Cleanup(func() { _ = pubsub.Close() })

	msgCh := pubsub.Channel()

	require.NoError(t, client.Publish(ctx, "order-events", `{"topic":"orders"}`).Err())

	select {
	case msg := <-msgCh:
		require.Equal(t, `{"topic":"orders"}`, msg.Payload)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for redis message")
	}
}
