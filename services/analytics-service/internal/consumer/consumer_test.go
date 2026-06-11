package consumer

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/IBM/sarama"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

type mockConsumerGroupSession struct {
	marked []*sarama.ConsumerMessage
}

func (m *mockConsumerGroupSession) Claims() map[string][]int32       { return nil }
func (m *mockConsumerGroupSession) MemberID() string                 { return "" }
func (m *mockConsumerGroupSession) GenerationID() int32              { return 0 }
func (m *mockConsumerGroupSession) MarkOffset(string, int32, int64, string) {}
func (m *mockConsumerGroupSession) Commit()                            {}
func (m *mockConsumerGroupSession) ResetOffset(string, int32, int64, string) {}
func (m *mockConsumerGroupSession) MarkMessage(msg *sarama.ConsumerMessage, _ string) {
	m.marked = append(m.marked, msg)
}
func (m *mockConsumerGroupSession) Context() context.Context { return context.Background() }

type mockConsumerGroupClaim struct {
	messages chan *sarama.ConsumerMessage
}

func (m *mockConsumerGroupClaim) Topic() string              { return "events" }
func (m *mockConsumerGroupClaim) Partition() int32           { return 0 }
func (m *mockConsumerGroupClaim) InitialOffset() int64       { return 0 }
func (m *mockConsumerGroupClaim) HighWaterMarkOffset() int64 { return 1 }
func (m *mockConsumerGroupClaim) Messages() <-chan *sarama.ConsumerMessage {
	return m.messages
}

func newEventMessage(t *testing.T, eventType, orderID, userID string) *sarama.ConsumerMessage {
	t.Helper()
	payload, _ := json.Marshal(Event{
		EventType: eventType,
		OrderID:   orderID,
		UserID:    userID,
	})
	return &sarama.ConsumerMessage{
		Topic:     "events",
		Partition: 0,
		Offset:    0,
		Value:     payload,
	}
}

func TestConsumerHandler_ConsumeClaim_OrderCreated(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockAnalyticsUsecase(ctrl)
	mockUC.EXPECT().TrackEvent(gomock.Any(), domain.EventTypeOrderCreated, "order-1", gomock.Any(), "user-1").Return(nil)

	h := &consumerHandler{uc: mockUC, log: zap.NewNop()}
	session := &mockConsumerGroupSession{}
	claim := &mockConsumerGroupClaim{messages: make(chan *sarama.ConsumerMessage, 1)}
	claim.messages <- newEventMessage(t, "OrderCreated", "order-1", "user-1")
	close(claim.messages)

	err := h.ConsumeClaim(session, claim)
	require.NoError(t, err)
	assert.Len(t, session.marked, 1)
}

func TestConsumerHandler_ConsumeClaim_OrderConfirmed(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockAnalyticsUsecase(ctrl)
	mockUC.EXPECT().TrackEvent(gomock.Any(), domain.EventTypeOrderConfirmed, "order-2", gomock.Any(), "user-2").Return(nil)

	h := &consumerHandler{uc: mockUC, log: zap.NewNop()}
	session := &mockConsumerGroupSession{}
	claim := &mockConsumerGroupClaim{messages: make(chan *sarama.ConsumerMessage, 1)}
	claim.messages <- newEventMessage(t, "OrderConfirmed", "order-2", "user-2")
	close(claim.messages)

	err := h.ConsumeClaim(session, claim)
	require.NoError(t, err)
	assert.Len(t, session.marked, 1)
}

func TestConsumerHandler_ConsumeClaim_OrderCancelled(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockAnalyticsUsecase(ctrl)
	mockUC.EXPECT().TrackEvent(gomock.Any(), domain.EventTypeOrderCancelled, "order-3", gomock.Any(), "user-3").Return(nil)

	h := &consumerHandler{uc: mockUC, log: zap.NewNop()}
	session := &mockConsumerGroupSession{}
	claim := &mockConsumerGroupClaim{messages: make(chan *sarama.ConsumerMessage, 1)}
	claim.messages <- newEventMessage(t, "OrderCancelled", "order-3", "user-3")
	close(claim.messages)

	err := h.ConsumeClaim(session, claim)
	require.NoError(t, err)
	assert.Len(t, session.marked, 1)
}

func TestConsumerHandler_ConsumeClaim_UnknownEvent(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockAnalyticsUsecase(ctrl)
	// no call expected

	h := &consumerHandler{uc: mockUC, log: zap.NewNop()}
	session := &mockConsumerGroupSession{}
	claim := &mockConsumerGroupClaim{messages: make(chan *sarama.ConsumerMessage, 1)}
	claim.messages <- newEventMessage(t, "UnknownEvent", "order-4", "user-4")
	close(claim.messages)

	err := h.ConsumeClaim(session, claim)
	require.NoError(t, err)
	assert.Len(t, session.marked, 1)
}

func TestConsumerHandler_ConsumeClaim_UnmarshalError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockAnalyticsUsecase(ctrl)

	h := &consumerHandler{uc: mockUC, log: zap.NewNop()}
	session := &mockConsumerGroupSession{}
	claim := &mockConsumerGroupClaim{messages: make(chan *sarama.ConsumerMessage, 1)}
	claim.messages <- &sarama.ConsumerMessage{
		Topic:     "events",
		Partition: 0,
		Offset:    0,
		Value:     []byte("not-json"),
	}
	close(claim.messages)

	err := h.ConsumeClaim(session, claim)
	require.NoError(t, err)
	assert.Len(t, session.marked, 1)
}

func TestConsumerHandler_ConsumeClaim_TrackEventError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockAnalyticsUsecase(ctrl)
	mockUC.EXPECT().TrackEvent(gomock.Any(), domain.EventTypeOrderCreated, "order-5", gomock.Any(), "user-5").Return(assert.AnError)

	h := &consumerHandler{uc: mockUC, log: zap.NewNop()}
	session := &mockConsumerGroupSession{}
	claim := &mockConsumerGroupClaim{messages: make(chan *sarama.ConsumerMessage, 1)}
	claim.messages <- newEventMessage(t, "OrderCreated", "order-5", "user-5")
	close(claim.messages)

	err := h.ConsumeClaim(session, claim)
	require.NoError(t, err)
	assert.Len(t, session.marked, 1)
}

func TestConsumer_StartClose(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockAnalyticsUsecase(ctrl)
	log := zap.NewNop()

	// Create a consumer with a bogus broker; it will fail but we only need to test Close.
	c, err := NewConsumer([]string{"127.0.0.1:1"}, "test-group", []string{"events"}, mockUC, log)
	// NewConsumer may succeed even if brokers are unreachable; sarama.NewConsumerGroup is lazy.
	if err != nil {
		// If it fails, that's fine for this test; we just want to ensure no panic.
		t.Skip("kafka not available")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)
	require.NoError(t, c.Close())
}
