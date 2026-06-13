package domain

import (
	"time"

	"github.com/google/uuid"
)

type Product struct {
	ID             uuid.UUID
	Name           string
	Description    string
	Price          int64
	Categories     []string
	IdempotencyKey string
	CreatedAt      time.Time
}
