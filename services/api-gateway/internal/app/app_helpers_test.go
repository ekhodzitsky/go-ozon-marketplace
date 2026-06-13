package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/circuitbreaker"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/ast"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestServerNameFromAddr(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"localhost:50051", "localhost"},
		{"user-service:50051", "user-service"},
		{"127.0.0.1:8080", "127.0.0.1"},
		{"no-port", "no-port"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, serverNameFromAddr(tc.input))
		})
	}
}

func TestClientCreds_InsecureSkipTLS(t *testing.T) {
	cfg := &config.Config{InsecureSkipTLS: true}
	creds, err := clientCreds(cfg, "localhost:50051")
	require.NoError(t, err)
	assert.NotNil(t, creds)
}

func TestClientCreds_MissingTLSConfig(t *testing.T) {
	cfg := &config.Config{InsecureSkipTLS: false, CertPath: ""}
	_, err := clientCreds(cfg, "localhost:50051")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no CERT_PATH configured")
}

func TestCorsMiddleware_AllowedOrigin(t *testing.T) {
	middleware := corsMiddleware([]string{"https://app.example.com"})
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/query", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "https://app.example.com", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCorsMiddleware_DisallowedOrigin(t *testing.T) {
	middleware := corsMiddleware([]string{"https://app.example.com"})
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/query", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCorsMiddleware_Preflight(t *testing.T) {
	middleware := corsMiddleware([]string{"https://app.example.com"})
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for preflight")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/query", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireAdminHTTP_MissingAuth(t *testing.T) {
	middleware := requireAdminHTTP("secret")
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/flags", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireAdminHTTP_InvalidToken(t *testing.T) {
	middleware := requireAdminHTTP("secret")
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/flags", nil)
	req.Header.Set("Authorization", "Bearer invalid")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireAdminHTTP_ForbiddenForUser(t *testing.T) {
	secret := "test-secret-must-be-long-enough"
	token := buildAdminJWT(t, secret, "user-1", "user")

	middleware := requireAdminHTTP(secret)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/flags", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireAdminHTTP_AdminAllowed(t *testing.T) {
	secret := "test-secret-must-be-long-enough"
	token := buildAdminJWT(t, secret, "admin-1", "admin")

	middleware := requireAdminHTTP(secret)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/flags", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func buildAdminJWT(t *testing.T, secret, subject, role string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  subject,
		"role": role,
	})
	s, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	return s
}

func TestAuthClientInterceptor_ForwardsAuthorization(t *testing.T) {
	opCtx := &graphql.OperationContext{
		Headers: http.Header{"Authorization": []string{"Bearer token123"}},
		Operation: &ast.OperationDefinition{
			Operation: ast.Mutation,
		},
	}
	ctx := graphql.WithOperationContext(context.Background(), opCtx)

	invoked := false
	interceptor := authClientInterceptor
	err := interceptor(ctx, "/test.Method", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		invoked = true
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"Bearer token123"}, md.Get("authorization"))
		return nil
	})
	require.NoError(t, err)
	assert.True(t, invoked)
}

func TestAuthClientInterceptor_NoAuthorizationHeader(t *testing.T) {
	opCtx := &graphql.OperationContext{
		Headers: http.Header{},
	}
	ctx := graphql.WithOperationContext(context.Background(), opCtx)

	invoked := false
	err := authClientInterceptor(ctx, "/test.Method", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		invoked = true
		_, ok := metadata.FromOutgoingContext(ctx)
		assert.False(t, ok)
		return nil
	})
	require.NoError(t, err)
	assert.True(t, invoked)
}

func TestCircuitBreakerClientInterceptor(t *testing.T) {
	cb := circuitbreaker.New(5, 2, 0)
	interceptor := circuitBreakerClientInterceptor(cb)

	invoked := false
	err := interceptor(context.Background(), "/test.Method", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		invoked = true
		return nil
	})
	require.NoError(t, err)
	assert.True(t, invoked)
}

func TestCircuitBreakerClientInterceptor_OpensAfterFailures(t *testing.T) {
	cb := circuitbreaker.New(1, 1, time.Minute)
	interceptor := circuitBreakerClientInterceptor(cb)

	err := interceptor(context.Background(), "/test.Method", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return errors.New("boom")
	})
	require.Error(t, err)

	err = interceptor(context.Background(), "/test.Method", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker")
}
