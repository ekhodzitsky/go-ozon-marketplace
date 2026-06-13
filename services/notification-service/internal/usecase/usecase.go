package usecase

import (
	"context"
	"fmt"
	"net/mail"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/notification-service/internal/email"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"go.uber.org/zap"
)

const (
	DefaultCallTimeout  = 5 * time.Second
	DefaultQueryTimeout = 3 * time.Second
)

type notificationUsecase struct {
	log          *zap.Logger
	provider     email.Provider
	callTimeout  time.Duration
	queryTimeout time.Duration
}

func NewNotificationUsecase(log *zap.Logger, provider email.Provider, callTimeout time.Duration, queryTimeout time.Duration) NotificationUsecase {
	return &notificationUsecase{log: log, provider: provider, callTimeout: callTimeout, queryTimeout: queryTimeout}
}

func (u *notificationUsecase) SendEmail(ctx context.Context, to, subject, body string) error {
	if err := validateSendEmail(to, subject, body); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()

	if err := u.provider.Send(ctx, to, subject, body); err != nil {
		u.log.Error("failed to send email",
			zap.String("to", email.MaskEmail(to)),
			zap.String("subject", subject),
			zap.Error(err),
		)
		return fmt.Errorf("send email: %w", err)
	}

	u.log.Info("notification sent",
		zap.String("type", "email"),
		zap.String("to", email.MaskEmail(to)),
		zap.String("subject", subject),
	)
	return nil
}

func validateSendEmail(to, subject, body string) error {
	if _, err := mail.ParseAddress(to); err != nil || to == "" {
		return fmt.Errorf("%w: invalid email address", apperrors.ErrInvalidArgument)
	}
	if subject == "" {
		return fmt.Errorf("%w: subject is required", apperrors.ErrInvalidArgument)
	}
	if body == "" {
		return fmt.Errorf("%w: body is required", apperrors.ErrInvalidArgument)
	}
	return nil
}
