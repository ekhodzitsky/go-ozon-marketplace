package domain

import (
	"time"

	"github.com/google/uuid"
)

// ABTestEvent represents an A/B test tracking event.
type ABTestEvent struct {
	EventID      uuid.UUID
	Experiment   string
	Variation    string
	UserID       uuid.UUID
	Conversion   bool
	RevenueMinor int64
	CreatedAt    time.Time
}
