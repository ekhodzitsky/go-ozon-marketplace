package grpc

import (
	"context"
	"time"

	analyticsv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/analytics/v1"
	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/usecase"
	"github.com/google/uuid"
)

type AnalyticsHandler struct {
	analyticsv1.UnimplementedAnalyticsServiceServer
	usecase usecase.AnalyticsUsecase
}

func NewAnalyticsHandler(uc usecase.AnalyticsUsecase) *AnalyticsHandler {
	return &AnalyticsHandler{usecase: uc}
}

func (h *AnalyticsHandler) TrackEvent(ctx context.Context, req *analyticsv1.TrackEventRequest) (*analyticsv1.TrackEventResponse, error) {
	if err := h.usecase.TrackEvent(ctx, domain.EventType(req.EventType), req.AggregateId, req.Payload, req.AggregationKey, 0); err != nil {
		return nil, apperrors.ToStatus(err)
	}
	return &analyticsv1.TrackEventResponse{Success: true}, nil
}

func (h *AnalyticsHandler) GetDailyRevenue(ctx context.Context, req *analyticsv1.GetDailyRevenueRequest) (*analyticsv1.GetDailyRevenueResponse, error) {
	revenue, err := h.usecase.GetDailyRevenue(ctx, req.Date)
	if err != nil {
		return nil, apperrors.ToStatus(err)
	}
	return &analyticsv1.GetDailyRevenueResponse{Revenue: revenue}, nil
}

func (h *AnalyticsHandler) TrackABTestEvent(ctx context.Context, req *analyticsv1.TrackABTestEventRequest) (*analyticsv1.TrackABTestEventResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, apperrors.ToStatus(apperrors.ErrInvalidArgument)
	}
	event := domain.ABTestEvent{
		Experiment:   req.Experiment,
		Variation:    req.Variation,
		UserID:       userID,
		Conversion:   req.Conversion,
		RevenueMinor: req.RevenueMinor,
		CreatedAt:    time.Now().UTC(),
	}
	if err := h.usecase.TrackABTestEvent(ctx, event); err != nil {
		return nil, apperrors.ToStatus(err)
	}
	return &analyticsv1.TrackABTestEventResponse{Success: true}, nil
}
