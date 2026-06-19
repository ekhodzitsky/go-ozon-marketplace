package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestUserAuthInterceptor_ForwardsAuthorizationHeader(t *testing.T) {
	ctx := WithIdentity(context.Background(), Identity{
		UserID:              "user-1",
		AuthorizationHeader: "Bearer user-token",
	})

	interceptor := UserAuthInterceptor()
	invoked := false
	err := interceptor(ctx, "/test.Method", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		invoked = true
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"Bearer user-token"}, md.Get("authorization"))
		return nil
	})

	require.NoError(t, err)
	assert.True(t, invoked)
}

func TestUserAuthInterceptor_NoIdentity(t *testing.T) {
	ctx := context.Background()

	interceptor := UserAuthInterceptor()
	invoked := false
	err := interceptor(ctx, "/test.Method", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		invoked = true
		_, ok := metadata.FromOutgoingContext(ctx)
		assert.False(t, ok)
		return nil
	})

	require.NoError(t, err)
	assert.True(t, invoked)
}
