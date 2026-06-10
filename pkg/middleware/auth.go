package middleware

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const ContextKeyUserID contextKey = "user_id"

// AuthUnaryInterceptor validates JWT bearer token from gRPC metadata
func AuthUnaryInterceptor(jwtSecret string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Skip auth for public endpoints (register, login, health checks)
		if isPublicEndpoint(info.FullMethod) {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization header")
		}

		tokenStr := strings.TrimPrefix(authHeader[0], "Bearer ")
		if tokenStr == authHeader[0] {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization header format")
		}

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "invalid token claims")
		}

		userID, ok := claims["user_id"].(string)
		if !ok || userID == "" {
			return nil, status.Error(codes.Unauthenticated, "missing user_id in token")
		}

		ctx = context.WithValue(ctx, ContextKeyUserID, userID)
		return handler(ctx, req)
	}
}

func isPublicEndpoint(method string) bool {
	public := []string{
		"/user.v1.UserService/Register",
		"/user.v1.UserService/Login",
		"/grpc.health.v1.Health/Check",
	}
	for _, p := range public {
		if method == p {
			return true
		}
	}
	return false
}

// GetUserID extracts user_id from context
func GetUserID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ContextKeyUserID).(string)
	return v, ok
}
