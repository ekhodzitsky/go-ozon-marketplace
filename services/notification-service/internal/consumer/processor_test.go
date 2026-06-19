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

func newEventMessage(t *testing.T, eventType, orderID, userID, email string) *sarama.ConsumerMessage {
	t.Helper()
	payload, _ := json.Marshal(Event{
		EventType: eventType,
		OrderID:   orderID,
		UserID:    userID,
		Email:     email,
	})
	return &sarama.ConsumerMessage{Topic: "events", Partition: 0, Offset: 0, Value: payload}
}

func TestProcessor_Dispatch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		event   string
		subject string
	}{
		{"OrderConfirmed", "OrderConfirmed", "Order Confirmed"},
		{"OrderCancelled", "OrderCancelled", "Order Cancelled"},
		{"PaymentFailed", "PaymentFailed", "Payment Failed"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockUC := mocks.NewMockNotificationUsecase(ctrl)
			mockUC.EXPECT().SendEmail(gomock.Any(), "user@example.com", tc.subject, gomock.Any()).Return(nil)

			p := NewProcessor(mockUC, zap.NewNop())
			err := p.Process(context.Background(), newEventMessage(t, tc.event, "order-1", "user-1", "user@example.com"))
			require.NoError(t, err)
		})
	}
}

func TestProcessor_UnknownEvent(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockNotificationUsecase(ctrl)
	p := NewProcessor(mockUC, zap.NewNop())

	err := p.Process(context.Background(), newEventMessage(t, "UnknownEvent", "order-1", "user-1", "user@example.com"))
	require.NoError(t, err)
}

func TestProcessor_UnmarshalError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockNotificationUsecase(ctrl)
	p := NewProcessor(mockUC, zap.NewNop())

	err := p.Process(context.Background(), &sarama.ConsumerMessage{Topic: "events", Value: []byte("not-json")})
	require.NoError(t, err)
}

func TestProcessor_SendEmailError(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUC := mocks.NewMockNotificationUsecase(ctrl)
	mockUC.EXPECT().SendEmail(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(apperrors.ErrInvalidArgument)

	p := NewProcessor(mockUC, zap.NewNop())
	err := p.Process(context.Background(), newEventMessage(t, "OrderConfirmed", "order-1", "user-1", "user@example.com"))
	require.Error(t, err)
	assert.ErrorIs(t, err, apperrors.ErrInvalidArgument)
}
