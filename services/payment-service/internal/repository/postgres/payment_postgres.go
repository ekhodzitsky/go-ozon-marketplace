package postgres

import (
	"context"
	"fmt"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PaymentPostgres struct {
	db *pgxpool.Pool
}

func NewPaymentPostgres(db *pgxpool.Pool) repository.PaymentRepository {
	return &PaymentPostgres{db: db}
}

func (r *PaymentPostgres) Create(ctx context.Context, payment *domain.Payment) error {
	query := `INSERT INTO payments (id, order_id, user_id, amount, status) VALUES ($1, $2, $3, $4, $5)`
	_, err := r.db.Exec(ctx, query, payment.ID, payment.OrderID, payment.UserID, payment.Amount, payment.Status)
	if err != nil {
		return fmt.Errorf("insert payment: %w", err)
	}
	return nil
}

func (r *PaymentPostgres) GetByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	query := `SELECT id, order_id, user_id, amount, status FROM payments WHERE id=$1`
	row := r.db.QueryRow(ctx, query, id)
	var payment domain.Payment
	if err := row.Scan(&payment.ID, &payment.OrderID, &payment.UserID, &payment.Amount, &payment.Status); err != nil {
		return nil, fmt.Errorf("get payment by id: %w", err)
	}
	return &payment, nil
}

func (r *PaymentPostgres) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	query := `UPDATE payments SET status=$1 WHERE id=$2`
	_, err := r.db.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("update payment status: %w", err)
	}
	return nil
}
