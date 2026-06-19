package grpc

import (
	"context"
	"fmt"

	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/usecase"
	"github.com/google/uuid"
)

type UserHandler struct {
	userv1.UnimplementedUserServiceServer
	usecase usecase.UserUsecase
}

func NewUserHandler(uc usecase.UserUsecase) userv1.UserServiceServer {
	return &UserHandler{usecase: uc}
}

func (h *UserHandler) Register(ctx context.Context, req *userv1.RegisterRequest) (*userv1.RegisterResponse, error) {
	id, err := h.usecase.Register(ctx, req.Email, req.Password, req.Name)
	if err != nil {
		return nil, apperrors.ToStatus(err)
	}
	return &userv1.RegisterResponse{UserId: id.String()}, nil
}

func (h *UserHandler) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.LoginResponse, error) {
	token, err := h.usecase.Login(ctx, req.Email, req.Password)
	if err != nil {
		return nil, apperrors.ToStatus(err)
	}
	return &userv1.LoginResponse{Token: token}, nil
}

func (h *UserHandler) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	userIDStr, ok := middleware.GetUserID(ctx)
	if !ok || userIDStr == "" {
		return nil, apperrors.ToStatus(apperrors.Wrap(apperrors.ErrInvalidCredentials, "unauthenticated", "missing user identity"))
	}

	id, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, apperrors.ToStatus(apperrors.Wrap(apperrors.ErrInvalidArgument, "invalid_argument", fmt.Sprintf("invalid user id: %v", err)))
	}

	user, err := h.usecase.GetUser(ctx, id)
	if err != nil {
		return nil, apperrors.ToStatus(err)
	}
	return &userv1.GetUserResponse{
		UserId:    user.ID.String(),
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}, nil
}
