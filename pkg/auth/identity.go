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
	// ContextKeyUserID — ключ в контексте, под которым лежит id пользователя.
	ContextKeyUserID contextKey = "user_id"
	// ContextKeyRole — ключ в контексте, под которым лежит роль пользователя.
	ContextKeyRole contextKey = "role"

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
