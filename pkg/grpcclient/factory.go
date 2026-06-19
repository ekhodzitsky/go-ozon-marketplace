package grpcclient

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/server"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/tracing"
	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// Factory creates gRPC client connections with TLS, service auth, circuit breaker and tracing.
type Factory struct {
	cfg             Config
	cb              *gobreaker.CircuitBreaker
	issuer          auth.Issuer
	insecureAllowed bool
	interceptors    []grpc.UnaryClientInterceptor
}

// Config holds factory configuration.
type Config struct {
	CertPath    string
	JWTSecret   string
	ServiceName string
}

// Option customizes a Factory.
type Option func(*Factory)

// WithUnaryInterceptor appends a user interceptor to the client chain.
func WithUnaryInterceptor(i grpc.UnaryClientInterceptor) Option {
	return func(f *Factory) { f.interceptors = append(f.interceptors, i) }
}

// WithInsecureAllowed controls whether insecure connections are permitted when no cert path is configured.
func WithInsecureAllowed(allowed bool) Option {
	return func(f *Factory) { f.insecureAllowed = allowed }
}

// NewFactory creates a client factory.
func NewFactory(cfg Config, cb *gobreaker.CircuitBreaker, opts ...Option) *Factory {
	f := &Factory{
		cfg:             cfg,
		cb:              cb,
		insecureAllowed: true,
	}
	if cfg.JWTSecret != "" && cfg.ServiceName != "" {
		f.issuer = auth.NewServiceTokenIssuer(cfg.JWTSecret, cfg.ServiceName, "api-gateway")
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// NewClient creates a connection to addr.
func (f *Factory) NewClient(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	creds, err := f.clientCreds(addr)
	if err != nil {
		return nil, err
	}

	interceptors := []grpc.UnaryClientInterceptor{
		tracing.UnaryClientInterceptor(),
	}
	if f.cb != nil {
		interceptors = append(interceptors, circuitBreakerInterceptor(f.cb))
	}
	interceptors = append(interceptors, f.interceptors...)
	if f.issuer != nil {
		interceptors = append(interceptors, middleware.ServiceAuthInterceptor(f.issuer))
	}

	return grpc.NewClient(addr,
		grpc.WithTransportCredentials(creds),
		grpc.WithChainUnaryInterceptor(interceptors...),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             20 * time.Second,
			PermitWithoutStream: true,
		}),
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
	if f.insecureAllowed {
		return insecure.NewCredentials(), nil
	}
	return nil, fmt.Errorf("no CERT_PATH configured and insecure connections are not allowed")
}

func serverNameFromAddr(addr string) string {
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

func circuitBreakerInterceptor(cb *gobreaker.CircuitBreaker) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		_, err := cb.Execute(func() (interface{}, error) {
			return nil, invoker(ctx, method, req, reply, cc, opts...)
		})
		return err
	}
}
