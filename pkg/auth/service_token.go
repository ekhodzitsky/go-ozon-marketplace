package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// ServiceTokenIssuer signs short-lived JWTs for service-to-service calls.
type ServiceTokenIssuer struct {
	secret   string
	subject  string
	audience string
}

// NewServiceTokenIssuer creates an issuer for the given service identity.
func NewServiceTokenIssuer(secret, subject, audience string) *ServiceTokenIssuer {
	return &ServiceTokenIssuer{
		secret:   secret,
		subject:  subject,
		audience: audience,
	}
}

// Issue returns a signed Bearer token valid for 1 hour.
func (i *ServiceTokenIssuer) Issue(ctx context.Context) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   i.subject,
			Issuer:    "go-ozon-marketplace",
			Audience:  jwt.ClaimStrings{i.audience},
			ID:        uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Role: string(RoleService),
	})
	signed, err := token.SignedString([]byte(i.secret))
	if err != nil {
		return "", fmt.Errorf("sign service token: %w", err)
	}
	return "Bearer " + signed, nil
}

// UserAuthInterceptor пробрасывает Authorization-заголовок пользователя из контекста
// в исходящий gRPC-запрос. Заголовок берём из auth.Identity, который ставит HTTP-middleware.
func UserAuthInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if id, ok := IdentityFromContext(ctx); ok && id.AuthorizationHeader != "" {
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", id.AuthorizationHeader)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
