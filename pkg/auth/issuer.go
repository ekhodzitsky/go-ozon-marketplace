package auth

import "context"

// Issuer signs a token for outbound service-to-service requests.
type Issuer interface {
	Issue(ctx context.Context) (string, error)
}
