package consumer

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/IBM/sarama"
	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

type mockConsumerGroupSession struct {
	marked []*sarama.ConsumerMessage
}

func (m *mockConsumerGroupSession) Claims() map[string][]int32               { return nil }
func (m *mockConsumerGroupSession) MemberID() string                         { return "" }
func (m *mockConsumerGroupSession) GenerationID() int32                      { return 0 }
func (m *mockConsumerGroupSession) MarkOffset(string, int32, int64, string)  {}
func (m *mockConsumerGroupSession) Commit()                                  {}
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

func newEventMessage(t *testing.T, eventType, orderID, userID, email string) *sarama.ConsumerMessage {
	t.Helper()
	payload, _ := json.Marshal(Event{
		EventType: eventType,
		OrderID:   orderID,
		UserID:    userID,
		Email:     email,
	})
	return &sarama.ConsumerMessage{
		Topic:     "events",
		Partition: 0,
		Offset:    0,
		Value:     payload,
	}
}

func TestConsumerHandler_ConsumeClaim_OrderConfirmed(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockNotificationUsecase(ctrl)
	mockUC.EXPECT().SendEmail(gomock.Any(), "user@example.com", "Order Confirmed", gomock.Any()).Return(nil)

	h := &consumerHandler{uc: mockUC, log: zap.NewNop()}
	session := &mockConsumerGroupSession{}
	claim := &mockConsumerGroupClaim{messages: make(chan *sarama.ConsumerMessage, 1)}
	claim.messages <- newEventMessage(t, "OrderConfirmed", "order-1", "user-1", "user@example.com")
	close(claim.messages)

	err := h.ConsumeClaim(session, claim)
	require.NoError(t, err)
	assert.Len(t, session.marked, 1)
}

func TestConsumerHandler_ConsumeClaim_OrderCancelled(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockNotificationUsecase(ctrl)
	mockUC.EXPECT().SendEmail(gomock.Any(), "user@example.com", "Order Cancelled", gomock.Any()).Return(nil)

	h := &consumerHandler{uc: mockUC, log: zap.NewNop()}
	session := &mockConsumerGroupSession{}
	claim := &mockConsumerGroupClaim{messages: make(chan *sarama.ConsumerMessage, 1)}
	claim.messages <- newEventMessage(t, "OrderCancelled", "order-2", "user-2", "user@example.com")
	close(claim.messages)

	err := h.ConsumeClaim(session, claim)
	require.NoError(t, err)
	assert.Len(t, session.marked, 1)
}

func TestConsumerHandler_ConsumeClaim_PaymentFailed(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockNotificationUsecase(ctrl)
	mockUC.EXPECT().SendEmail(gomock.Any(), "user@example.com", "Payment Failed", gomock.Any()).Return(nil)

	h := &consumerHandler{uc: mockUC, log: zap.NewNop()}
	session := &mockConsumerGroupSession{}
	claim := &mockConsumerGroupClaim{messages: make(chan *sarama.ConsumerMessage, 1)}
	claim.messages <- newEventMessage(t, "PaymentFailed", "order-3", "user-3", "user@example.com")
	close(claim.messages)

	err := h.ConsumeClaim(session, claim)
	require.NoError(t, err)
	assert.Len(t, session.marked, 1)
}

func TestConsumerHandler_ConsumeClaim_UnknownEvent(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockNotificationUsecase(ctrl)

	h := &consumerHandler{uc: mockUC, log: zap.NewNop()}
	session := &mockConsumerGroupSession{}
	claim := &mockConsumerGroupClaim{messages: make(chan *sarama.ConsumerMessage, 1)}
	claim.messages <- newEventMessage(t, "UnknownEvent", "order-4", "user-4", "user@example.com")
	close(claim.messages)

	err := h.ConsumeClaim(session, claim)
	require.NoError(t, err)
	assert.Len(t, session.marked, 1)
}

func TestConsumerHandler_ConsumeClaim_UnmarshalError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockNotificationUsecase(ctrl)

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

func TestConsumerHandler_ConsumeClaim_SendEmailPermanentError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockNotificationUsecase(ctrl)
	mockUC.EXPECT().SendEmail(gomock.Any(), "user@example.com", "Order Confirmed", gomock.Any()).Return(apperrors.ErrInvalidArgument)

	h := &consumerHandler{uc: mockUC, log: zap.NewNop()}
	session := &mockConsumerGroupSession{}
	claim := &mockConsumerGroupClaim{messages: make(chan *sarama.ConsumerMessage, 1)}
	claim.messages <- newEventMessage(t, "OrderConfirmed", "order-5", "user-5", "user@example.com")
	close(claim.messages)

	err := h.ConsumeClaim(session, claim)
	require.NoError(t, err)
	assert.Len(t, session.marked, 1)
}

func TestConsumerHandler_ConsumeClaim_SendEmailTransientError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockNotificationUsecase(ctrl)
	mockUC.EXPECT().SendEmail(gomock.Any(), "user@example.com", "Order Confirmed", gomock.Any()).Return(assert.AnError).Times(3)

	h := &consumerHandler{uc: mockUC, log: zap.NewNop()}
	session := &mockConsumerGroupSession{}
	claim := &mockConsumerGroupClaim{messages: make(chan *sarama.ConsumerMessage, 1)}
	claim.messages <- newEventMessage(t, "OrderConfirmed", "order-6", "user-6", "user@example.com")
	close(claim.messages)

	err := h.ConsumeClaim(session, claim)
	require.NoError(t, err)
	assert.Len(t, session.marked, 1)
}

func TestConsumer_StartClose(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockNotificationUsecase(ctrl)
	log := zap.NewNop()

	c, err := NewConsumer([]string{"127.0.0.1:1"}, "test-group", []string{"events"}, "", mockUC, log)
	if err != nil {
		t.Skip("kafka not available")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)
	require.NoError(t, c.Close())
}
