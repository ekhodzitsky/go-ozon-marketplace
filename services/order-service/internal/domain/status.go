package domain

import (
	"fmt"
	"strings"
)

// OrderStatus represents the finite state machine states for an order.
type OrderStatus string

const (
	OrderStatusUnspecified     OrderStatus = "unspecified"
	OrderStatusPending         OrderStatus = "pending"
	OrderStatusAwaitingPayment OrderStatus = "awaiting_payment"
	OrderStatusPaid            OrderStatus = "paid"
	OrderStatusProcessing      OrderStatus = "processing"
	OrderStatusShipped         OrderStatus = "shipped"
	OrderStatusDelivered       OrderStatus = "delivered"
	OrderStatusCancelled       OrderStatus = "cancelled"
	OrderStatusRefunded        OrderStatus = "refunded"
)

// allowedTransitions defines the valid order status transitions.
var allowedTransitions = map[OrderStatus][]OrderStatus{
	OrderStatusPending:         {OrderStatusAwaitingPayment, OrderStatusCancelled},
	OrderStatusAwaitingPayment: {OrderStatusPaid, OrderStatusCancelled},
	OrderStatusPaid:            {OrderStatusProcessing, OrderStatusCancelled, OrderStatusRefunded},
	OrderStatusProcessing:      {OrderStatusShipped, OrderStatusCancelled, OrderStatusRefunded},
	OrderStatusShipped:         {OrderStatusDelivered, OrderStatusCancelled, OrderStatusRefunded},
	OrderStatusDelivered:       {OrderStatusRefunded},
}

// CanTransition reports whether the order can move from its current status to target.
func (s OrderStatus) CanTransition(target OrderStatus) bool {
	if s == target {
		return true
	}
	for _, next := range allowedTransitions[s] {
		if next == target {
			return true
		}
	}
	return false
}

// ValidateTransition returns an error if the transition is not allowed.
func (s OrderStatus) ValidateTransition(target OrderStatus) error {
	if !s.CanTransition(target) {
		return fmt.Errorf("invalid status transition from %s to %s", s, target)
	}
	return nil
}

// CancellableDirectly reports whether the order can be cancelled without compensation.
func (s OrderStatus) CancellableDirectly() bool {
	return s == OrderStatusPending || s == OrderStatusAwaitingPayment
}

// ParseOrderStatus converts a string to an OrderStatus (case-insensitive).
func ParseOrderStatus(status string) (OrderStatus, error) {
	switch strings.ToLower(status) {
	case "", "unspecified":
		return OrderStatusUnspecified, nil
	case "pending":
		return OrderStatusPending, nil
	case "awaiting_payment":
		return OrderStatusAwaitingPayment, nil
	case "paid":
		return OrderStatusPaid, nil
	case "processing":
		return OrderStatusProcessing, nil
	case "shipped":
		return OrderStatusShipped, nil
	case "delivered":
		return OrderStatusDelivered, nil
	case "cancelled":
		return OrderStatusCancelled, nil
	case "refunded":
		return OrderStatusRefunded, nil
	default:
		return "", fmt.Errorf("unknown order status: %s", status)
	}
}
