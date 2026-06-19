package middleware

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// ServiceAuthInterceptor цепляет свежий сервисный токен к исходящим gRPC-вызовам.
func ServiceAuthInterceptor(issuer auth.Issuer) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		token, err := issuer.Issue(ctx)
		if err != nil {
			return err
		}
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", token)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
