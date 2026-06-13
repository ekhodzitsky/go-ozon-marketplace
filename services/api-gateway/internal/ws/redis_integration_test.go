//go:build integration

package ws_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	pkgredis "github.com/ekhodzitsky/go-ozon-marketplace/pkg/redis"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestStartRedisPubSub_ForwardsMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	container, err := tcredis.Run(ctx, "redis:7-alpine")
	require.NoError(t, err)
	defer func() { _ = container.Terminate(ctx) }()

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "6379/tcp")
	require.NoError(t, err)
	addr := fmt.Sprintf("%s:%s", host, port.Port())

	client, err := pkgredis.NewClient(ctx, addr)
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	publisher, err := pkgredis.NewClient(ctx, addr)
	require.NoError(t, err)
	defer func() { _ = publisher.Close() }()

	hub := ws.NewHub()
	// Do not start hub.Run so the broadcast channel is only consumed by the test.

	pubSubCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go ws.StartRedisPubSub(pubSubCtx, client, hub)

	// Give the pubsub subscription time to establish.
	time.Sleep(500 * time.Millisecond)

	publisher.Publish(ctx, "order-events", `{"topic":"orders","user_id":"user-1","payload":{"order_id":"order-123","status":"paid"}}`)

	select {
	case msg := <-hub.BroadcastChannel():
		assert.Contains(t, string(msg), "order-123")
	case <-time.After(5 * time.Second):
		t.Fatal("expected message to be broadcast via Redis pubsub")
	}
}
