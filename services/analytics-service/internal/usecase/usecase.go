package usecase

import (
	"context"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/domain"
	"go.uber.org/zap"
)

type analyticsUsecase struct {
	repo         EventRepository
	batcher      *eventBatcher
	callTimeout  time.Duration
	queryTimeout time.Duration
}

// NewAnalyticsUsecase creates the analytics use-case with buffered event inserts.
func NewAnalyticsUsecase(repo EventRepository, callTimeout, queryTimeout time.Duration, log *zap.Logger) AnalyticsUsecase {
	uc := &analyticsUsecase{
		repo:         repo,
		callTimeout:  callTimeout,
		queryTimeout: queryTimeout,
	}
	// Buffer up to 100 events or flush every second.
	uc.batcher = newEventBatcher(repo, log, 100, time.Second, callTimeout)
	return uc
}

func (u *analyticsUsecase) TrackEvent(ctx context.Context, eventType domain.EventType, aggregateID, payload, aggregationKey string, amount float64) error {
	event := domain.Event{
		EventType:      eventType,
		AggregateID:    aggregateID,
		Payload:        payload,
		Amount:         amount,
		Currency:       "",
		OccurredAt:     time.Now().UTC(),
		CreatedAt:      time.Now().UTC(),
		AggregationKey: aggregationKey,
	}
	return u.batcher.Add(ctx, event)
}

func (u *analyticsUsecase) GetDailyRevenue(ctx context.Context, date string) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()
	return u.repo.GetDailyRevenue(ctx, date)
}

func (u *analyticsUsecase) TrackABTestEvent(ctx context.Context, event domain.ABTestEvent) error {
	ctx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()
	return u.repo.TrackABTestEvent(ctx, event)
}

func (u *analyticsUsecase) Flush(ctx context.Context) error {
	return u.batcher.Stop(ctx)
}
