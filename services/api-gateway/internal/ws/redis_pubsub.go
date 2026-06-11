package ws

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

// StartRedisPubSub subscribes to Redis channels and forwards messages to the Hub.
func StartRedisPubSub(ctx context.Context, redisClient *redis.Client, hub *Hub) {
	pubsub := redisClient.PSubscribe(ctx, "order-events", "inventory-events")
	defer pubsub.Close()

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
			hub.Broadcast([]byte(msg.Payload))
		}
	}
}
