package middleware

import (
	"context"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestAuthUnaryInterceptor_PublicEndpoint(t *testing.T) {
	verifier := auth.NewJWTVerifier("secret")
	interceptor := AuthUnaryInterceptor(verifier)

	called := false
	info := &grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/Login"}
	_, err := interceptor(context.Background(), nil, info, func(ctx context.Context, req interface{}) (interface{}, error) {
		called = true
		return "ok", nil
	})

	require.NoError(t, err)
	assert.True(t, called)
}

func TestAuthUnaryInterceptor_MissingMetadata(t *testing.T) {
	verifier := auth.NewJWTVerifier("secret")
	interceptor := AuthUnaryInterceptor(verifier)

	info := &grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/GetProfile"}
	_, err := interceptor(context.Background(), nil, info, func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Fatal("handler must not be called")
		return nil, nil
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestAuthUnaryInterceptor_ValidToken(t *testing.T) {
	secret := "secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &auth.CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    "go-ozon-marketplace",
			Audience:  jwt.ClaimStrings{"api-gateway"},
			ID:        "tok-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
		Role: string(auth.RoleUser),
	})
	signed, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	verifier := auth.NewJWTVerifier(secret)
	interceptor := AuthUnaryInterceptor(verifier)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+signed))
	info := &grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/GetProfile"}

	var userID string
	var role auth.Role
	_, err = interceptor(ctx, nil, info, func(ctx context.Context, req interface{}) (interface{}, error) {
		userID, _ = GetUserID(ctx)
		role, _ = GetRole(ctx)
		return "ok", nil
	})

	require.NoError(t, err)
	assert.Equal(t, "user-1", userID)
	assert.Equal(t, auth.RoleUser, role)
}

func TestAuthUnaryInterceptor_InvalidToken(t *testing.T) {
	verifier := auth.NewJWTVerifier("secret")
	interceptor := AuthUnaryInterceptor(verifier)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer bad-token"))
	info := &grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/GetProfile"}

	_, err := interceptor(ctx, nil, info, func(ctx context.Context, req interface{}) (interface{}, error) {
		t.Fatal("handler must not be called")
		return nil, nil
	})

	st, _ := status.FromError(err)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}
