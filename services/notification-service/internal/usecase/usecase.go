package usecase

import (
	"context"
	"time"

	"go.uber.org/zap"
)

const (
	DefaultCallTimeout  = 5 * time.Second
	DefaultQueryTimeout = 3 * time.Second
)

type notificationUsecase struct {
	log          *zap.Logger
	callTimeout  time.Duration
	queryTimeout time.Duration
}

func NewNotificationUsecase(log *zap.Logger, callTimeout time.Duration, queryTimeout time.Duration) NotificationUsecase {
	return &notificationUsecase{log: log, callTimeout: callTimeout, queryTimeout: queryTimeout}
}

func (u *notificationUsecase) SendEmail(ctx context.Context, to, subject, body string) error {
	ctx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()
	u.log.Info("notification sent",
		zap.String("type", "email"),
		zap.String("to", to),
		zap.String("subject", subject),
		zap.String("body", body),
	)
	return nil
}
