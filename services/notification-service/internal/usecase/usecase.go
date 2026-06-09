package usecase

import (
	"context"

	"go.uber.org/zap"
)

type NotificationUsecase struct {
	log *zap.Logger
}

func NewNotificationUsecase(log *zap.Logger) *NotificationUsecase {
	return &NotificationUsecase{log: log}
}

func (u *NotificationUsecase) SendEmail(ctx context.Context, to, subject, body string) error {
	u.log.Info("notification sent",
		zap.String("type", "email"),
		zap.String("to", to),
		zap.String("subject", subject),
		zap.String("body", body),
	)
	return nil
}
