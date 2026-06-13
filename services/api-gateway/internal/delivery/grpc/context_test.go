package grpc_test

import (
	"context"
	"net/http"
	"testing"

	gatewaygrpc "github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/delivery/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestAppendAuthFromHTTP_NoHeader(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/query", nil)
	require.NoError(t, err)

	ctx := gatewaygrpc.AppendAuthFromHTTP(context.Background(), req)
	md, ok := metadata.FromOutgoingContext(ctx)
	assert.False(t, ok)
	assert.Empty(t, md.Get("authorization"))
}

func TestAppendAuthFromHTTP_WithHeader(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/query", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer token123")

	ctx := gatewaygrpc.AppendAuthFromHTTP(context.Background(), req)
	md, ok := metadata.FromOutgoingContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, []string{"Bearer token123"}, md.Get("authorization"))
}
