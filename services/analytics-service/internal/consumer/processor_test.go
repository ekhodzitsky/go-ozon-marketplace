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

func newEventMessage(t *testing.T, eventType, orderID, userID string) *sarama.ConsumerMessage {
	t.Helper()
	payload, _ := json.Marshal(Event{EventType: eventType, OrderID: orderID, UserID: userID})
	return &sarama.ConsumerMessage{Topic: "events", Partition: 0, Offset: 0, Value: payload}
}

func newPaymentMessage(t *testing.T, orderID string, amountCents int64) *sarama.ConsumerMessage {
	t.Helper()
	payload, _ := json.Marshal(Event{EventType: "PaymentSuccess", OrderID: orderID, AmountCents: amountCents})
	return &sarama.ConsumerMessage{Topic: "events", Partition: 0, Offset: 0, Value: payload}
}

func TestProcessor_Dispatch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		msg        *sarama.ConsumerMessage
		eventType  domain.EventType
		wantAmount float64
	}{
		{"OrderCreated", newEventMessage(t, "OrderCreated", "order-1", "user-1"), domain.EventTypeOrderCreated, 0},
		{"OrderConfirmed", newEventMessage(t, "OrderConfirmed", "order-2", "user-2"), domain.EventTypeOrderConfirmed, 0},
		{"OrderCancelled", newEventMessage(t, "OrderCancelled", "order-3", "user-3"), domain.EventTypeOrderCancelled, 0},
		{"PaymentSuccess", newPaymentMessage(t, "order-4", 1234), domain.EventTypePaymentSuccess, 12.34},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockUC := mocks.NewMockAnalyticsUsecase(ctrl)
			mockUC.EXPECT().TrackEvent(gomock.Any(), tc.eventType, gomock.Any(), gomock.Any(), "events:0:0", tc.wantAmount).Return(nil)

			p := NewProcessor(mockUC, zap.NewNop())
			require.NoError(t, p.Process(context.Background(), tc.msg))
		})
	}
}

func TestProcessor_KeyAggregation(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	msg := newEventMessage(t, "OrderCreated", "order-1", "user-1")
	msg.Key = []byte("aggregate-1")

	mockUC := mocks.NewMockAnalyticsUsecase(ctrl)
	mockUC.EXPECT().TrackEvent(gomock.Any(), domain.EventTypeOrderCreated, "order-1", gomock.Any(), "aggregate-1", float64(0)).Return(nil)

	p := NewProcessor(mockUC, zap.NewNop())
	require.NoError(t, p.Process(context.Background(), msg))
}

func TestProcessor_UnknownEvent(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockAnalyticsUsecase(ctrl)
	p := NewProcessor(mockUC, zap.NewNop())

	err := p.Process(context.Background(), newEventMessage(t, "UnknownEvent", "order-5", "user-5"))
	require.Error(t, err)
	assert.ErrorContains(t, err, "unknown analytics event type")
}

func TestProcessor_UnmarshalError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockAnalyticsUsecase(ctrl)
	p := NewProcessor(mockUC, zap.NewNop())

	err := p.Process(context.Background(), &sarama.ConsumerMessage{Topic: "events", Value: []byte("not-json")})
	require.Error(t, err)
	assert.ErrorContains(t, err, "unmarshal analytics event")
}

func TestProcessor_TrackEventError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockAnalyticsUsecase(ctrl)
	mockUC.EXPECT().TrackEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(assert.AnError)

	p := NewProcessor(mockUC, zap.NewNop())
	err := p.Process(context.Background(), newEventMessage(t, "OrderCreated", "order-6", "user-6"))
	require.ErrorIs(t, err, assert.AnError)
}
