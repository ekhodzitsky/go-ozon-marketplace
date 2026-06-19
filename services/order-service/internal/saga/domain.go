package saga

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SagaStatus — статусы, через которые проходит сага заказа.
type SagaStatus string

const (
	SagaStatusPending      SagaStatus = "pending"
	SagaStatusReserving    SagaStatus = "reserving"
	SagaStatusReserved     SagaStatus = "reserved"
	SagaStatusPaying       SagaStatus = "paying"
	SagaStatusPaid         SagaStatus = "paid"
	SagaStatusConfirming   SagaStatus = "confirming"
	SagaStatusConfirmed    SagaStatus = "confirmed"
	SagaStatusCompensating SagaStatus = "compensating"
	SagaStatusCancelled    SagaStatus = "cancelled"
	SagaStatusFailed       SagaStatus = "failed"
)

// SagaReservedItem — один зарезервированный товар, нужен для компенсации.
type SagaReservedItem struct {
	ProductID string `json:"product_id"`
	Quantity  int32  `json:"quantity"`
}

// Saga — состояние саги по заказу.
type Saga struct {
	ID            uuid.UUID
	OrderID       uuid.UUID
	Status        SagaStatus
	CurrentStep   string
	ErrorMessage  string
	PaymentID     string
	ReservedItems []SagaReservedItem
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Locker — распределенный замок, чтобы recovery worker работал в одном инстансе.
type Locker interface {
	TryLock(ctx context.Context, key string) (bool, error)
	Unlock(ctx context.Context, key string) error
}

// Recoverer — тот, кто умеет доигрывать незавершенные саги.
type Recoverer interface {
	Recover(ctx context.Context) error
}

// SagaRepository — хранилище состояния саги.
type SagaRepository interface {
	Create(ctx context.Context, s *Saga) error
	GetByOrderID(ctx context.Context, orderID uuid.UUID) (*Saga, error)
	UpdateStatus(ctx context.Context, orderID uuid.UUID, status SagaStatus, step string, errMsg string) error
	Save(ctx context.Context, s *Saga) error
	ListIncomplete(ctx context.Context, limit int) ([]Saga, error)
}
