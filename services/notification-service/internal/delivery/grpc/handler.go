package grpc

import (
	"context"

	notificationv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/notification/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/usecase"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
)

type NotificationHandler struct {
	notificationv1.UnimplementedNotificationServiceServer
	usecase usecase.NotificationUsecase
}

func NewNotificationHandler(uc usecase.NotificationUsecase) *NotificationHandler {
	return &NotificationHandler{usecase: uc}
}

func (h *NotificationHandler) SendEmail(ctx context.Context, req *notificationv1.SendEmailRequest) (*notificationv1.SendEmailResponse, error) {
	if err := middleware.RequireRole(ctx, middleware.RoleService); err != nil {
		return nil, err
	}

	if err := h.usecase.SendEmail(ctx, req.To, req.Subject, req.Body); err != nil {
		return nil, apperrors.ToStatus(err)
	}

	return &notificationv1.SendEmailResponse{
		Sent: true,
	}, nil
}
