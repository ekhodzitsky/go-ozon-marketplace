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

// Factory создаёт gRPC-соединения с TLS, сервисной авторизацией, circuit breaker и трассировкой.
type Factory struct {
	cfg             Config
	cb              *gobreaker.CircuitBreaker
	issuer          auth.Issuer
	insecureAllowed bool
	interceptors    []grpc.UnaryClientInterceptor
}

// Config — настройки фабрики.
type Config struct {
	CertPath        string
	JWTSecret       string
	ServiceName     string
	UserAuth        bool
	InsecureSkipTLS bool
}

// Option настраивает Factory.
type Option func(*Factory)

// WithUnaryInterceptor добавляет пользовательский интерцептор в цепочку клиента.
func WithUnaryInterceptor(i grpc.UnaryClientInterceptor) Option {
	return func(f *Factory) { f.interceptors = append(f.interceptors, i) }
}

// WithInsecureAllowed разрешает или запрещает незащищённые соединения, если не настроен cert path.
func WithInsecureAllowed(allowed bool) Option {
	return func(f *Factory) { f.insecureAllowed = allowed }
}

// NewFactory создаёт фабрику клиентов.
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

// NewClient создаёт соединение с addr.
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
	if f.cfg.UserAuth {
		interceptors = append(interceptors, auth.UserAuthInterceptor())
	} else if f.issuer != nil {
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
	if f.cfg.InsecureSkipTLS || f.insecureAllowed {
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
