package domain

import (
	"time"

	"github.com/google/uuid"
)

type LedgerEntry struct {
	ID             uuid.UUID
	ProductID      uuid.UUID
	OrderID        *uuid.UUID
	QuantityChange int
	OperationType  string
	CreatedAt      time.Time
}
