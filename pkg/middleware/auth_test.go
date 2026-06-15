package middleware

import (
	"context"
	"testing"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetRole(t *testing.T) {
	ctx := context.WithValue(context.Background(), auth.ContextKeyRole, auth.RoleAdmin)
	role, ok := GetRole(ctx)
	assert.True(t, ok)
	assert.Equal(t, auth.RoleAdmin, role)

	_, ok = GetRole(context.Background())
	assert.False(t, ok)
}

func TestRequireRole(t *testing.T) {
	t.Run("allowed", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), auth.ContextKeyRole, auth.RoleUser)
		err := RequireRole(ctx, auth.RoleUser, auth.RoleAdmin)
		assert.NoError(t, err)
	})

	t.Run("denied", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), auth.ContextKeyRole, auth.RoleUser)
		err := RequireRole(ctx, auth.RoleAdmin)
		assert.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.PermissionDenied, st.Code())
	})

	t.Run("missing role", func(t *testing.T) {
		err := RequireRole(context.Background(), auth.RoleUser)
		assert.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.PermissionDenied, st.Code())
	})
}
