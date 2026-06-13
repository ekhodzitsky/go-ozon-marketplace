package domain

import (
	"time"

	"github.com/google/uuid"
)

type Order struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Items       []OrderItem
	TotalAmount int64
	Status      OrderStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type OrderItem struct {
	ID        uuid.UUID
	OrderID   uuid.UUID
	ProductID uuid.UUID
	Quantity  int
	Price     int64
}

type OutboxEvent struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
	CreatedAt     time.Time
	ProcessedAt   *time.Time
	RetryCount    int
	LastError     *string
	NextRetryAt   time.Time
}

type OutboxDLQEvent struct {
	ID            uuid.UUID
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
	CreatedAt     time.Time
	FailedAt      time.Time
	LastError     *string
	RetryCount    int
}
