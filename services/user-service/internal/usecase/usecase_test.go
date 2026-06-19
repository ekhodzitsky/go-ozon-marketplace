package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/ratelimit"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// mockUserRepository is a test double for UserRepository
type mockUserRepository struct {
	users map[string]*domain.User // email -> user
	byID  map[uuid.UUID]*domain.User
}

func newMockUserRepository() *mockUserRepository {
	return &mockUserRepository{
		users: make(map[string]*domain.User),
		byID:  make(map[uuid.UUID]*domain.User),
	}
}

func (m *mockUserRepository) Create(ctx context.Context, user *domain.User) error {
	if _, exists := m.users[user.Email]; exists {
		return apperrors.ErrAlreadyExists
	}
	m.users[user.Email] = user
	m.byID[user.ID] = user
	return nil
}

func (m *mockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	user, ok := m.byID[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return user, nil
}

func (m *mockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	user, ok := m.users[email]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return user, nil
}

func TestUserUsecase_Register(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		email    string
		password string
		wantErr  bool
		errIs    error
	}{
		{
			name:     "success",
			email:    "test@ozon.ru",
			password: "securepassword123",
			wantErr:  false,
		},
		{
			name:     "duplicate_email",
			email:    "existing@ozon.ru",
			password: "securepassword123",
			wantErr:  true,
			errIs:    apperrors.ErrAlreadyExists,
		},
		{
			name:     "email_normalized",
			email:    "  TEST NormalizeD@Ozon.RU  ",
			password: "securepassword123",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := newMockUserRepository()
			uc := NewUserUsecase(repo, "test-secret", time.Second, time.Second, nil)

			// Pre-seed existing user for duplicate test
			if tt.name == "duplicate_email" {
				_ = repo.Create(context.Background(), &domain.User{
					ID:        uuid.New(),
					Email:     "existing@ozon.ru",
					Name:      "Existing",
					CreatedAt: time.Now().UTC(),
				})
			}

			id, err := uc.Register(context.Background(), tt.email, tt.password, "Test User")

			if tt.wantErr {
				require.Error(t, err)
				if tt.errIs != nil {
					assert.True(t, errors.Is(err, tt.errIs), "expected error to wrap %v, got %v", tt.errIs, err)
				}
				return
			}

			require.NoError(t, err)
			assert.NotEqual(t, uuid.Nil, id)

			// Verify email was normalized
			expectedEmail := normalizeEmail(tt.email)
			user, _ := repo.GetByEmail(context.Background(), expectedEmail)
			require.NotNil(t, user)
			assert.Equal(t, expectedEmail, user.Email)

			// Verify password was hashed
			assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(tt.password)))
		})
	}
}

func TestUserUsecase_Login(t *testing.T) {
	t.Parallel()

	repo := newMockUserRepository()
	uc := NewUserUsecase(repo, "test-secret", time.Second, time.Second, nil)

	// Register a user first
	email := "login@ozon.ru"
	password := "mypassword"
	_, err := uc.Register(context.Background(), email, password, "Login User")
	require.NoError(t, err)

	tests := []struct {
		name     string
		email    string
		password string
		wantErr  bool
		errIs    error
	}{
		{
			name:     "success",
			email:    email,
			password: password,
			wantErr:  false,
		},
		{
			name:     "wrong_password",
			email:    email,
			password: "wrongpassword",
			wantErr:  true,
			errIs:    apperrors.ErrInvalidCredentials,
		},
		{
			name:     "user_not_found",
			email:    "nonexistent@ozon.ru",
			password: password,
			wantErr:  true,
			errIs:    apperrors.ErrInvalidCredentials,
		},
		{
			name:     "email_normalized",
			email:    "  LOGIN@OZON.RU  ",
			password: password,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			token, err := uc.Login(context.Background(), tt.email, tt.password)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errIs != nil {
					assert.True(t, errors.Is(err, tt.errIs), "expected error to wrap %v, got %v", tt.errIs, err)
				}
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, token)
		})
	}
}

func TestUserUsecase_GetUser(t *testing.T) {
	t.Parallel()

	repo := newMockUserRepository()
	uc := NewUserUsecase(repo, "test-secret", time.Second, time.Second, nil)

	id, err := uc.Register(context.Background(), "get@ozon.ru", "password", "Get User")
	require.NoError(t, err)

	user, err := uc.GetUser(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "get@ozon.ru", user.Email)
	assert.Equal(t, "Get User", user.Name)

	// Not found
	_, err = uc.GetUser(context.Background(), uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrNotFound))
}

func TestUserUsecase_RateLimit(t *testing.T) {
	t.Parallel()

	repo := newMockUserRepository()
	// Very strict limiter: 2 requests per hour per key.
	rl := ratelimit.NewMemoryRateLimiter(2, time.Hour)
	uc := NewUserUsecase(repo, "test-secret", time.Second, time.Second, rl)

	email := "ratelimit@ozon.ru"
	password := "password123"
	_, err := uc.Register(context.Background(), email, password, "Rate Limited")
	require.NoError(t, err)

	// Two login attempts allowed.
	_, err = uc.Login(context.Background(), email, password)
	require.NoError(t, err)
	_, err = uc.Login(context.Background(), email, password)
	require.NoError(t, err)
	// Third should be rate limited.
	_, err = uc.Login(context.Background(), email, password)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrFailedPrecondition))
}

func TestUserUsecase_DefaultTimeoutsAndRateLimiter(t *testing.T) {
	t.Parallel()

	repo := newMockUserRepository()
	uc := NewUserUsecase(repo, "test-secret", 0, 0, nil)

	assert.Equal(t, DefaultCallTimeout, uc.(*userUsecase).callTimeout)
	assert.NotNil(t, uc.(*userUsecase).rateLimiter)
}

func TestUserUsecase_Login_NormalizesEmailBeforeLookup(t *testing.T) {
	t.Parallel()

	repo := newMockUserRepository()
	uc := NewUserUsecase(repo, "test-secret", time.Second, time.Second, nil)

	_, err := uc.Register(context.Background(), "Mixed@Ozon.RU", "password123", "User")
	require.NoError(t, err)

	// Login with different casing should find the normalized stored email.
	token, err := uc.Login(context.Background(), "MIXED@OZON.RU", "password123")
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestUserUsecase_Register_DuplicateAfterNormalization(t *testing.T) {
	t.Parallel()

	repo := newMockUserRepository()
	uc := NewUserUsecase(repo, "test-secret", time.Second, time.Second, nil)

	_, err := uc.Register(context.Background(), "Duplicate@Ozon.RU", "password123", "User")
	require.NoError(t, err)

	_, err = uc.Register(context.Background(), "  duplicate@ozon.ru  ", "password123", "User")
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrAlreadyExists))
}

func TestUserUsecase_Login_EmptyRoleDefaultsToUser(t *testing.T) {
	t.Parallel()

	repo := newMockUserRepository()
	uc := NewUserUsecase(repo, "test-secret", time.Second, time.Second, nil)

	email := "role@ozon.ru"
	password := "password123"
	id, err := uc.Register(context.Background(), email, password, "Role User")
	require.NoError(t, err)

	// Simulate a legacy user with an empty role in storage.
	user, ok := repo.byID[id]
	require.True(t, ok)
	user.Role = ""

	token, err := uc.Login(context.Background(), email, password)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestUserUsecase_Register_RateLimited(t *testing.T) {
	t.Parallel()

	repo := newMockUserRepository()
	rl := ratelimit.NewMemoryRateLimiter(1, time.Hour)
	uc := NewUserUsecase(repo, "test-secret", time.Second, time.Second, rl)

	email := "register-limit@ozon.ru"
	_, err := uc.Register(context.Background(), email, "password123", "User")
	require.NoError(t, err)

	_, err = uc.Register(context.Background(), email, "password123", "User")
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrFailedPrecondition))
}

func TestUserUsecase_GetUser_NotFound(t *testing.T) {
	t.Parallel()

	repo := newMockUserRepository()
	uc := NewUserUsecase(repo, "test-secret", time.Second, time.Second, nil)

	_, err := uc.GetUser(context.Background(), uuid.Nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrNotFound))
}

func TestUserUsecase_Register_TrimsAndLowercasesEmail(t *testing.T) {
	t.Parallel()

	repo := newMockUserRepository()
	uc := NewUserUsecase(repo, "test-secret", time.Second, time.Second, nil)

	id, err := uc.Register(context.Background(), "  Spaced@Ozon.RU  ", "password123", "User")
	require.NoError(t, err)

	user, err := uc.GetUser(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "spaced@ozon.ru", user.Email)
}
