package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/saga"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type SagaPostgres struct {
	db Querier
}

func NewSagaPostgres(db Querier) saga.SagaRepository {
	return &SagaPostgres{db: db}
}

func (r *SagaPostgres) WithTx(tx pgx.Tx) *SagaPostgres {
	return &SagaPostgres{db: tx}
}

func marshalReservedItems(items []saga.SagaReservedItem) []byte {
	if len(items) == 0 {
		return []byte("[]")
	}
	b, _ := json.Marshal(items)
	return b
}

func unmarshalReservedItems(data []byte, items *[]saga.SagaReservedItem) {
	if len(data) == 0 {
		*items = nil
		return
	}
	_ = json.Unmarshal(data, items)
}

func (r *SagaPostgres) Create(ctx context.Context, s *saga.Saga) error {
	query := `INSERT INTO sagas (id, order_id, status, current_step, error_message, payment_id, reserved_items, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := r.db.Exec(ctx, query, s.ID, s.OrderID, s.Status, s.CurrentStep, s.ErrorMessage, s.PaymentID, marshalReservedItems(s.ReservedItems), s.CreatedAt, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert saga: %w", err)
	}
	return nil
}

func (r *SagaPostgres) GetByOrderID(ctx context.Context, orderID uuid.UUID) (*saga.Saga, error) {
	query := `SELECT id, order_id, status, current_step, error_message, payment_id, reserved_items, created_at, updated_at FROM sagas WHERE order_id=$1`
	row := r.db.QueryRow(ctx, query, orderID)
	var s saga.Saga
	var reservedItems []byte
	if err := row.Scan(&s.ID, &s.OrderID, &s.Status, &s.CurrentStep, &s.ErrorMessage, &s.PaymentID, &reservedItems, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: saga", apperrors.ErrNotFound)
		}
		return nil, fmt.Errorf("get saga by order id: %w", err)
	}
	unmarshalReservedItems(reservedItems, &s.ReservedItems)
	return &s, nil
}

func (r *SagaPostgres) UpdateStatus(ctx context.Context, orderID uuid.UUID, status saga.SagaStatus, step string, errMsg string) error {
	query := `UPDATE sagas SET status=$1, current_step=$2, error_message=$3, updated_at=NOW() WHERE order_id=$4`
	tag, err := r.db.Exec(ctx, query, status, step, errMsg, orderID)
	if err != nil {
		return fmt.Errorf("update saga status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: saga", apperrors.ErrNotFound)
	}
	return nil
}

func (r *SagaPostgres) Save(ctx context.Context, s *saga.Saga) error {
	query := `UPDATE sagas SET status=$1, current_step=$2, error_message=$3, payment_id=$4, reserved_items=$5, updated_at=NOW() WHERE order_id=$6`
	tag, err := r.db.Exec(ctx, query, s.Status, s.CurrentStep, s.ErrorMessage, s.PaymentID, marshalReservedItems(s.ReservedItems), s.OrderID)
	if err != nil {
		return fmt.Errorf("save saga: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: saga", apperrors.ErrNotFound)
	}
	return nil
}

func (r *SagaPostgres) ListIncomplete(ctx context.Context, limit int) ([]saga.Saga, error) {
	query := `SELECT id, order_id, status, current_step, error_message, payment_id, reserved_items, created_at, updated_at FROM sagas WHERE status NOT IN ($1, $2, $3) ORDER BY created_at ASC LIMIT $4`
	rows, err := r.db.Query(ctx, query, saga.SagaStatusConfirmed, saga.SagaStatusCancelled, saga.SagaStatusFailed, limit)
	if err != nil {
		return nil, fmt.Errorf("list incomplete sagas: %w", err)
	}
	defer rows.Close()

	var sagas []saga.Saga
	for rows.Next() {
		var s saga.Saga
		var reservedItems []byte
		if err := rows.Scan(&s.ID, &s.OrderID, &s.Status, &s.CurrentStep, &s.ErrorMessage, &s.PaymentID, &reservedItems, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan saga: %w", err)
		}
		unmarshalReservedItems(reservedItems, &s.ReservedItems)
		sagas = append(sagas, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sagas: %w", err)
	}
	return sagas, nil
}
