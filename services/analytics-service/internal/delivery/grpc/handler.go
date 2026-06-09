package grpc

import (
	"context"

	analyticsv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/analytics/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/usecase"
)

type AnalyticsHandler struct {
	analyticsv1.UnimplementedAnalyticsServiceServer
	usecase *usecase.AnalyticsUsecase
}

func NewAnalyticsHandler(uc *usecase.AnalyticsUsecase) *AnalyticsHandler {
	return &AnalyticsHandler{usecase: uc}
}

func (h *AnalyticsHandler) TrackEvent(ctx context.Context, req *analyticsv1.TrackEventRequest) (*analyticsv1.TrackEventResponse, error) {
	if err := h.usecase.TrackEvent(ctx, domain.EventType(req.EventType), req.AggregateId, req.Payload); err != nil {
		return nil, err
	}
	return &analyticsv1.TrackEventResponse{Success: true}, nil
}

func (h *AnalyticsHandler) GetDailyRevenue(ctx context.Context, req *analyticsv1.GetDailyRevenueRequest) (*analyticsv1.GetDailyRevenueResponse, error) {
	revenue, err := h.usecase.GetDailyRevenue(ctx, req.Date)
	if err != nil {
		return nil, err
	}
	return &analyticsv1.GetDailyRevenueResponse{Revenue: revenue}, nil
}
