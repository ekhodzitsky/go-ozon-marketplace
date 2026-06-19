package grpc_test

import (
	"context"
	"testing"
	"time"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func authCtxWithRole(role auth.Role) context.Context {
	return context.WithValue(context.Background(), auth.ContextKeyRole, string(role))
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
			ctx:  authCtxWithRole(auth.RoleService),
			req:  &inventoryv1.ReserveRequest{ProductId: validProduct, OrderId: validOrder, Quantity: 1, IdempotencyKey: uuid.New().String()},
			setupMock: func(m *mocks.MockInventoryUsecase) {
				m.EXPECT().Reserve(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name:     "missing_role",
			ctx:      context.Background(),
			req:      &inventoryv1.ReserveRequest{ProductId: validProduct, OrderId: validOrder, Quantity: 1, IdempotencyKey: uuid.New().String()},
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name:     "external_role_denied",
			ctx:      authCtxWithRole(auth.RoleUser),
			req:      &inventoryv1.ReserveRequest{ProductId: validProduct, OrderId: validOrder, Quantity: 1, IdempotencyKey: uuid.New().String()},
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name:     "invalid_product_id",
			ctx:      authCtxWithRole(auth.RoleService),
			req:      &inventoryv1.ReserveRequest{ProductId: "bad", OrderId: validOrder, Quantity: 1, IdempotencyKey: uuid.New().String()},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name:     "invalid_order_id",
			ctx:      authCtxWithRole(auth.RoleService),
			req:      &inventoryv1.ReserveRequest{ProductId: validProduct, OrderId: "bad", Quantity: 1, IdempotencyKey: uuid.New().String()},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name:     "missing_idempotency_key",
			ctx:      authCtxWithRole(auth.RoleService),
			req:      &inventoryv1.ReserveRequest{ProductId: validProduct, OrderId: validOrder, Quantity: 1},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "usecase_error",
			ctx:  authCtxWithRole(auth.RoleService),
			req:  &inventoryv1.ReserveRequest{ProductId: validProduct, OrderId: validOrder, Quantity: 1, IdempotencyKey: uuid.New().String()},
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
			ctx:  authCtxWithRole(auth.RoleService),
			req:  &inventoryv1.ReleaseRequest{ProductId: validProduct, OrderId: validOrder, Quantity: 1, IdempotencyKey: uuid.New().String()},
			setupMock: func(m *mocks.MockInventoryUsecase) {
				m.EXPECT().Release(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name:     "missing_role",
			ctx:      context.Background(),
			req:      &inventoryv1.ReleaseRequest{ProductId: validProduct, OrderId: validOrder, Quantity: 1, IdempotencyKey: uuid.New().String()},
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name:     "external_role_denied",
			ctx:      authCtxWithRole(auth.RoleUser),
			req:      &inventoryv1.ReleaseRequest{ProductId: validProduct, OrderId: validOrder, Quantity: 1, IdempotencyKey: uuid.New().String()},
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name:     "invalid_product_id",
			ctx:      authCtxWithRole(auth.RoleService),
			req:      &inventoryv1.ReleaseRequest{ProductId: "bad", OrderId: validOrder, Quantity: 1, IdempotencyKey: uuid.New().String()},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name:     "invalid_order_id",
			ctx:      authCtxWithRole(auth.RoleService),
			req:      &inventoryv1.ReleaseRequest{ProductId: validProduct, OrderId: "bad", Quantity: 1, IdempotencyKey: uuid.New().String()},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name:     "missing_idempotency_key",
			ctx:      authCtxWithRole(auth.RoleService),
			req:      &inventoryv1.ReleaseRequest{ProductId: validProduct, OrderId: validOrder, Quantity: 1},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "usecase_error",
			ctx:  authCtxWithRole(auth.RoleService),
			req:  &inventoryv1.ReleaseRequest{ProductId: validProduct, OrderId: validOrder, Quantity: 1, IdempotencyKey: uuid.New().String()},
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
			req:  &inventoryv1.GetStockRequest{ProductId: validProduct},
			setupMock: func(m *mocks.MockInventoryUsecase) {
				m.EXPECT().GetStock(gomock.Any(), gomock.Any()).Return(&domain.Stock{Available: 10, Reserved: 2}, nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name:     "invalid_product_id",
			req:      &inventoryv1.GetStockRequest{ProductId: "bad"},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "not_found",
			req:  &inventoryv1.GetStockRequest{ProductId: validProduct},
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

func TestInventoryHandler_GetLedger(t *testing.T) {
	t.Parallel()

	validProduct := uuid.New().String()
	orderID := uuid.New()
	now := time.Now()

	testsCases := []struct {
		name      string
		req       *inventoryv1.GetLedgerRequest
		setupMock func(m *mocks.MockInventoryUsecase)
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success",
			req:  &inventoryv1.GetLedgerRequest{ProductId: validProduct},
			setupMock: func(m *mocks.MockInventoryUsecase) {
				m.EXPECT().GetLedger(gomock.Any(), gomock.Any()).Return([]*domain.LedgerEntry{
					{ID: uuid.New(), ProductID: uuid.MustParse(validProduct), OrderID: &orderID, QuantityChange: -5, OperationType: "LEDGER_OPERATION_RESERVE", CreatedAt: now},
				}, nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name:     "invalid_product_id",
			req:      &inventoryv1.GetLedgerRequest{ProductId: "bad"},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "usecase_error",
			req:  &inventoryv1.GetLedgerRequest{ProductId: validProduct},
			setupMock: func(m *mocks.MockInventoryUsecase) {
				m.EXPECT().GetLedger(gomock.Any(), gomock.Any()).Return(nil, assert.AnError)
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
			_, err := h.GetLedger(context.Background(), tt.req)
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
