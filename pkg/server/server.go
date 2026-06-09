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
	s.Server.GracefulStop()
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
