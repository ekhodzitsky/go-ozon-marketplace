package middleware

import (
	"context"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

var (
	grpcRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_requests_total",
		Help: "Total gRPC requests",
	}, []string{"method", "status"})
	grpcRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "grpc_request_duration_seconds",
		Help: "gRPC request duration",
	}, []string{"method"})
)

func init() {
	prometheus.MustRegister(grpcRequestsTotal, grpcRequestDuration)
}

// LoggingUnaryInterceptor — дефолтный логирующий интерцептор, использует логгер по умолчанию.
func LoggingUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	return NewLoggingInterceptor(nil)(ctx, req, info, handler)
}

// NewLoggingInterceptor возвращает gRPC unary интерцептор, который логирует запросы заданным логгером.
func NewLoggingInterceptor(log *zap.Logger) grpc.UnaryServerInterceptor {
	if log == nil {
		log = defaultGRPCLog()
	}
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		st, _ := status.FromError(err)
		log.Info("gRPC request",
			zap.String("method", info.FullMethod),
			zap.Duration("duration", time.Since(start)),
			zap.String("code", st.Code().String()),
		)

		return resp, err
	}
}

func defaultGRPCLog() *zap.Logger {
	log, _ := logger.New("info", "json")
	return log
}

func MetricsUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	st, _ := status.FromError(err)
	grpcRequestDuration.WithLabelValues(info.FullMethod).Observe(time.Since(start).Seconds())
	grpcRequestsTotal.WithLabelValues(info.FullMethod, st.Code().String()).Inc()
	return resp, err
}
