package auth

import "context"

// Identity — данные аутентифицированного вызывающего, которые таскаем через контекст запроса.
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

// Role — роль пользователя.
type Role string

const (
	RoleUser    Role = "user"
	RoleAdmin   Role = "admin"
	RoleService Role = "service"
)

// WithIdentity кладёт Identity в контекст.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, contextKeyIdentity, id)
}

// IdentityFromContext достаёт Identity из контекста.
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(contextKeyIdentity).(Identity)
	return id, ok
}
