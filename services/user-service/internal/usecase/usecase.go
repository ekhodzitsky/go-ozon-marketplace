package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserUsecase struct {
	repo      repository.UserRepository
	jwtSecret string
}

func NewUserUsecase(repo repository.UserRepository, jwtSecret string) *UserUsecase {
	return &UserUsecase{repo: repo, jwtSecret: jwtSecret}
}

func (u *UserUsecase) Register(ctx context.Context, email, password, name string) (uuid.UUID, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return uuid.Nil, fmt.Errorf("hash password: %w", err)
	}

	user := &domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: string(hash),
		Name:         name,
		CreatedAt:    time.Now().UTC(),
	}

	if err := u.repo.Create(ctx, user); err != nil {
		return uuid.Nil, fmt.Errorf("create user: %w", err)
	}
	return user.ID, nil
}

func (u *UserUsecase) Login(ctx context.Context, email, password string) (string, error) {
	user, err := u.repo.GetByEmail(ctx, email)
	if err != nil {
		return "", fmt.Errorf("user not found: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", fmt.Errorf("invalid password")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID.String(),
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(u.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return tokenString, nil
}

func (u *UserUsecase) GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return u.repo.GetByID(ctx, id)
}
