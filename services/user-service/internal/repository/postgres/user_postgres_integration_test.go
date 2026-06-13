//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/domain"
	userpostgres "github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/repository/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	sharedDSN     string
	sharedPool    *pgxpool.Pool
	sharedOnce    sync.Once
	sharedCleanup func()
)

func startSharedPostgres(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	sharedOnce.Do(func() {
		ctx := context.Background()
		container, err := postgres.Run(ctx, "postgres:16-alpine",
			postgres.WithDatabase("marketplace"),
			postgres.WithUsername("ozon"),
			postgres.WithPassword("ozonpass"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
			),
		)
		require.NoError(t, err)

		dsn, err := container.ConnectionString(ctx, "sslmode=disable")
		require.NoError(t, err)
		sharedDSN = dsn

		pool, err := pgxpool.New(ctx, dsn)
		require.NoError(t, err)
		sharedPool = pool

		runMigrations(t, dsn)

		sharedCleanup = func() {
			pool.Close()
			_ = container.Terminate(ctx)
		}
	})
}

func runMigrations(t *testing.T, dsn string) {
	t.Helper()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	migrationDir := filepath.Join("..", "..", "..", "migrations")
	entries, err := os.ReadDir(migrationDir)
	require.NoError(t, err)

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	for _, file := range files {
		content, err := os.ReadFile(filepath.Join(migrationDir, file))
		require.NoError(t, err)

		tx, err := pool.Begin(ctx)
		require.NoError(t, err)

		_, err = tx.Exec(ctx, string(content))
		if err != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("failed to execute migration %s: %v", file, err)
		}
		require.NoError(t, tx.Commit(ctx))
	}
}

func TestMain(m *testing.M) {
	code := m.Run()
	if sharedCleanup != nil {
		sharedCleanup()
	}
	os.Exit(code)
}

func newTestRepository(t *testing.T) (*userpostgres.UserPostgres, string) {
	t.Helper()
	startSharedPostgres(t)
	return userpostgres.NewUserPostgres(sharedPool).(*userpostgres.UserPostgres), sharedDSN
}

func newUser(email string) *domain.User {
	return &domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: "hash",
		Name:         "Test User",
		Role:         "user",
		CreatedAt:    time.Now().UTC(),
	}
}

func TestUserPostgres_CreateAndGet(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()

	user := newUser("create.get@ozon.ru")
	err := repo.Create(ctx, user)
	require.NoError(t, err)

	byID, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, byID.ID)
	assert.Equal(t, user.Email, byID.Email)
	assert.Equal(t, user.Name, byID.Name)
	assert.Equal(t, user.Role, byID.Role)

	byEmail, err := repo.GetByEmail(ctx, user.Email)
	require.NoError(t, err)
	assert.Equal(t, user.ID, byEmail.ID)
}

func TestUserPostgres_Create_DuplicateEmail(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()

	user := newUser("duplicate@ozon.ru")
	require.NoError(t, repo.Create(ctx, user))

	dup := newUser("duplicate@ozon.ru")
	err := repo.Create(ctx, dup)
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrAlreadyExists))
}

func TestUserPostgres_GetByID_NotFound(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrNotFound))
}

func TestUserPostgres_GetByEmail_NotFound(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()

	_, err := repo.GetByEmail(ctx, "missing@ozon.ru")
	require.Error(t, err)
	assert.True(t, errors.Is(err, apperrors.ErrNotFound))
}

func TestUserPostgres_Create_ConcurrentUniqueConstraint(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()

	email := "concurrent@ozon.ru"
	created := make(chan bool, 2)
	errCh := make(chan error, 2)

	for i := 0; i < 2; i++ {
		go func() {
			user := newUser(email)
			err := repo.Create(ctx, user)
			if err == nil {
				created <- true
			} else {
				errCh <- err
			}
		}()
	}

	successCount := 0
	errorCount := 0
	for i := 0; i < 2; i++ {
		select {
		case <-created:
			successCount++
		case err := <-errCh:
			require.Error(t, err)
			assert.True(t, errors.Is(err, apperrors.ErrAlreadyExists))
			errorCount++
		case <-time.After(10 * time.Second):
			t.Fatal("timeout waiting for concurrent create results")
		}
	}

	assert.Equal(t, 1, successCount)
	assert.Equal(t, 1, errorCount)
}
