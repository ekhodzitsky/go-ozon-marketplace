package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
)

type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type PaymentPostgres struct {
	db Querier
}

func NewPaymentPostgres(db Querier) repository.PaymentRepository {
	return &PaymentPostgres{db: db}
}

func (r *PaymentPostgres) WithTx(tx pgx.Tx) repository.PaymentRepository {
	return &PaymentPostgres{db: tx}
}

func (r *PaymentPostgres) Create(ctx context.Context, payment *domain.Payment) error {
	query := `INSERT INTO payments (id, order_id, user_id, amount, status) VALUES ($1, $2, $3, $4, $5)`
	_, err := r.db.Exec(ctx, query, payment.ID, payment.OrderID, payment.UserID, payment.Amount, payment.Status)
	if err != nil {
		return fmt.Errorf("insert payment: %w", err)
	}
	return nil
}

func (r *PaymentPostgres) CreateOrGet(ctx context.Context, payment *domain.Payment) (*domain.Payment, error) {
	query := `
		INSERT INTO payments (id, order_id, user_id, amount, status)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (order_id) DO NOTHING
		RETURNING id, order_id, user_id, amount, status
	`
	row := r.db.QueryRow(ctx, query, payment.ID, payment.OrderID, payment.UserID, payment.Amount, payment.Status)
	var p domain.Payment
	if err := row.Scan(&p.ID, &p.OrderID, &p.UserID, &p.Amount, &p.Status); err == nil {
		return &p, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("insert payment: %w", err)
	}

	query = `SELECT id, order_id, user_id, amount, status FROM payments WHERE order_id=$1 FOR UPDATE`
	row = r.db.QueryRow(ctx, query, payment.OrderID)
	if err := row.Scan(&p.ID, &p.OrderID, &p.UserID, &p.Amount, &p.Status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: payment", apperrors.ErrNotFound)
		}
		return nil, fmt.Errorf("get payment by order id: %w", err)
	}
	return &p, nil
}

func (r *PaymentPostgres) GetByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	query := `SELECT id, order_id, user_id, amount, status FROM payments WHERE id=$1`
	row := r.db.QueryRow(ctx, query, id)
	var payment domain.Payment
	if err := row.Scan(&payment.ID, &payment.OrderID, &payment.UserID, &payment.Amount, &payment.Status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: payment", apperrors.ErrNotFound)
		}
		return nil, fmt.Errorf("get payment by id: %w", err)
	}
	return &payment, nil
}

func (r *PaymentPostgres) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status) error {
	query := `UPDATE payments SET status=$1 WHERE id=$2`
	_, err := r.db.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("update payment status: %w", err)
	}
	return nil
}

func (r *PaymentPostgres) UpdateStatusIf(ctx context.Context, id uuid.UUID, newStatus, expectedStatus domain.Status) (bool, error) {
	query := `UPDATE payments SET status=$1 WHERE id=$2 AND status=$3`
	tag, err := r.db.Exec(ctx, query, newStatus, id, expectedStatus)
	if err != nil {
		return false, fmt.Errorf("update payment status: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
