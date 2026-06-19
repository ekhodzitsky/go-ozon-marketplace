package ws

import (
	"context"
	"log"

	"github.com/olahol/melody"
	"github.com/redis/go-redis/v9"
)

// StartRedisPubSub subscribes to Redis channels and forwards messages to the hub.
func StartRedisPubSub(ctx context.Context, redisClient *redis.Client, m *melody.Melody) {
	pubsub := redisClient.PSubscribe(ctx, "order-events", "inventory-events")
	defer func() { _ = pubsub.Close() }()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				log.Println("redis pubsub channel closed")
				return
			}
			if err := Broadcast(m, []byte(msg.Payload)); err != nil {
				log.Printf("broadcast error: %v", err)
			}
		}
	}
}
