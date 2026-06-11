package grpc_test

import (
	"context"
	"testing"

	analyticsv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/analytics/v1"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/analytics-service/mocks"
	"github.com/ekhodzitsky/go-ozon-marketplace/tests"
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
			req:  tests.NewTrackEventRequestBuilder().WithEventType("click").WithAggregateID("a1").WithPayload("{}").WithAggregationKey("k").Build(),
			setupMock: func(m *mocks.MockAnalyticsUsecase) {
				m.EXPECT().TrackEvent(gomock.Any(), domain.EventType("click"), "a1", "{}", "k").Return(nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name: "usecase_error",
			req:  tests.NewTrackEventRequestBuilder().WithEventType("click").WithAggregateID("a1").Build(),
			setupMock: func(m *mocks.MockAnalyticsUsecase) {
				m.EXPECT().TrackEvent(gomock.Any(), domain.EventType("click"), "a1", "", "").Return(assert.AnError)
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
			req:  tests.NewGetDailyRevenueRequestBuilder().WithDate("2024-01-01").Build(),
			setupMock: func(m *mocks.MockAnalyticsUsecase) {
				m.EXPECT().GetDailyRevenue(gomock.Any(), "2024-01-01").Return(1234.5, nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name: "usecase_error",
			req:  tests.NewGetDailyRevenueRequestBuilder().WithDate("2024-01-01").Build(),
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
