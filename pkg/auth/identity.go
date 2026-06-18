package auth

import "context"

// Identity carries authenticated caller information through the request graph.
type Identity struct {
	UserID              string
	Role                Role
	AuthorizationHeader string
}

type contextKey string

const (
	// ContextKeyUserID carries the authenticated user id in context.
	ContextKeyUserID contextKey = "user_id"
	// ContextKeyRole carries the authenticated user role in context.
	ContextKeyRole contextKey = "role"
	// ContextKeyAuthorizationHeader carries the raw Authorization header for gRPC forwarding.
	ContextKeyAuthorizationHeader contextKey = "authorization_header"

	contextKeyIdentity contextKey = "auth_identity"
)

// Role represents user roles.
type Role string

const (
	RoleUser    Role = "user"
	RoleAdmin   Role = "admin"
	RoleService Role = "service"
)

// WithIdentity injects an Identity into the context.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, contextKeyIdentity, id)
}

// IdentityFromContext extracts an Identity from the context.
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(contextKeyIdentity).(Identity)
	return id, ok
}
