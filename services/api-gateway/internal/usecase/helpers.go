package usecase

import (
	"context"
	"fmt"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
)

func isAdmin(ctx context.Context) bool {
	role, _ := middleware.GetRole(ctx)
	return role == auth.RoleAdmin
}

func requireOwnerOrAdmin(ctx context.Context, ownerID string) error {
	userID, err := requireAuth(ctx)
	if err != nil {
		return err
	}
	if userID == ownerID || isAdmin(ctx) {
		return nil
	}
	return fmt.Errorf("forbidden")
}
