package middleware

import (
	"context"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func LoggingUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	start := time.Now()
	log := logger.New()

	resp, err := handler(ctx, req)

	st, _ := status.FromError(err)
	log.Info("gRPC request",
		zap.String("method", info.FullMethod),
		zap.Duration("duration", time.Since(start)),
		zap.String("code", st.Code().String()),
	)

	return resp, err
}
