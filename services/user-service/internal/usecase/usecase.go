package usecase

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/domain"
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
	repo         repository.UserRepository
	jwtSecret    string
	callTimeout  time.Duration
	queryTimeout time.Duration
	rateLimiter  RateLimiter
}

// RateLimiter limits the rate of requests per key.
type RateLimiter interface {
	Allow(ctx context.Context, key string) bool
}

type noopRateLimiter struct{}

func (n *noopRateLimiter) Allow(ctx context.Context, key string) bool { return true }

// MemoryRateLimiter is a simple in-memory sliding-window rate limiter.
type MemoryRateLimiter struct {
	limit  int
	window time.Duration
	mu     sync.Mutex
	hits   map[string][]time.Time
}

// NewMemoryRateLimiter creates an in-memory rate limiter with the given limit per window.
func NewMemoryRateLimiter(limit int, window time.Duration) *MemoryRateLimiter {
	if limit <= 0 {
		limit = 10
	}
	if window <= 0 {
		window = time.Minute
	}
	return &MemoryRateLimiter{
		limit:  limit,
		window: window,
		hits:   make(map[string][]time.Time),
	}
}

func (r *MemoryRateLimiter) Allow(ctx context.Context, key string) bool {
	now := time.Now()
	cutoff := now.Add(-r.window)

	r.mu.Lock()
	defer r.mu.Unlock()

	var recent []time.Time
	for _, t := range r.hits[key] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= r.limit {
		r.hits[key] = recent
		return false
	}
	r.hits[key] = append(recent, now)
	return true
}

func NewUserUsecase(repo repository.UserRepository, jwtSecret string, callTimeout time.Duration, queryTimeout time.Duration, rateLimiter RateLimiter) UserUsecase {
	if callTimeout == 0 {
		callTimeout = DefaultCallTimeout
	}
	if queryTimeout == 0 {
		queryTimeout = DefaultQueryTimeout
	}
	if rateLimiter == nil {
		rateLimiter = &noopRateLimiter{}
	}
	return &userUsecase{repo: repo, jwtSecret: jwtSecret, callTimeout: callTimeout, queryTimeout: queryTimeout, rateLimiter: rateLimiter}
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (u *userUsecase) Register(ctx context.Context, email, password, name string) (uuid.UUID, error) {
	email = normalizeEmail(email)
	if !u.rateLimiter.Allow(ctx, "register:"+email) {
		return uuid.Nil, apperrors.Wrap(apperrors.ErrFailedPrecondition, "failed_precondition", "rate limit exceeded")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return uuid.Nil, fmt.Errorf("hash password: %w", err)
	}

	user := &domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: string(hash),
		Name:         name,
		Role:         string(middleware.RoleUser),
		CreatedAt:    time.Now().UTC(),
	}

	ctx, cancel := context.WithTimeout(ctx, u.queryTimeout)
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

	ctx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()
	user, err := u.repo.GetByEmail(ctx, email)
	if err != nil {
		return "", apperrors.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", apperrors.ErrInvalidCredentials
	}

	now := time.Now()
	role := user.Role
	if role == "" {
		role = string(middleware.RoleUser)
	}
	claims := middleware.CustomClaims{
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

func (u *userUsecase) GetUser(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()
	return u.repo.GetByID(ctx, id)
}
