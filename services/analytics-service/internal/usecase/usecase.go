package usecase

import (
	"context"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/repository/clickhouse"
)

type analyticsUsecase struct {
	repo         *clickhouse.EventRepo
	callTimeout  time.Duration
	queryTimeout time.Duration
}

func NewAnalyticsUsecase(repo *clickhouse.EventRepo, callTimeout time.Duration, queryTimeout time.Duration) AnalyticsUsecase {
	return &analyticsUsecase{repo: repo, callTimeout: callTimeout, queryTimeout: queryTimeout}
}

func (u *analyticsUsecase) TrackEvent(ctx context.Context, eventType domain.EventType, aggregateID, payload, aggregationKey string) error {
	event := domain.Event{
		EventType:      eventType,
		AggregateID:    aggregateID,
		Payload:        payload,
		AggregationKey: aggregationKey,
		CreatedAt:      time.Now().UTC(),
	}
	ctx, cancel := context.WithTimeout(ctx, u.callTimeout)
	defer cancel()
	return u.repo.Insert(ctx, event)
}

func (u *analyticsUsecase) GetDailyRevenue(ctx context.Context, date string) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, u.queryTimeout)
	defer cancel()
	return u.repo.GetDailyRevenue(ctx, date)
}
