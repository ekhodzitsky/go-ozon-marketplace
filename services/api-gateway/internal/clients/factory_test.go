package clients

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/circuitbreaker"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	factory := NewFactory(cfg, circuitbreaker.New(5, 2, 30*time.Second))
	creds, err := factory.clientCreds("localhost:50051")
	require.NoError(t, err)
	assert.NotNil(t, creds)
}

func TestClientCreds_MissingTLSConfig(t *testing.T) {
	cfg := &config.Config{InsecureSkipTLS: false, CertPath: ""}
	factory := NewFactory(cfg, circuitbreaker.New(5, 2, 30*time.Second))
	_, err := factory.clientCreds("localhost:50051")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no CERT_PATH configured")
}

func TestClientCreds_WithCertPath(t *testing.T) {
	tmp := t.TempDir()
	generateTestCerts(t, tmp)

	cfg := &config.Config{CertPath: tmp}
	factory := NewFactory(cfg, circuitbreaker.New(5, 2, 30*time.Second))
	creds, err := factory.clientCreds("localhost:50051")
	require.NoError(t, err)
	assert.NotNil(t, creds)
}

func generateTestCerts(t *testing.T, dir string) {
	t.Helper()

	caKey := filepath.Join(dir, "ca-key.pem")
	caCert := filepath.Join(dir, "ca-cert.pem")
	serverKey := filepath.Join(dir, "server-key.pem")
	serverCert := filepath.Join(dir, "server-cert.pem")
	serverCSR := filepath.Join(dir, "server.csr")

	cmd := exec.Command("openssl", "req", "-x509", "-newkey", "rsa:2048",
		"-keyout", caKey, "-out", caCert,
		"-days", "1", "-nodes",
		"-subj", "/CN=Test CA", "-batch")
	require.NoError(t, cmd.Run())

	cmd = exec.Command("openssl", "req", "-newkey", "rsa:2048",
		"-keyout", serverKey, "-out", serverCSR,
		"-nodes", "-subj", "/CN=localhost", "-batch")
	require.NoError(t, cmd.Run())

	cmd = exec.Command("openssl", "x509", "-req", "-in", serverCSR,
		"-CA", caCert, "-CAkey", caKey,
		"-CAcreateserial", "-out", serverCert, "-days", "1")
	require.NoError(t, cmd.Run())

	require.NoError(t, os.Remove(serverCSR))
}

func TestAuthClientInterceptor_ForwardsAuthorization(t *testing.T) {
	ctx := context.WithValue(context.Background(), auth.ContextKeyAuthorizationHeader, "Bearer token123")

	invoked := false
	err := authClientInterceptor(ctx, "/test.Method", nil, nil, nil, func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
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
	ctx := context.Background()

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
