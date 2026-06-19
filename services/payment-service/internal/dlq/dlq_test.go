package dlq_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/dlq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type mockProducer struct {
	msgs []*mockProducerMsg
	err  error
	sent chan struct{}
}

type mockProducerMsg struct {
	topic string
	key   []byte
	value []byte
}

func newMockProducer() *mockProducer {
	return &mockProducer{sent: make(chan struct{}, 1)}
}

func (m *mockProducer) SendMessage(topic string, key, value []byte) error {
	m.msgs = append(m.msgs, &mockProducerMsg{topic: topic, key: key, value: value})
	if m.sent != nil {
		m.sent <- struct{}{}
	}
	return m.err
}

func (m *mockProducer) Close() error {
	return nil
}

func TestProducer_SendToDLQ(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		mock := newMockProducer()
		p := dlq.NewProducerWithClient(mock, "dlq-topic", zap.NewNop())

		p.SendToDLQ("PaymentFailed", `{"id":"123"}`, "timeout")
		select {
		case <-mock.sent:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for dlq send")
		}
		require.Len(t, mock.msgs, 1)

		msg := mock.msgs[0]
		assert.Equal(t, "dlq-topic", msg.topic)
		assert.Equal(t, []byte("PaymentFailed"), msg.key)

		var event dlq.Event
		err := json.Unmarshal(msg.value, &event)
		require.NoError(t, err)
		assert.Equal(t, "PaymentFailed", event.EventType)
		assert.Equal(t, `{"id":"123"}`, event.Payload)
		assert.Equal(t, "timeout", event.Reason)
		assert.False(t, event.Timestamp.IsZero())
	})

	t.Run("send_error", func(t *testing.T) {
		t.Parallel()
		mock := newMockProducer()
		mock.err = errors.New("kafka down")
		p := dlq.NewProducerWithClient(mock, "dlq-topic", zap.NewNop())

		p.SendToDLQ("PaymentFailed", "", "error")
		select {
		case <-mock.sent:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for dlq send")
		}
	})

	t.Run("no_op_when_producer_nil", func(t *testing.T) {
		t.Parallel()
		p := dlq.NewProducerWithClient(nil, "dlq-topic", zap.NewNop())
		p.SendToDLQ("PaymentFailed", "", "error")
		// should return without blocking or panicking
	})
}

func TestProducer_Close(t *testing.T) {
	t.Parallel()
	mock := newMockProducer()
	p := dlq.NewProducerWithClient(mock, "dlq-topic", zap.NewNop())
	require.NoError(t, p.Close())
}

func TestProducer_NewProducer_DoesNotFailOnUnavailableKafka(t *testing.T) {
	t.Parallel()
	p, err := dlq.NewProducer([]string{"127.0.0.1:1"}, "dlq-topic", zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, p)
	require.NoError(t, p.Close())
}
