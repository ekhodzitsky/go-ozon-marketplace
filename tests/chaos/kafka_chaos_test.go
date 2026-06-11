//go:build chaos

package chaos

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	"github.com/google/uuid"
)

func TestOutboxWithKafkaDown(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}
	dockerComposeUp(t)
	runMigrations(t)

	ctx := context.Background()
	orderClient := newOrderClient(t)

	userID := uuid.New().String()
	productID := uuid.New().String()
	ensureStock(t, productID, 100)

	// Stop Kafka (Redpanda)
	dockerStop(t, "go-ozon-marketplace-redpanda-1")

	// Create order while Kafka is down
	_, err := orderClient.CreateOrder(authContext(ctx, userID), &orderv1.CreateOrderRequest{
		UserId: userID,
		Items: []*orderv1.OrderItem{
			{ProductId: productID, Quantity: 1, Price: 49.99},
		},
	})
	if err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	// Give outbox a moment to write
	time.Sleep(500 * time.Millisecond)

	// Verify outbox accumulated unprocessed events
	unprocessed := getOutboxUnprocessedCount(t)
	if unprocessed == 0 {
		t.Fatal("expected outbox to have unprocessed events while kafka is down")
	}

	// Restart Kafka
	dockerStart(t, "go-ozon-marketplace-redpanda-1")

	// Wait for outbox relay to process
	time.Sleep(8 * time.Second)

	unprocessed = getOutboxUnprocessedCount(t)
	if unprocessed != 0 {
		t.Fatalf("expected outbox to be empty after kafka restarted, got %d unprocessed", unprocessed)
	}
}

type lagConsumer struct {
	mu    sync.Mutex
	count int
	delay time.Duration
}

func (c *lagConsumer) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (c *lagConsumer) Cleanup(sarama.ConsumerGroupSession) error { return nil }
func (c *lagConsumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		time.Sleep(c.delay)
		session.MarkMessage(msg, "")
		c.mu.Lock()
		c.count++
		c.mu.Unlock()
	}
	return nil
}

func (c *lagConsumer) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

func TestKafkaConsumerLag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in short mode")
	}
	dockerComposeUp(t)

	brokers := []string{"localhost:19092"}
	topic := "chaos-test-lag"
	groupID := "chaos-lag-group"

	// Ensure topic exists by sending a dummy message
	cfg := sarama.NewConfig()
	cfg.Producer.RequiredAcks = sarama.WaitForLocal
	cfg.Producer.Return.Successes = true
	producer, err := sarama.NewSyncProducer(brokers, cfg)
	if err != nil {
		t.Fatalf("failed to create producer: %v", err)
	}
	defer producer.Close()

	_, _, err = producer.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder("init"),
	})
	if err != nil {
		t.Fatalf("failed to init topic: %v", err)
	}

	// Start slow consumer
	consumer := &lagConsumer{delay: 100 * time.Millisecond}
	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		t.Fatalf("failed to create consumer group: %v", err)
	}
	defer group.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for ctx.Err() == nil {
			_ = group.Consume(ctx, []string{topic}, consumer)
		}
	}()

	// Wait for consumer to join
	time.Sleep(2 * time.Second)

	// Generate 100 events
	for i := 0; i < 100; i++ {
		_, _, err = producer.SendMessage(&sarama.ProducerMessage{
			Topic: topic,
			Key:   sarama.StringEncoder(uuid.New().String()),
			Value: sarama.StringEncoder("event"),
		})
		if err != nil {
			t.Fatalf("failed to send message: %v", err)
		}
	}

	// Wait a bit and verify lag exists
	time.Sleep(3 * time.Second)
	if consumer.Count() >= 100 {
		t.Fatal("expected consumer lag, but all messages were processed immediately")
	}

	// Unblock consumer by reducing delay
	consumer.delay = 0

	// Wait for all messages to be processed
	time.Sleep(5 * time.Second)
	if consumer.Count() != 100 {
		t.Fatalf("expected 100 processed events, got %d", consumer.Count())
	}
}
