package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/ratelimit"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	DefaultCallTimeout  = 5 * time.Second
	DefaultQueryTimeout = 3 * time.Second
)

type userUsecase struct {
	repo        repository.UserRepository
	jwtSecret   string
	callTimeout time.Duration
	rateLimiter ratelimit.RateLimiter
}

func NewUserUsecase(repo repository.UserRepository, jwtSecret string, callTimeout time.Duration, queryTimeout time.Duration, rateLimiter ratelimit.RateLimiter) UserUsecase {
	if callTimeout == 0 {
		callTimeout = DefaultCallTimeout
	}
	if queryTimeout == 0 {
		queryTimeout = DefaultQueryTimeout
	}
	if rateLimiter == nil {
		rateLimiter = &ratelimit.NoopLimiter{}
	}
	return &userUsecase{repo: repo, jwtSecret: jwtSecret, callTimeout: callTimeout, rateLimiter: rateLimiter}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (u *userUsecase) hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func (u *userUsecase) verifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func (u *userUsecase) issueToken(user *domain.User) (string, error) {
	now := time.Now()
	role := user.Role
	if role == "" {
		role = string(auth.RoleUser)
	}
	claims := auth.CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			Issuer:    "go-ozon-marketplace",
			Audience:  jwt.ClaimStrings{"api-gateway"},
			ID:        uuid.NewString(),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
		Role: role,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(u.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return tokenString, nil
}

func (u *userUsecase) Register(ctx context.Context, email, password, name string) (uuid.UUID, error) {
	email = normalizeEmail(email)
	if !u.rateLimiter.Allow(ctx, "register:"+email) {
		return uuid.Nil, apperrors.Wrap(apperrors.ErrFailedPrecondition, "failed_precondition", "rate limit exceeded")
	}

	hash, err := u.hashPassword(password)
	if err != nil {
		return uuid.Nil, err
	}

	user := &domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: hash,
		Name:         name,
		Role:         string(auth.RoleUser),
		CreatedAt:    time.Now().UTC(),
	}

	ctx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()
	if err := u.repo.Create(ctx, user); err != nil {
		return uuid.Nil, fmt.Errorf("create user: %w", err)
	}
	return user.ID, nil
}

func (u *userUsecase) Login(ctx context.Context, email, password string) (string, error) {
	email = normalizeEmail(email)
	if !u.rateLimiter.Allow(ctx, "login:"+email) {
		return "", apperrors.Wrap(apperrors.ErrFailedPrecondition, "failed_precondition", "rate limit exceeded")
	}

	ctx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()
	user, err := u.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return "", apperrors.ErrInvalidCredentials
		}
		return "", fmt.Errorf("lookup user: %w", err)
	}

	if !u.verifyPassword(user.PasswordHash, password) {
		return "", apperrors.ErrInvalidCredentials
	}

	return u.issueToken(user)
}

func (u *userUsecase) GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()
	return u.repo.GetByID(ctx, id)
}
