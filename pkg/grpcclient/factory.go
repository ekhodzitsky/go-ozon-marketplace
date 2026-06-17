package grpcclient

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
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
	cfg    Config
	cb     *gobreaker.CircuitBreaker
	issuer *auth.ServiceTokenIssuer
}

// Config holds factory configuration.
type Config struct {
	CertPath    string
	JWTSecret   string
	ServiceName string
}

// NewFactory creates a client factory.
func NewFactory(cfg Config, cb *gobreaker.CircuitBreaker) *Factory {
	var issuer *auth.ServiceTokenIssuer
	if cfg.JWTSecret != "" && cfg.ServiceName != "" {
		issuer = auth.NewServiceTokenIssuer(cfg.JWTSecret, cfg.ServiceName, "api-gateway")
	}
	return &Factory{cfg: cfg, cb: cb, issuer: issuer}
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
	if f.issuer != nil {
		interceptors = append(interceptors, auth.ServiceAuthInterceptor(f.issuer))
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
	return insecure.NewCredentials(), nil
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
