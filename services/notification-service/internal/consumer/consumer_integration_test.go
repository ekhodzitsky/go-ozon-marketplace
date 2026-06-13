//go:build integration

package consumer

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"
	"go.uber.org/zap"
)

type integrationUsecase struct {
	mu        sync.Mutex
	calls     []sendCall
	returnErr error
}

type sendCall struct {
	To      string
	Subject string
	Body    string
}

func (u *integrationUsecase) SendEmail(ctx context.Context, to, subject, body string) error {
	if u.returnErr != nil {
		return u.returnErr
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.calls = append(u.calls, sendCall{To: to, Subject: subject, Body: body})
	return nil
}

func (u *integrationUsecase) Calls() []sendCall {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]sendCall{}, u.calls...)
}

var (
	sharedBroker string
	kafkaOnce    sync.Once
	kafkaCleanup func()
)

func startSharedKafka(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	kafkaOnce.Do(func() {
		ctx := context.Background()
		container, err := redpanda.Run(
			ctx,
			"redpandadata/redpanda:v24.1.1",
			redpanda.WithAutoCreateTopics(),
		)
		require.NoError(t, err)

		kafkaCleanup = func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = container.Terminate(ctx)
		}

		sharedBroker, err = container.KafkaSeedBroker(ctx)
		require.NoError(t, err)
	})

	return sharedBroker
}

func newProducer(t *testing.T, brokers []string) sarama.SyncProducer {
	t.Helper()
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	producer, err := sarama.NewSyncProducer(brokers, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = producer.Close() })
	return producer
}

func produceEvent(t *testing.T, producer sarama.SyncProducer, topic string, event Event) {
	t.Helper()
	payload, err := json.Marshal(event)
	require.NoError(t, err)
	_, _, err = producer.SendMessage(&sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(payload),
	})
	require.NoError(t, err)
}

func TestMain(m *testing.M) {
	code := m.Run()
	if kafkaCleanup != nil {
		kafkaCleanup()
	}
	os.Exit(code)
}

func TestConsumer_ReceiveOrderConfirmed(t *testing.T) {
	broker := startSharedKafka(t)
	brokers := []string{broker}
	topic := "order-events-confirmed"
	groupID := "notification-integration-test-confirmed"

	uc := &integrationUsecase{}
	log := zap.NewNop()

	c, err := NewConsumer(brokers, groupID, []string{topic}, "", uc, log)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)

	producer := newProducer(t, brokers)
	produceEvent(t, producer, topic, Event{
		EventType: "OrderConfirmed",
		OrderID:   "order-123",
		UserID:    "user-123",
		Email:     "customer@example.com",
	})

	require.Eventually(t, func() bool {
		return len(uc.Calls()) == 1
	}, 30*time.Second, 500*time.Millisecond, "expected SendEmail to be called")

	calls := uc.Calls()
	require.Len(t, calls, 1)
	assert.Equal(t, "customer@example.com", calls[0].To)
	assert.Equal(t, "Order Confirmed", calls[0].Subject)
	assert.Contains(t, calls[0].Body, "order-123")

	require.NoError(t, c.Close())
	cancel()
}

func TestConsumer_ReceivePaymentFailed(t *testing.T) {
	broker := startSharedKafka(t)
	brokers := []string{broker}
	topic := "order-events-payment"
	groupID := "notification-integration-test-payment"

	uc := &integrationUsecase{}
	log := zap.NewNop()

	c, err := NewConsumer(brokers, groupID, []string{topic}, "", uc, log)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)

	producer := newProducer(t, brokers)
	produceEvent(t, producer, topic, Event{
		EventType: "PaymentFailed",
		OrderID:   "order-456",
		UserID:    "user-456",
		Email:     "payer@example.com",
	})

	require.Eventually(t, func() bool {
		return len(uc.Calls()) == 1
	}, 30*time.Second, 500*time.Millisecond, "expected SendEmail to be called")

	calls := uc.Calls()
	require.Len(t, calls, 1)
	assert.Equal(t, "payer@example.com", calls[0].To)
	assert.Equal(t, "Payment Failed", calls[0].Subject)

	require.NoError(t, c.Close())
	cancel()
}

func TestConsumer_UnknownEventNotProcessed(t *testing.T) {
	broker := startSharedKafka(t)
	brokers := []string{broker}
	topic := "order-events-unknown"
	groupID := "notification-integration-test-unknown"

	uc := &integrationUsecase{}
	log := zap.NewNop()

	c, err := NewConsumer(brokers, groupID, []string{topic}, "", uc, log)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)

	producer := newProducer(t, brokers)
	produceEvent(t, producer, topic, Event{
		EventType: "UnknownEvent",
		OrderID:   "order-789",
		UserID:    "user-789",
		Email:     "nobody@example.com",
	})

	require.Never(t, func() bool {
		return len(uc.Calls()) > 0
	}, 3*time.Second, 500*time.Millisecond, "expected SendEmail not to be called")

	require.NoError(t, c.Close())
	cancel()
}
