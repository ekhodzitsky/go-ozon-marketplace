package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type stubIssuer struct {
	token string
	err   error
}

func (s *stubIssuer) Issue(ctx context.Context) (string, error) {
	return s.token, s.err
}

func TestServiceAuthInterceptor_AttachesToken(t *testing.T) {
	issuer := &stubIssuer{token: "Bearer svc-token"}
	interceptor := ServiceAuthInterceptor(issuer)

	invoked := false
	err := interceptor(context.Background(), "/test.Method", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		invoked = true
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		assert.Equal(t, []string{"Bearer svc-token"}, md.Get("authorization"))
		return nil
	})

	require.NoError(t, err)
	assert.True(t, invoked)
}

func TestServiceAuthInterceptor_IssuerError(t *testing.T) {
	issuer := &stubIssuer{err: errors.New("issuer failed")}
	interceptor := ServiceAuthInterceptor(issuer)

	err := interceptor(context.Background(), "/test.Method", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		t.Fatal("invoker must not be called")
		return nil
	})

	assert.Error(t, err)
}
