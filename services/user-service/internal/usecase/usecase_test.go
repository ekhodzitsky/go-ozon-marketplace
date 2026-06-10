package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/domain"
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
		return assert.AnError
	}
	m.users[user.Email] = user
	m.byID[user.ID] = user
	return nil
}

func (m *mockUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	user, ok := m.byID[id]
	if !ok {
		return nil, assert.AnError
	}
	return user, nil
}

func (m *mockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	user, ok := m.users[email]
	if !ok {
		return nil, assert.AnError
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := newMockUserRepository()
			uc := NewUserUsecase(repo, "test-secret")

			// Pre-seed existing user for duplicate test
			if tt.name == "duplicate_email" {
				_ = repo.Create(context.Background(), &domain.User{
					ID:        uuid.New(),
					Email:     tt.email,
					Name:      "Existing",
					CreatedAt: time.Now().UTC(),
				})
			}

			id, err := uc.Register(context.Background(), tt.email, tt.password, "Test User")

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEqual(t, uuid.Nil, id)

			// Verify password was hashed
			user, _ := repo.GetByEmail(context.Background(), tt.email)
			require.NotNil(t, user)
			assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(tt.password)))
		})
	}
}

func TestUserUsecase_Login(t *testing.T) {
	t.Parallel()

	repo := newMockUserRepository()
	uc := NewUserUsecase(repo, "test-secret")

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
		},
		{
			name:     "user_not_found",
			email:    "nonexistent@ozon.ru",
			password: password,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			token, err := uc.Login(context.Background(), tt.email, tt.password)

			if tt.wantErr {
				require.Error(t, err)
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
	uc := NewUserUsecase(repo, "test-secret")

	id, err := uc.Register(context.Background(), "get@ozon.ru", "password", "Get User")
	require.NoError(t, err)

	user, err := uc.GetUser(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "get@ozon.ru", user.Email)
	assert.Equal(t, "Get User", user.Name)

	// Not found
	_, err = uc.GetUser(context.Background(), uuid.New())
	require.Error(t, err)
}
