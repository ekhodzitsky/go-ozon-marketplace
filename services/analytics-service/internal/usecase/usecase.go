package usecase

import (
	"context"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/repository/clickhouse"
)

type AnalyticsUsecase struct {
	repo *clickhouse.EventRepo
}

func NewAnalyticsUsecase(repo *clickhouse.EventRepo) *AnalyticsUsecase {
	return &AnalyticsUsecase{repo: repo}
}

func (u *AnalyticsUsecase) TrackEvent(ctx context.Context, eventType domain.EventType, aggregateID, payload string) error {
	event := domain.Event{
		EventType:   eventType,
		AggregateID: aggregateID,
		Payload:     payload,
		CreatedAt:   time.Now().UTC(),
	}
	return u.repo.Insert(ctx, event)
}

func (u *AnalyticsUsecase) GetDailyRevenue(ctx context.Context, date string) (float64, error) {
	return u.repo.GetDailyRevenue(ctx, date)
}
