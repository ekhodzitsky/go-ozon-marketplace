package dlq_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/IBM/sarama"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/dlq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSyncProducer struct {
	msgs []*sarama.ProducerMessage
	err  error
}

func (m *mockSyncProducer) SendMessage(msg *sarama.ProducerMessage) (int32, int64, error) {
	m.msgs = append(m.msgs, msg)
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
		mock := &mockSyncProducer{}
		p := dlq.NewProducerWithClient(mock, "dlq-topic")

		err := p.SendToDLQ("PaymentFailed", `{"id":"123"}`, "timeout")
		require.NoError(t, err)
		require.Len(t, mock.msgs, 1)

		msg := mock.msgs[0]
		assert.Equal(t, "dlq-topic", msg.Topic)
		assert.Equal(t, sarama.ByteEncoder([]byte("PaymentFailed")), msg.Key)

		var event dlq.Event
		err = json.Unmarshal(msg.Value.(sarama.ByteEncoder), &event)
		require.NoError(t, err)
		assert.Equal(t, "PaymentFailed", event.EventType)
		assert.Equal(t, `{"id":"123"}`, event.Payload)
		assert.Equal(t, "timeout", event.Reason)
		assert.False(t, event.Timestamp.IsZero())
	})

	t.Run("send_error", func(t *testing.T) {
		t.Parallel()
		mock := &mockSyncProducer{err: errors.New("kafka down")}
		p := dlq.NewProducerWithClient(mock, "dlq-topic")

		err := p.SendToDLQ("PaymentFailed", "", "error")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "kafka down")
	})
}

func TestProducer_Close(t *testing.T) {
	t.Parallel()
	mock := &mockSyncProducer{}
	p := dlq.NewProducerWithClient(mock, "dlq-topic")
	require.NoError(t, p.Close())
}
