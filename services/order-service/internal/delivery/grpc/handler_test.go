package grpc_test

import (
	"context"
	"errors"
	"testing"
	"time"

	orderv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/order/v1"
	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/order-service/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func authCtx(userID string) context.Context {
	return context.WithValue(context.Background(), middleware.ContextKeyUserID, userID)
}

func authCtxWithRole(userID string, role middleware.Role) context.Context {
	ctx := context.WithValue(context.Background(), middleware.ContextKeyUserID, userID)
	return context.WithValue(ctx, middleware.ContextKeyRole, string(role))
}

func newOrderItem(productID string, quantity int32, priceCents int64) *orderv1.OrderItem {
	return &orderv1.OrderItem{
		ProductId:  productID,
		Quantity:   quantity,
		PriceCents: priceCents,
	}
}

func newCreateOrderRequest(items ...*orderv1.OrderItem) *orderv1.CreateOrderRequest {
	return &orderv1.CreateOrderRequest{
		Items:          items,
		IdempotencyKey: uuid.New().String(),
	}
}

func TestOrderHandler_CreateOrder(t *testing.T) {
	t.Parallel()

	validUser := uuid.New().String()
	validProduct := uuid.New().String()

	testsCases := []struct {
		name      string
		ctx       context.Context
		req       *orderv1.CreateOrderRequest
		setupMock func(m *mocks.MockOrderUsecase)
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success_matching_price",
			ctx:  authCtx(validUser),
			req:  newCreateOrderRequest(newOrderItem(validProduct, 1, 1000)),
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().CreateOrder(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(uuid.New(), nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name: "tampered_lower_price",
			ctx:  authCtx(validUser),
			req:  newCreateOrderRequest(newOrderItem(validProduct, 1, 1)),
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().CreateOrder(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(uuid.Nil, apperrors.ErrInvalidArgument)
			},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "tampered_higher_price",
			ctx:  authCtx(validUser),
			req:  newCreateOrderRequest(newOrderItem(validProduct, 1, 999999)),
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().CreateOrder(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(uuid.Nil, apperrors.ErrInvalidArgument)
			},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name:     "unauthenticated",
			ctx:      context.Background(),
			req:      newCreateOrderRequest(newOrderItem(validProduct, 1, 1000)),
			wantCode: codes.Unauthenticated,
			wantErr:  true,
		},
		{
			name:     "missing_idempotency_key",
			ctx:      authCtx(validUser),
			req:      &orderv1.CreateOrderRequest{Items: []*orderv1.OrderItem{newOrderItem(validProduct, 1, 1000)}},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name:     "invalid_product_id",
			ctx:      authCtx(validUser),
			req:      newCreateOrderRequest(newOrderItem("not-a-uuid", 1, 1000)),
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name:     "invalid_price",
			ctx:      authCtx(validUser),
			req:      newCreateOrderRequest(newOrderItem(validProduct, 1, 0)),
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "usecase_error",
			ctx:  authCtx(validUser),
			req:  newCreateOrderRequest(newOrderItem(validProduct, 1, 1000)),
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().CreateOrder(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(uuid.Nil, assert.AnError)
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
			mockUC := mocks.NewMockOrderUsecase(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockUC)
			}
			h := grpcdelivery.NewOrderHandler(mockUC)
			_, err := h.CreateOrder(tt.ctx, tt.req)

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

func TestOrderHandler_GetOrder(t *testing.T) {
	t.Parallel()

	validUser := uuid.New().String()
	orderID := uuid.New()

	testsCases := []struct {
		name      string
		ctx       context.Context
		req       *orderv1.GetOrderRequest
		setupMock func(m *mocks.MockOrderUsecase)
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success",
			ctx:  authCtx(validUser),
			req:  &orderv1.GetOrderRequest{OrderId: orderID.String()},
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().GetOrder(gomock.Any(), gomock.Any()).Return(&domain.Order{
					ID:        orderID,
					UserID:    uuid.MustParse(validUser),
					Items:     nil,
					Status:    domain.OrderStatusPending,
					CreatedAt: time.Now().UTC(),
					UpdatedAt: time.Now().UTC(),
				}, nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name: "success_admin",
			ctx:  authCtxWithRole(uuid.New().String(), middleware.RoleAdmin),
			req:  &orderv1.GetOrderRequest{OrderId: orderID.String()},
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().GetOrder(gomock.Any(), gomock.Any()).Return(&domain.Order{
					ID:        orderID,
					UserID:    uuid.MustParse(validUser),
					Items:     nil,
					Status:    domain.OrderStatusPending,
					CreatedAt: time.Now().UTC(),
					UpdatedAt: time.Now().UTC(),
				}, nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name:     "unauthenticated",
			ctx:      context.Background(),
			req:      &orderv1.GetOrderRequest{OrderId: orderID.String()},
			wantCode: codes.Unauthenticated,
			wantErr:  true,
		},
		{
			name: "not_found",
			ctx:  authCtx(validUser),
			req:  &orderv1.GetOrderRequest{OrderId: orderID.String()},
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().GetOrder(gomock.Any(), gomock.Any()).Return(nil, apperrors.ErrNotFound)
			},
			wantCode: codes.NotFound,
			wantErr:  true,
		},
		{
			name: "permission_denied_other_user",
			ctx:  authCtx(uuid.New().String()),
			req:  &orderv1.GetOrderRequest{OrderId: orderID.String()},
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().GetOrder(gomock.Any(), gomock.Any()).Return(&domain.Order{
					ID:     orderID,
					UserID: uuid.New(),
				}, nil)
			},
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
	}

	for _, tt := range testsCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockUC := mocks.NewMockOrderUsecase(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockUC)
			}
			h := grpcdelivery.NewOrderHandler(mockUC)
			_, err := h.GetOrder(tt.ctx, tt.req)

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

func TestOrderHandler_ListOrders(t *testing.T) {
	t.Parallel()

	validUser := uuid.New().String()

	testsCases := []struct {
		name      string
		ctx       context.Context
		req       *orderv1.ListOrdersRequest
		setupMock func(m *mocks.MockOrderUsecase)
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success",
			ctx:  authCtx(validUser),
			req:  &orderv1.ListOrdersRequest{Page: 1, PageSize: 10},
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().ListOrders(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]domain.Order{}, 0, nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name:     "unauthenticated",
			ctx:      context.Background(),
			req:      &orderv1.ListOrdersRequest{Page: 1, PageSize: 10},
			wantCode: codes.Unauthenticated,
			wantErr:  true,
		},
	}

	for _, tt := range testsCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockUC := mocks.NewMockOrderUsecase(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockUC)
			}
			h := grpcdelivery.NewOrderHandler(mockUC)
			_, err := h.ListOrders(tt.ctx, tt.req)

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

func TestOrderHandler_CancelOrder(t *testing.T) {
	t.Parallel()

	validUser := uuid.New().String()
	orderID := uuid.New()

	testsCases := []struct {
		name      string
		ctx       context.Context
		req       *orderv1.CancelOrderRequest
		setupMock func(m *mocks.MockOrderUsecase)
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success_owner",
			ctx:  authCtx(validUser),
			req:  &orderv1.CancelOrderRequest{OrderId: orderID.String()},
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().GetOrder(gomock.Any(), orderID).Return(&domain.Order{
					ID:     orderID,
					UserID: uuid.MustParse(validUser),
				}, nil)
				m.EXPECT().CancelOrder(gomock.Any(), orderID).Return(nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name: "success_admin",
			ctx:  authCtxWithRole(uuid.New().String(), middleware.RoleAdmin),
			req:  &orderv1.CancelOrderRequest{OrderId: orderID.String()},
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().GetOrder(gomock.Any(), orderID).Return(&domain.Order{
					ID:     orderID,
					UserID: uuid.MustParse(validUser),
				}, nil)
				m.EXPECT().CancelOrder(gomock.Any(), orderID).Return(nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name:     "unauthenticated",
			ctx:      context.Background(),
			req:      &orderv1.CancelOrderRequest{OrderId: orderID.String()},
			wantCode: codes.Unauthenticated,
			wantErr:  true,
		},
		{
			name:     "invalid_order_id",
			ctx:      authCtx(validUser),
			req:      &orderv1.CancelOrderRequest{OrderId: "bad"},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "not_found",
			ctx:  authCtx(validUser),
			req:  &orderv1.CancelOrderRequest{OrderId: orderID.String()},
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().GetOrder(gomock.Any(), orderID).Return(nil, apperrors.ErrNotFound)
			},
			wantCode: codes.NotFound,
			wantErr:  true,
		},
		{
			name: "permission_denied",
			ctx:  authCtx(uuid.New().String()),
			req:  &orderv1.CancelOrderRequest{OrderId: orderID.String()},
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().GetOrder(gomock.Any(), orderID).Return(&domain.Order{
					ID:     orderID,
					UserID: uuid.MustParse(validUser),
				}, nil)
			},
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name: "invalid_argument_from_usecase",
			ctx:  authCtx(validUser),
			req:  &orderv1.CancelOrderRequest{OrderId: orderID.String()},
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().GetOrder(gomock.Any(), orderID).Return(&domain.Order{
					ID:     orderID,
					UserID: uuid.MustParse(validUser),
				}, nil)
				m.EXPECT().CancelOrder(gomock.Any(), orderID).Return(apperrors.ErrInvalidArgument)
			},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "failed_precondition_from_usecase",
			ctx:  authCtx(validUser),
			req:  &orderv1.CancelOrderRequest{OrderId: orderID.String()},
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().GetOrder(gomock.Any(), orderID).Return(&domain.Order{
					ID:     orderID,
					UserID: uuid.MustParse(validUser),
				}, nil)
				m.EXPECT().CancelOrder(gomock.Any(), orderID).Return(apperrors.ErrFailedPrecondition)
			},
			wantCode: codes.FailedPrecondition,
			wantErr:  true,
		},
		{
			name: "internal_error",
			ctx:  authCtx(validUser),
			req:  &orderv1.CancelOrderRequest{OrderId: orderID.String()},
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().GetOrder(gomock.Any(), orderID).Return(&domain.Order{
					ID:     orderID,
					UserID: uuid.MustParse(validUser),
				}, nil)
				m.EXPECT().CancelOrder(gomock.Any(), orderID).Return(errors.New("boom"))
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
			mockUC := mocks.NewMockOrderUsecase(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockUC)
			}
			h := grpcdelivery.NewOrderHandler(mockUC)
			_, err := h.CancelOrder(tt.ctx, tt.req)

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

func TestOrderHandler_UpdateOrderStatus(t *testing.T) {
	t.Parallel()

	validUser := uuid.New().String()
	orderID := uuid.New()

	testsCases := []struct {
		name      string
		ctx       context.Context
		req       *orderv1.UpdateOrderStatusRequest
		setupMock func(m *mocks.MockOrderUsecase)
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success_admin",
			ctx:  authCtxWithRole(validUser, middleware.RoleAdmin),
			req:  &orderv1.UpdateOrderStatusRequest{OrderId: orderID.String(), Status: orderv1.OrderStatus_ORDER_STATUS_SHIPPED},
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().UpdateOrderStatus(gomock.Any(), orderID, domain.OrderStatusShipped).Return(nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name: "success_service",
			ctx:  authCtxWithRole(validUser, middleware.RoleService),
			req:  &orderv1.UpdateOrderStatusRequest{OrderId: orderID.String(), Status: orderv1.OrderStatus_ORDER_STATUS_SHIPPED},
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().UpdateOrderStatus(gomock.Any(), orderID, domain.OrderStatusShipped).Return(nil)
			},
			wantCode: codes.OK,
			wantErr:  false,
		},
		{
			name:     "unauthenticated",
			ctx:      context.Background(),
			req:      &orderv1.UpdateOrderStatusRequest{OrderId: orderID.String(), Status: orderv1.OrderStatus_ORDER_STATUS_SHIPPED},
			wantCode: codes.Unauthenticated,
			wantErr:  true,
		},
		{
			name:     "permission_denied_user",
			ctx:      authCtx(validUser),
			req:      &orderv1.UpdateOrderStatusRequest{OrderId: orderID.String(), Status: orderv1.OrderStatus_ORDER_STATUS_SHIPPED},
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name:     "invalid_order_id",
			ctx:      authCtxWithRole(validUser, middleware.RoleAdmin),
			req:      &orderv1.UpdateOrderStatusRequest{OrderId: "bad", Status: orderv1.OrderStatus_ORDER_STATUS_SHIPPED},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name:     "unspecified_status",
			ctx:      authCtxWithRole(validUser, middleware.RoleAdmin),
			req:      &orderv1.UpdateOrderStatusRequest{OrderId: orderID.String(), Status: orderv1.OrderStatus_ORDER_STATUS_UNSPECIFIED},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "invalid_argument_from_usecase",
			ctx:  authCtxWithRole(validUser, middleware.RoleAdmin),
			req:  &orderv1.UpdateOrderStatusRequest{OrderId: orderID.String(), Status: orderv1.OrderStatus_ORDER_STATUS_SHIPPED},
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().UpdateOrderStatus(gomock.Any(), orderID, domain.OrderStatusShipped).Return(apperrors.ErrInvalidArgument)
			},
			wantCode: codes.InvalidArgument,
			wantErr:  true,
		},
		{
			name: "internal_error",
			ctx:  authCtxWithRole(validUser, middleware.RoleAdmin),
			req:  &orderv1.UpdateOrderStatusRequest{OrderId: orderID.String(), Status: orderv1.OrderStatus_ORDER_STATUS_SHIPPED},
			setupMock: func(m *mocks.MockOrderUsecase) {
				m.EXPECT().UpdateOrderStatus(gomock.Any(), orderID, domain.OrderStatusShipped).Return(errors.New("boom"))
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
			mockUC := mocks.NewMockOrderUsecase(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockUC)
			}
			h := grpcdelivery.NewOrderHandler(mockUC)
			_, err := h.UpdateOrderStatus(tt.ctx, tt.req)

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
