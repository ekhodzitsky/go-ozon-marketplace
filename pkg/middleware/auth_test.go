package middleware

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGetRole(t *testing.T) {
	ctx := context.WithValue(context.Background(), ContextKeyRole, RoleAdmin)
	role, ok := GetRole(ctx)
	assert.True(t, ok)
	assert.Equal(t, RoleAdmin, role)

	_, ok = GetRole(context.Background())
	assert.False(t, ok)
}

func TestRequireRole(t *testing.T) {
	t.Run("allowed", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), ContextKeyRole, RoleUser)
		err := RequireRole(ctx, RoleUser, RoleAdmin)
		assert.NoError(t, err)
	})

	t.Run("denied", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), ContextKeyRole, RoleUser)
		err := RequireRole(ctx, RoleAdmin)
		assert.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.PermissionDenied, st.Code())
	})

	t.Run("missing role", func(t *testing.T) {
		err := RequireRole(context.Background(), RoleUser)
		assert.Error(t, err)
		st, _ := status.FromError(err)
		assert.Equal(t, codes.PermissionDenied, st.Code())
	})
}
