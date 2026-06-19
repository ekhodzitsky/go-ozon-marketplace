package auth

import "context"

// Issuer подписывает токен для исходящих запросов между сервисами.
type Issuer interface {
	Issue(ctx context.Context) (string, error)
}
