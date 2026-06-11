package grpc_test

import (
	"context"
	"testing"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/mocks"
	"github.com/ekhodzitsky/go-ozon-marketplace/tests"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func authCtxWithRole(role middleware.Role) context.Context {
	return context.WithValue(context.Background(), middleware.ContextKeyRole, string(role))
}

func TestInventoryHandler_Reserve(t *testing.T) {
	t.Parallel()

	validProduct := uuid.New().String()
	validOrder := uuid.New().String()

	testsCases := []struct {
		name      string
		ctx       context.Context
		req       *inventoryv1.ReserveRequest
		setupMock func(m *mocks.MockInventoryUsecase)
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success",
			ctx:  authCtxWithRole(middleware.RoleService),
			req:  tests.NewReserveRequestBuilder().WithProductID(validProduct).WithOrderID(validOrder).WithQuantity(1).Build(),
			setupMock: func(m *mocks.MockInventoryUsecase) {
				m.EXPECT().Reserve(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name:     "missing_role",
			ctx:      context.Background(),
			req:      tests.NewReserveRequestBuilder().WithProductID(validProduct).WithOrderID(validOrder).WithQuantity(1).Build(),
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name:     "external_role_denied",
			ctx:      authCtxWithRole(middleware.RoleUser),
			req:      tests.NewReserveRequestBuilder().WithProductID(validProduct).WithOrderID(validOrder).WithQuantity(1).Build(),
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name:     "invalid_product_id",
			ctx:      authCtxWithRole(middleware.RoleService),
			req:      tests.NewReserveRequestBuilder().WithProductID("bad").WithOrderID(validOrder).WithQuantity(1).Build(),
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name:     "invalid_order_id",
			ctx:      authCtxWithRole(middleware.RoleService),
			req:      tests.NewReserveRequestBuilder().WithProductID(validProduct).WithOrderID("bad").WithQuantity(1).Build(),
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "usecase_error",
			ctx:  authCtxWithRole(middleware.RoleService),
			req:  tests.NewReserveRequestBuilder().WithProductID(validProduct).WithOrderID(validOrder).WithQuantity(1).Build(),
			setupMock: func(m *mocks.MockInventoryUsecase) {
				m.EXPECT().Reserve(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(assert.AnError)
			},
			wantCode: codes.Internal,
			wantErr:  true,
		},
	}

	for _, tt := range testsCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockUC := mocks.NewMockInventoryUsecase(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockUC)
			}
			h := grpcdelivery.NewInventoryHandler(mockUC)
			_, err := h.Reserve(tt.ctx, tt.req)
			if tt.wantErr {
				require.Error(t, err)
				s, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.wantCode, s.Code())
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestInventoryHandler_Release(t *testing.T) {
	t.Parallel()

	validProduct := uuid.New().String()
	validOrder := uuid.New().String()

	testsCases := []struct {
		name      string
		ctx       context.Context
		req       *inventoryv1.ReleaseRequest
		setupMock func(m *mocks.MockInventoryUsecase)
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success",
			ctx:  authCtxWithRole(middleware.RoleService),
			req:  tests.NewReleaseRequestBuilder().WithProductID(validProduct).WithOrderID(validOrder).WithQuantity(1).Build(),
			setupMock: func(m *mocks.MockInventoryUsecase) {
				m.EXPECT().Release(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name:     "missing_role",
			ctx:      context.Background(),
			req:      tests.NewReleaseRequestBuilder().WithProductID(validProduct).WithOrderID(validOrder).WithQuantity(1).Build(),
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name:     "external_role_denied",
			ctx:      authCtxWithRole(middleware.RoleUser),
			req:      tests.NewReleaseRequestBuilder().WithProductID(validProduct).WithOrderID(validOrder).WithQuantity(1).Build(),
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name:     "invalid_product_id",
			ctx:      authCtxWithRole(middleware.RoleService),
			req:      tests.NewReleaseRequestBuilder().WithProductID("bad").WithOrderID(validOrder).WithQuantity(1).Build(),
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name:     "invalid_order_id",
			ctx:      authCtxWithRole(middleware.RoleService),
			req:      tests.NewReleaseRequestBuilder().WithProductID(validProduct).WithOrderID("bad").WithQuantity(1).Build(),
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "usecase_error",
			ctx:  authCtxWithRole(middleware.RoleService),
			req:  tests.NewReleaseRequestBuilder().WithProductID(validProduct).WithOrderID(validOrder).WithQuantity(1).Build(),
			setupMock: func(m *mocks.MockInventoryUsecase) {
				m.EXPECT().Release(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(assert.AnError)
			},
			wantCode: codes.Internal,
			wantErr:  true,
		},
	}

	for _, tt := range testsCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockUC := mocks.NewMockInventoryUsecase(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockUC)
			}
			h := grpcdelivery.NewInventoryHandler(mockUC)
			_, err := h.Release(tt.ctx, tt.req)
			if tt.wantErr {
				require.Error(t, err)
				s, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.wantCode, s.Code())
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestInventoryHandler_GetStock(t *testing.T) {
	t.Parallel()

	validProduct := uuid.New().String()

	testsCases := []struct {
		name      string
		req       *inventoryv1.GetStockRequest
		setupMock func(m *mocks.MockInventoryUsecase)
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success",
			req:  tests.NewGetStockRequestBuilder().WithProductID(validProduct).Build(),
			setupMock: func(m *mocks.MockInventoryUsecase) {
				m.EXPECT().GetStock(gomock.Any(), gomock.Any()).Return(&domain.Stock{Available: 10, Reserved: 2}, nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name:     "invalid_product_id",
			req:      tests.NewGetStockRequestBuilder().WithProductID("bad").Build(),
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "not_found",
			req:  tests.NewGetStockRequestBuilder().WithProductID(validProduct).Build(),
			setupMock: func(m *mocks.MockInventoryUsecase) {
				m.EXPECT().GetStock(gomock.Any(), gomock.Any()).Return(nil, apperrors.ErrNotFound)
			},
			wantCode: codes.NotFound,
			wantErr:  true,
		},
	}

	for _, tt := range testsCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockUC := mocks.NewMockInventoryUsecase(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockUC)
			}
			h := grpcdelivery.NewInventoryHandler(mockUC)
			_, err := h.GetStock(context.Background(), tt.req)
			if tt.wantErr {
				require.Error(t, err)
				s, ok := status.FromError(err)
				require.True(t, ok)
				assert.Equal(t, tt.wantCode, s.Code())
				return
			}
			require.NoError(t, err)
		})
	}
}
