package usecase

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/domain"
	"github.com/google/uuid"
)

// UserUsecase defines the user use-case boundary.
type UserUsecase interface {
	Register(ctx context.Context, email, password, name string) (uuid.UUID, error)
	Login(ctx context.Context, email, password string) (string, error)
	GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error)
}
