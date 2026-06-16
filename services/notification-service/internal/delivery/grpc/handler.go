package grpc

import (
	"context"
	"fmt"
	"net/mail"

	notificationv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/notification/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
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
	if err := middleware.RequireRole(ctx, auth.RoleService); err != nil {
		return nil, err
	}

	if err := validateSendEmailRequest(req); err != nil {
		return nil, apperrors.ToStatus(err)
	}

	if err := h.usecase.SendEmail(ctx, req.To, req.Subject, req.Body); err != nil {
		return nil, apperrors.ToStatus(err)
	}

	return &notificationv1.SendEmailResponse{
		Sent: true,
	}, nil
}

func validateSendEmailRequest(req *notificationv1.SendEmailRequest) error {
	if req == nil {
		return fmt.Errorf("%w: request is nil", apperrors.ErrInvalidArgument)
	}
	if _, err := mail.ParseAddress(req.To); err != nil || req.To == "" {
		return fmt.Errorf("%w: invalid email address", apperrors.ErrInvalidArgument)
	}
	if req.Subject == "" {
		return fmt.Errorf("%w: subject is required", apperrors.ErrInvalidArgument)
	}
	if req.Body == "" {
		return fmt.Errorf("%w: body is required", apperrors.ErrInvalidArgument)
	}
	return nil
}
