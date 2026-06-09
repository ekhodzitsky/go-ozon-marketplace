package grpc

import (
	"context"

	notificationv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/notification/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/usecase"
)

type NotificationHandler struct {
	notificationv1.UnimplementedNotificationServiceServer
	usecase *usecase.NotificationUsecase
}

func NewNotificationHandler(uc *usecase.NotificationUsecase) *NotificationHandler {
	return &NotificationHandler{usecase: uc}
}

func (h *NotificationHandler) SendEmail(ctx context.Context, req *notificationv1.SendEmailRequest) (*notificationv1.SendEmailResponse, error) {
	if err := h.usecase.SendEmail(ctx, req.To, req.Subject, req.Body); err != nil {
		return nil, err
	}

	return &notificationv1.SendEmailResponse{
		Sent: true,
	}, nil
}
