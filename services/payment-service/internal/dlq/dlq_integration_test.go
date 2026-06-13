//go:build integration

package dlq_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/dlq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"
	"go.uber.org/zap"
)

func startKafka(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	container, err := redpanda.Run(ctx, "redpandadata/redpanda:v24.1.1")
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = container.Terminate(ctx)
	})

	broker, err := container.KafkaSeedBroker(ctx)
	require.NoError(t, err)
	return broker
}

func TestProducer_SendToDLQ_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	broker := startKafka(t)
	topic := "payment-dlq-test"

	// Ensure topic exists before producing/consuming.
	admin, err := sarama.NewClusterAdmin([]string{broker}, sarama.NewConfig())
	require.NoError(t, err)
	require.NoError(t, admin.CreateTopic(topic, &sarama.TopicDetail{NumPartitions: 1, ReplicationFactor: 1}, false))
	require.NoError(t, admin.Close())

	producer, err := dlq.NewProducer([]string{broker}, topic, zap.NewNop())
	require.NoError(t, err)
	defer func() { _ = producer.Close() }()

	producer.SendToDLQ("PaymentFailed", `{"id":"123"}`, "timeout")

	// Consume the message to verify delivery.
	config := sarama.NewConfig()
	config.Consumer.Return.Errors = true
	consumer, err := sarama.NewConsumer([]string{broker}, config)
	require.NoError(t, err)
	defer func() { _ = consumer.Close() }()

	partitionConsumer, err := consumer.ConsumePartition(topic, 0, sarama.OffsetOldest)
	require.NoError(t, err)
	defer func() { _ = partitionConsumer.Close() }()

	select {
	case msg := <-partitionConsumer.Messages():
		require.NotNil(t, msg)
		assert.Equal(t, topic, msg.Topic)
		assert.Equal(t, []byte("PaymentFailed"), msg.Key)

		var event dlq.Event
		require.NoError(t, json.Unmarshal(msg.Value, &event))
		assert.Equal(t, "PaymentFailed", event.EventType)
		assert.Equal(t, `{"id":"123"}`, event.Payload)
		assert.Equal(t, "timeout", event.Reason)
		assert.False(t, event.Timestamp.IsZero())
	case err := <-partitionConsumer.Errors():
		t.Fatalf("consumer error: %v", err)
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for dlq message")
	}
}
