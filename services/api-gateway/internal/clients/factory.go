package clients

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/circuitbreaker"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/tracing"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// Factory creates gRPC clients with common interceptors and TLS.
type Factory struct {
	cfg *config.Config
	cb  *circuitbreaker.CircuitBreaker
}

// NewFactory creates a new gRPC client factory.
func NewFactory(cfg *config.Config, cb *circuitbreaker.CircuitBreaker) *Factory {
	return &Factory{cfg: cfg, cb: cb}
}

// NewClient creates a gRPC client connection for the given address.
func (f *Factory) NewClient(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	creds, err := f.clientCreds(addr)
	if err != nil {
		return nil, fmt.Errorf("tls credentials: %w", err)
	}

	return grpc.NewClient(addr,
		grpc.WithTransportCredentials(creds),
		grpc.WithChainUnaryInterceptor(
			authClientInterceptor,
			circuitBreakerClientInterceptor(f.cb),
			tracing.UnaryClientInterceptor(),
		),
	)
}

func (f *Factory) clientCreds(addr string) (credentials.TransportCredentials, error) {
	if f.cfg.CertPath != "" {
		return server.LoadClientMTLSCredentials(
			filepath.Join(f.cfg.CertPath, "server-cert.pem"),
			filepath.Join(f.cfg.CertPath, "server-key.pem"),
			filepath.Join(f.cfg.CertPath, "ca-cert.pem"),
			serverNameFromAddr(addr),
		)
	}
	if f.cfg.InsecureSkipTLS {
		return insecure.NewCredentials(), nil
	}
	return nil, fmt.Errorf("no CERT_PATH configured and INSECURE_SKIP_TLS is false")
}

func serverNameFromAddr(addr string) string {
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

func authClientInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if authHeader, ok := ctx.Value(auth.ContextKeyAuthorizationHeader).(string); ok && authHeader != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", authHeader)
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}

func circuitBreakerClientInterceptor(cb *circuitbreaker.CircuitBreaker) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return cb.Call(func() error {
			return invoker(ctx, method, req, reply, cc, opts...)
		})
	}
}
