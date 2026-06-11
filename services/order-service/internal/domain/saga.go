package domain

import (
	"time"

	"github.com/google/uuid"
)

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

type SagaReservedItem struct {
	ProductID string `json:"product_id"`
	Quantity  int32  `json:"quantity"`
}

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
