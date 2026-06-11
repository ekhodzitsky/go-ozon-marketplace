package domain

import (
	"github.com/google/uuid"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusSuccess  Status = "success"
	StatusFailed   Status = "failed"
	StatusRefunded Status = "refunded"
)

type Payment struct {
	ID      uuid.UUID
	OrderID uuid.UUID
	UserID  uuid.UUID
	Amount  int64
	Status  Status
}
