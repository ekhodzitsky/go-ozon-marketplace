package grpc_test

import (
	"context"
	"testing"

	analyticsv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/analytics/v1"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAnalyticsHandler_TrackEvent(t *testing.T) {
	t.Parallel()

	testsCases := []struct {
		name      string
		req       *analyticsv1.TrackEventRequest
		setupMock func(m *mocks.MockAnalyticsUsecase)
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success",
			req: &analyticsv1.TrackEventRequest{
				EventType:      "click",
				AggregateId:    "a1",
				Payload:        "{}",
				AggregationKey: "k",
			},
			setupMock: func(m *mocks.MockAnalyticsUsecase) {
				m.EXPECT().TrackEvent(gomock.Any(), domain.EventType("click"), "a1", "{}", "k", float64(0)).Return(nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name: "usecase_error",
			req: &analyticsv1.TrackEventRequest{
				EventType:   "click",
				AggregateId: "a1",
			},
			setupMock: func(m *mocks.MockAnalyticsUsecase) {
				m.EXPECT().TrackEvent(gomock.Any(), domain.EventType("click"), "a1", "", "", float64(0)).Return(assert.AnError)
			},
			wantCode: codes.Internal,
			wantErr:  true,
		},
	}

	for _, tt := range testsCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			mockUC := mocks.NewMockAnalyticsUsecase(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockUC)
			}
			h := grpcdelivery.NewAnalyticsHandler(mockUC)
			_, err := h.TrackEvent(context.Background(), tt.req)

			if tt.wantErr {
				require.Error(t, err)
				s, _ := status.FromError(err)
				assert.Equal(t, tt.wantCode, s.Code())
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestAnalyticsHandler_GetDailyRevenue(t *testing.T) {
	t.Parallel()

	testsCases := []struct {
		name      string
		req       *analyticsv1.GetDailyRevenueRequest
		setupMock func(m *mocks.MockAnalyticsUsecase)
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success",
			req:  &analyticsv1.GetDailyRevenueRequest{Date: "2024-01-01"},
			setupMock: func(m *mocks.MockAnalyticsUsecase) {
				m.EXPECT().GetDailyRevenue(gomock.Any(), "2024-01-01").Return(1234.5, nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name: "usecase_error",
			req:  &analyticsv1.GetDailyRevenueRequest{Date: "2024-01-01"},
			setupMock: func(m *mocks.MockAnalyticsUsecase) {
				m.EXPECT().GetDailyRevenue(gomock.Any(), "2024-01-01").Return(0.0, assert.AnError)
			},
			wantCode: codes.Internal,
			wantErr:  true,
		},
	}

	for _, tt := range testsCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			mockUC := mocks.NewMockAnalyticsUsecase(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockUC)
			}
			h := grpcdelivery.NewAnalyticsHandler(mockUC)
			_, err := h.GetDailyRevenue(context.Background(), tt.req)

			if tt.wantErr {
				require.Error(t, err)
				s, _ := status.FromError(err)
				assert.Equal(t, tt.wantCode, s.Code())
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestAnalyticsHandler_TrackABTestEvent(t *testing.T) {
	t.Parallel()

	validUserID := uuid.Must(uuid.NewV7()).String()

	testsCases := []struct {
		name      string
		req       *analyticsv1.TrackABTestEventRequest
		setupMock func(m *mocks.MockAnalyticsUsecase)
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success",
			req: &analyticsv1.TrackABTestEventRequest{
				Experiment:   "exp-1",
				Variation:    "var-a",
				UserId:       validUserID,
				Conversion:   true,
				RevenueMinor: 100,
			},
			setupMock: func(m *mocks.MockAnalyticsUsecase) {
				m.EXPECT().TrackABTestEvent(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name: "invalid_user_id",
			req: &analyticsv1.TrackABTestEventRequest{
				Experiment:   "exp-1",
				Variation:    "var-a",
				UserId:       "not-a-uuid",
				Conversion:   true,
				RevenueMinor: 100,
			},
			setupMock: func(m *mocks.MockAnalyticsUsecase) {},
			wantCode:  codes.InvalidArgument,
			wantErr:   true,
		},
	}

	for _, tt := range testsCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			mockUC := mocks.NewMockAnalyticsUsecase(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockUC)
			}
			h := grpcdelivery.NewAnalyticsHandler(mockUC)
			_, err := h.TrackABTestEvent(context.Background(), tt.req)

			if tt.wantErr {
				require.Error(t, err)
				s, _ := status.FromError(err)
				assert.Equal(t, tt.wantCode, s.Code())
				return
			}
			require.NoError(t, err)
		})
	}
}
