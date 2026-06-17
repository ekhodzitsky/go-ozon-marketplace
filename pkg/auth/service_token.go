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
func (i *ServiceTokenIssuer) Issue() (string, error) {
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

// ServiceAuthInterceptor attaches a fresh service token to outgoing gRPC calls.
func ServiceAuthInterceptor(issuer *ServiceTokenIssuer) func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		token, err := issuer.Issue()
		if err != nil {
			return err
		}
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", token)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
