package domain

import (
	"github.com/google/uuid"
)

type Payment struct {
	ID      uuid.UUID
	OrderID uuid.UUID
	UserID  uuid.UUID
	Amount  float64
	Status  string
}

const (
	StatusPending  = "pending"
	StatusSuccess  = "success"
	StatusFailed   = "failed"
	StatusRefunded = "refunded"
)
