package domain

import (
	"time"

	"github.com/google/uuid"
)

type Refund struct {
	ID        uuid.UUID
	PaymentID uuid.UUID
	Amount    int64
	Reason    string
	Status    string
	CreatedAt time.Time
}
