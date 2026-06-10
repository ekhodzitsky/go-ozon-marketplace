package grpc

import (
	"context"
	"net/http"

	"google.golang.org/grpc/metadata"
)

// AppendAuthFromHTTP extracts Authorization header from HTTP request and adds to gRPC outgoing metadata
func AppendAuthFromHTTP(ctx context.Context, req *http.Request) context.Context {
	auth := req.Header.Get("Authorization")
	if auth != "" {
		return metadata.AppendToOutgoingContext(ctx, "authorization", auth)
	}
	return ctx
}
