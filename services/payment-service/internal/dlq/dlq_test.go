package dlq_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/dlq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type mockSyncProducer struct {
	msgs []*sarama.ProducerMessage
	err  error
	sent chan struct{}
}

func newMockSyncProducer() *mockSyncProducer {
	return &mockSyncProducer{sent: make(chan struct{}, 1)}
}

func (m *mockSyncProducer) SendMessage(msg *sarama.ProducerMessage) (int32, int64, error) {
	m.msgs = append(m.msgs, msg)
	if m.sent != nil {
		m.sent <- struct{}{}
	}
	return 0, 0, m.err
}

func (m *mockSyncProducer) SendMessages(msgs []*sarama.ProducerMessage) error {
	m.msgs = append(m.msgs, msgs...)
	return nil
}

func (m *mockSyncProducer) Close() error {
	return nil
}

func (m *mockSyncProducer) AbortTxn() error { return nil }
func (m *mockSyncProducer) AddMessageToTxn(msg *sarama.ConsumerMessage, groupID string, metadata *string) error {
	return nil
}
func (m *mockSyncProducer) AddMessageToTxnWithGroupMetadata(msg *sarama.ConsumerMessage, group *sarama.ConsumerGroupMetadata, metadata *string) error {
	return nil
}
func (m *mockSyncProducer) AddOffsetsToTxn(offsets map[string][]*sarama.PartitionOffsetMetadata, groupID string) error {
	return nil
}
func (m *mockSyncProducer) AddOffsetsToTxnWithGroupMetadata(offsets map[string][]*sarama.PartitionOffsetMetadata, groupMetadata *sarama.ConsumerGroupMetadata) error {
	return nil
}
func (m *mockSyncProducer) BeginTxn() error                         { return nil }
func (m *mockSyncProducer) CommitTxn() error                        { return nil }
func (m *mockSyncProducer) InitTransactions() error                 { return nil }
func (m *mockSyncProducer) IsTransactional() bool                   { return false }
func (m *mockSyncProducer) TxnStatus() sarama.ProducerTxnStatusFlag { return 0 }

func TestProducer_SendToDLQ(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		mock := newMockSyncProducer()
		p := dlq.NewProducerWithClient(mock, "dlq-topic", zap.NewNop())

		p.SendToDLQ("PaymentFailed", `{"id":"123"}`, "timeout")
		select {
		case <-mock.sent:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for dlq send")
		}
		require.Len(t, mock.msgs, 1)

		msg := mock.msgs[0]
		assert.Equal(t, "dlq-topic", msg.Topic)
		assert.Equal(t, sarama.ByteEncoder([]byte("PaymentFailed")), msg.Key)

		var event dlq.Event
		err := json.Unmarshal(msg.Value.(sarama.ByteEncoder), &event)
		require.NoError(t, err)
		assert.Equal(t, "PaymentFailed", event.EventType)
		assert.Equal(t, `{"id":"123"}`, event.Payload)
		assert.Equal(t, "timeout", event.Reason)
		assert.False(t, event.Timestamp.IsZero())
	})

	t.Run("send_error", func(t *testing.T) {
		t.Parallel()
		mock := newMockSyncProducer()
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
	mock := newMockSyncProducer()
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
