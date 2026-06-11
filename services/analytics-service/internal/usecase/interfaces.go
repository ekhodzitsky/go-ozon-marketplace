package usecase

import (
	"context"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/domain"
)

// AnalyticsUsecase defines the analytics use-case boundary.
type AnalyticsUsecase interface {
	TrackEvent(ctx context.Context, eventType domain.EventType, aggregateID, payload, aggregationKey string) error
	GetDailyRevenue(ctx context.Context, date string) (float64, error)
	TrackABTestEvent(ctx context.Context, event domain.ABTestEvent) error
}
