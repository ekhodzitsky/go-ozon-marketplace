package domain

import "time"

type EventType string

const (
	EventTypeOrderCreated   EventType = "order_created"
	EventTypeOrderConfirmed EventType = "order_confirmed"
	EventTypeOrderCancelled EventType = "order_cancelled"
	EventTypePaymentSuccess EventType = "payment_success"
	EventTypeUserRegistered EventType = "user_registered"
)

type Event struct {
	EventType      EventType
	AggregateID    string
	Payload        string
	Amount         float64
	Currency       string
	OccurredAt     time.Time
	CreatedAt      time.Time
	AggregationKey string
}
