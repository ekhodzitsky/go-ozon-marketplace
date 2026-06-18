package clients

import (
	"context"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func testCircuitBreaker() *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "test",
		MaxRequests: 1,
		Interval:    0,
		Timeout:     time.Minute,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
	})
}

func TestNewFactory_AdapterUsesInsecureFlag(t *testing.T) {
	cfg := &config.Config{InsecureSkipTLS: true}
	factory := NewFactory(cfg, testCircuitBreaker())
	require.NotNil(t, factory)
}

func TestAuthForwardingInterceptor_ForwardsIdentityAuthorization(t *testing.T) {
	ctx := auth.WithIdentity(context.Background(), auth.Identity{
		UserID:              "user-1",
		AuthorizationHeader: "Bearer token123",
	})

	invoked := false
	err := authForwardingInterceptor(ctx, "/test.Method", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		invoked = true
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"Bearer token123"}, md.Get("authorization"))
		return nil
	})
	require.NoError(t, err)
	assert.True(t, invoked)
}

func TestAuthForwardingInterceptor_FallsBackToLegacyKey(t *testing.T) {
	ctx := context.WithValue(context.Background(), auth.ContextKeyAuthorizationHeader, "Bearer legacy-token")

	invoked := false
	err := authForwardingInterceptor(ctx, "/test.Method", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		invoked = true
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"Bearer legacy-token"}, md.Get("authorization"))
		return nil
	})
	require.NoError(t, err)
	assert.True(t, invoked)
}

func TestAuthForwardingInterceptor_NoAuthorization(t *testing.T) {
	ctx := context.Background()

	invoked := false
	err := authForwardingInterceptor(ctx, "/test.Method", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		invoked = true
		_, ok := metadata.FromOutgoingContext(ctx)
		assert.False(t, ok)
		return nil
	})
	require.NoError(t, err)
	assert.True(t, invoked)
}
