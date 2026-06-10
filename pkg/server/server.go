package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type GRPCServer struct {
	Server *grpc.Server
	Port   int
	log    *zap.Logger
}

func NewGRPC(port int, opts ...grpc.ServerOption) *GRPCServer {
	return &GRPCServer{
		Server: grpc.NewServer(opts...),
		Port:   port,
		log:    logger.New(),
	}
}

func (s *GRPCServer) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.Port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	s.log.Info("starting gRPC server", zap.Int("port", s.Port))
	return s.Server.Serve(lis)
}

func (s *GRPCServer) GracefulStop() {
	s.log.Info("stopping gRPC server gracefully")
	done := make(chan struct{})
	go func() {
		s.Server.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(25 * time.Second):
		s.log.Warn("force stopping gRPC server")
		s.Server.Stop()
	}
}

// LoadServerCredentials loads TLS certificate and key and returns a gRPC server option.
func LoadServerCredentials(certFile, keyFile string) (grpc.ServerOption, error) {
	creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load server tls credentials: %w", err)
	}
	return grpc.Creds(creds), nil
}

// LoadClientCredentials loads CA certificate and returns transport credentials for gRPC client connections.
func LoadClientCredentials(caFile string, serverName string) (credentials.TransportCredentials, error) {
	creds, err := credentials.NewClientTLSFromFile(caFile, serverName)
	if err != nil {
		return nil, fmt.Errorf("load client tls credentials: %w", err)
	}
	return creds, nil
}

type HTTPServer struct {
	Server *http.Server
	log    *zap.Logger
}

func NewHTTP(handler http.Handler, port int) *HTTPServer {
	return &HTTPServer{
		Server: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      handler,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
		},
		log: logger.New(),
	}
}

func (s *HTTPServer) Start() error {
	s.log.Info("starting HTTP server", zap.String("addr", s.Server.Addr))
	return s.Server.ListenAndServe()
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	s.log.Info("shutting down HTTP server")
	return s.Server.Shutdown(ctx)
}

func WaitShutdown(shutdown func()) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	shutdown()
}
