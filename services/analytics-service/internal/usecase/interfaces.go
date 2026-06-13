package usecase

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/domain"
)

// AnalyticsUsecase defines the analytics use-case boundary.
type AnalyticsUsecase interface {
	TrackEvent(ctx context.Context, eventType domain.EventType, aggregateID, payload, aggregationKey string, amount float64) error
	GetDailyRevenue(ctx context.Context, date string) (float64, error)
	TrackABTestEvent(ctx context.Context, event domain.ABTestEvent) error
	Flush(ctx context.Context) error
}

// EventRepository is the persistence boundary for analytics events.
type EventRepository interface {
	BatchInsert(ctx context.Context, events []domain.Event) error
	TrackABTestEvent(ctx context.Context, event domain.ABTestEvent) error
	GetDailyRevenue(ctx context.Context, date string) (float64, error)
}
