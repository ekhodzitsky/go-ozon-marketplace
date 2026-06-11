package usecase

import "context"

// NotificationUsecase defines the notification use-case boundary.
type NotificationUsecase interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}
