package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/user-service/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserPostgres struct {
	db           *pgxpool.Pool
	queryTimeout time.Duration
}

func NewUserPostgres(db *pgxpool.Pool) repository.UserRepository {
	return &UserPostgres{db: db, queryTimeout: repository.DefaultQueryTimeout}
}

func (r *UserPostgres) Create(ctx context.Context, user *domain.User) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	query := `INSERT INTO users (id, email, password_hash, name, role, created_at) VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.Exec(ctx, query, user.ID, user.Email, user.PasswordHash, user.Name, user.Role, user.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return fmt.Errorf("%w: user with email %s", apperrors.ErrAlreadyExists, user.Email)
		}
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (r *UserPostgres) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	query := `SELECT id, email, password_hash, name, role, created_at FROM users WHERE id=$1`
	row := r.db.QueryRow(ctx, query, id)
	var user domain.User
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: user", apperrors.ErrNotFound)
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &user, nil
}

func (r *UserPostgres) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	query := `SELECT id, email, password_hash, name, role, created_at FROM users WHERE email=$1`
	row := r.db.QueryRow(ctx, query, email)
	var user domain.User
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: user", apperrors.ErrNotFound)
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &user, nil
}
