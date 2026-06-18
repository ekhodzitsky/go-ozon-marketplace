package clients

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// authForwardingInterceptor forwards the caller's Authorization header from the incoming request
// context into outgoing gRPC metadata. It prefers the structured auth.Identity and falls back to
// the legacy context key for compatibility with existing tests and resolvers.
func authForwardingInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if id, ok := auth.IdentityFromContext(ctx); ok && id.AuthorizationHeader != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", id.AuthorizationHeader)
	} else if authHeader, ok := ctx.Value(auth.ContextKeyAuthorizationHeader).(string); ok && authHeader != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", authHeader)
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}
