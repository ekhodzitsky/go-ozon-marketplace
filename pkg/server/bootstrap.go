package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const defaultShutdownTimeout = 10 * time.Second

// ServiceConfig configures a gRPC service server and its metrics sidecar.
type ServiceConfig struct {
	GRPCPort    int
	MetricsPort int
	CertPath    string
}

// RegisterFn registers service implementations on the provided gRPC server.
type RegisterFn func(*grpc.Server)

// StartService creates and starts a gRPC server plus a metrics HTTP server.
// It returns two shutdown functions that must be called on stop.
func StartService(cfg ServiceConfig, register RegisterFn, interceptors []grpc.UnaryServerInterceptor, log *zap.Logger) (*GRPCServer, *HTTPServer, error) {
	opts := []grpc.ServerOption{}
	if len(interceptors) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(interceptors...))
	}

	if cfg.CertPath != "" {
		tlsOpt, err := LoadServerMTLSCredentials(
			filepath.Join(cfg.CertPath, "server-cert.pem"),
			filepath.Join(cfg.CertPath, "server-key.pem"),
			filepath.Join(cfg.CertPath, "ca-cert.pem"),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("load tls credentials: %w", err)
		}
		opts = append(opts, tlsOpt)
		log.Info("tls enabled for gRPC server", zap.String("cert_path", cfg.CertPath))
	}

	grpcServer := NewGRPCWithLogger(cfg.GRPCPort, log, opts...)
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer.Server, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	register(grpcServer.Server)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	metricsServer := NewHTTPWithLogger(mux, cfg.MetricsPort, log)

	return grpcServer, metricsServer, nil
}

// RunService starts both servers and blocks until the context is cancelled or a fatal error occurs.
func RunService(ctx context.Context, cfg ServiceConfig, register RegisterFn, interceptors []grpc.UnaryServerInterceptor, log *zap.Logger) error {
	grpcServer, metricsServer, err := StartService(cfg, register, interceptors, log)
	if err != nil {
		return err
	}

	go func() {
		if err := metricsServer.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("metrics server error", zap.Error(err))
		}
	}()

	go func() {
		if err := grpcServer.Start(); err != nil {
			log.Fatal("grpc server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		log.Error("metrics server shutdown error", zap.Error(err))
	}
	grpcServer.GracefulStop()
	return nil
}
