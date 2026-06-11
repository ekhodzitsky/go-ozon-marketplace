package domain

import "time"

type EventType string

const (
	EventTypeOrderCreated   EventType = "order_created"
	EventTypePaymentSuccess EventType = "payment_success"
	EventTypeUserRegistered EventType = "user_registered"
)

type Event struct {
	EventType      EventType
	AggregateID    string
	Payload        string
	CreatedAt      time.Time
	AggregationKey string
}
