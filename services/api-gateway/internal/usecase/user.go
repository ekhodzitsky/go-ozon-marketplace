package usecase

import (
	"context"
	"fmt"
	"time"

	userv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/user/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph/model"
)

func Register(ctx context.Context, client userv1.UserServiceClient, email, password, name string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := client.Register(ctx, &userv1.RegisterRequest{Email: email, Password: password, Name: name})
	if err != nil {
		return "", err
	}
	return resp.UserId, nil
}

func Login(ctx context.Context, client userv1.UserServiceClient, email, password string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := client.Login(ctx, &userv1.LoginRequest{Email: email, Password: password})
	if err != nil {
		return "", err
	}
	return resp.Token, nil
}

func Me(ctx context.Context, client userv1.UserServiceClient, timeout time.Duration) (*model.User, error) {
	if _, err := requireAuth(ctx); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := client.GetUser(ctx, &userv1.GetUserRequest{})
	if err != nil {
		return nil, err
	}
	return &model.User{ID: resp.UserId, Email: resp.Email, Name: resp.Name, CreatedAt: resp.CreatedAt}, nil
}

func GetUser(ctx context.Context, client userv1.UserServiceClient, id string, timeout time.Duration) (*model.User, error) {
	if err := requireOwnerOrAdmin(ctx, id); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := client.GetUser(ctx, &userv1.GetUserRequest{UserId: id})
	if err != nil {
		return nil, err
	}
	return &model.User{ID: resp.UserId, Email: resp.Email, Name: resp.Name, CreatedAt: resp.CreatedAt}, nil
}

func requireAuth(ctx context.Context) (string, error) {
	userID, ok := middleware.GetUserID(ctx)
	if !ok || userID == "" {
		return "", fmt.Errorf("unauthenticated")
	}
	return userID, nil
}
