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

func (r *PaymentPostgres) CreateRefund(ctx context.Context, refund *domain.Refund) error {
	query := `INSERT INTO refunds (id, payment_id, amount, reason, status, idempotency_key, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := r.db.Exec(ctx, query, refund.ID, refund.PaymentID, refund.Amount, refund.Reason, refund.Status, refund.IdempotencyKey, refund.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert refund: %w", err)
	}
	return nil
}

func (r *PaymentPostgres) GetRefund(ctx context.Context, id uuid.UUID) (*domain.Refund, error) {
	query := `SELECT id, payment_id, amount, reason, status, idempotency_key, created_at FROM refunds WHERE id=$1`
	row := r.db.QueryRow(ctx, query, id)
	var refund domain.Refund
	if err := row.Scan(&refund.ID, &refund.PaymentID, &refund.Amount, &refund.Reason, &refund.Status, &refund.IdempotencyKey, &refund.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: refund", apperrors.ErrNotFound)
		}
		return nil, fmt.Errorf("get refund by id: %w", err)
	}
	return &refund, nil
}

func (r *PaymentPostgres) GetRefundByIdempotencyKey(ctx context.Context, key string) (*domain.Refund, error) {
	query := `SELECT id, payment_id, amount, reason, status, idempotency_key, created_at FROM refunds WHERE idempotency_key=$1`
	row := r.db.QueryRow(ctx, query, key)
	var refund domain.Refund
	if err := row.Scan(&refund.ID, &refund.PaymentID, &refund.Amount, &refund.Reason, &refund.Status, &refund.IdempotencyKey, &refund.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: refund", apperrors.ErrNotFound)
		}
		return nil, fmt.Errorf("get refund by idempotency key: %w", err)
	}
	return &refund, nil
}

func (r *PaymentPostgres) ListRefunds(ctx context.Context, paymentID uuid.UUID) ([]*domain.Refund, error) {
	query := `SELECT id, payment_id, amount, reason, status, idempotency_key, created_at FROM refunds WHERE payment_id=$1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, paymentID)
	if err != nil {
		return nil, fmt.Errorf("list refunds: %w", err)
	}
	defer rows.Close()

	var refunds []*domain.Refund
	for rows.Next() {
		var refund domain.Refund
		if err := rows.Scan(&refund.ID, &refund.PaymentID, &refund.Amount, &refund.Reason, &refund.Status, &refund.IdempotencyKey, &refund.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan refund: %w", err)
		}
		refunds = append(refunds, &refund)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("refund rows err: %w", err)
	}
	return refunds, nil
}
