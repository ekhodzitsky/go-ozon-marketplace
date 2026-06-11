package grpc_test

import (
	"context"
	"testing"

	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/mocks"
	"github.com/ekhodzitsky/go-ozon-marketplace/tests"
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

func TestPaymentHandler_ProcessPayment(t *testing.T) {
	t.Parallel()

	validUser := uuid.New().String()
	orderID := uuid.New().String()

	testsCases := []struct {
		name       string
		ctx        context.Context
		req        *paymentv1.ProcessPaymentRequest
		setupMock  func(m *mocks.MockPaymentUsecase)
		wantCode   codes.Code
		wantStatus paymentv1.PaymentStatus
		wantErr    bool
	}{
		{
			name: "success",
			ctx:  authCtx(validUser),
			req:  tests.NewProcessPaymentRequestBuilder().WithOrderID(orderID).WithUserID(validUser).WithAmount(100.0).Build(),
			setupMock: func(m *mocks.MockPaymentUsecase) {
				m.EXPECT().ProcessPayment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&domain.Payment{ID: uuid.New(), Status: domain.StatusSuccess}, nil)
			},
			wantCode:   codes.OK,
			wantStatus: paymentv1.PaymentStatus_PAYMENT_STATUS_SUCCESS,
			wantErr:    false,
		},
		{
			name:     "unauthenticated",
			ctx:      context.Background(),
			req:      tests.NewProcessPaymentRequestBuilder().WithOrderID(orderID).WithUserID(validUser).WithAmount(100.0).Build(),
			wantCode: codes.Unauthenticated,
			wantErr:  true,
		},
		{
			name:    "invalid_order_id",
			ctx:     authCtx(validUser),
			req:     tests.NewProcessPaymentRequestBuilder().WithOrderID("bad").WithUserID(validUser).WithAmount(100.0).Build(),
			wantErr: true,
		},
		{
			name:    "invalid_user_id",
			ctx:     authCtx(validUser),
			req:     tests.NewProcessPaymentRequestBuilder().WithOrderID(orderID).WithUserID("bad").WithAmount(100.0).Build(),
			wantErr: true,
		},
		{
			name:     "user_id_mismatch",
			ctx:      authCtx(validUser),
			req:      tests.NewProcessPaymentRequestBuilder().WithOrderID(orderID).WithUserID(uuid.New().String()).WithAmount(100.0).Build(),
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name: "admin_can_process_for_other",
			ctx:  authCtxWithRole(validUser, middleware.RoleAdmin),
			req:  tests.NewProcessPaymentRequestBuilder().WithOrderID(orderID).WithUserID(uuid.New().String()).WithAmount(100.0).Build(),
			setupMock: func(m *mocks.MockPaymentUsecase) {
				m.EXPECT().ProcessPayment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&domain.Payment{ID: uuid.New(), Status: domain.StatusSuccess}, nil)
			},
			wantCode:   codes.OK,
			wantStatus: paymentv1.PaymentStatus_PAYMENT_STATUS_SUCCESS,
			wantErr:    false,
		},
		{
			name: "usecase_error",
			ctx:  authCtx(validUser),
			req:  tests.NewProcessPaymentRequestBuilder().WithOrderID(orderID).WithUserID(validUser).WithAmount(100.0).Build(),
			setupMock: func(m *mocks.MockPaymentUsecase) {
				m.EXPECT().ProcessPayment(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, assert.AnError)
			},
			wantErr: true,
		},
	}

	for _, tt := range testsCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockUC := mocks.NewMockPaymentUsecase(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockUC)
			}
			h := grpcdelivery.NewPaymentHandler(mockUC, nil)
			resp, err := h.ProcessPayment(tt.ctx, tt.req)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantCode != codes.OK {
					s, _ := status.FromError(err)
					assert.Equal(t, tt.wantCode, s.Code())
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, resp.Status)
		})
	}
}

func TestPaymentHandler_Refund(t *testing.T) {
	t.Parallel()

	validUser := uuid.New().String()
	paymentID := uuid.New()

	testsCases := []struct {
		name       string
		ctx        context.Context
		req        *paymentv1.RefundRequest
		setupMock  func(m *mocks.MockPaymentUsecase)
		wantCode   codes.Code
		wantStatus paymentv1.PaymentStatus
		wantErr    bool
	}{
		{
			name: "success_owner",
			ctx:  authCtx(validUser),
			req:  tests.NewRefundRequestBuilder().WithPaymentID(paymentID.String()).Build(),
			setupMock: func(m *mocks.MockPaymentUsecase) {
				m.EXPECT().GetByID(gomock.Any(), paymentID).
					Return(&domain.Payment{ID: paymentID, UserID: uuid.MustParse(validUser), Status: domain.StatusSuccess}, nil)
				m.EXPECT().Refund(gomock.Any(), paymentID).
					Return(&domain.Payment{ID: paymentID, Status: domain.StatusRefunded}, nil)
			},
			wantCode:   codes.OK,
			wantStatus: paymentv1.PaymentStatus_PAYMENT_STATUS_REFUNDED,
			wantErr:    false,
		},
		{
			name:     "unauthenticated",
			ctx:      context.Background(),
			req:      tests.NewRefundRequestBuilder().WithPaymentID(paymentID.String()).Build(),
			wantCode: codes.Unauthenticated,
			wantErr:  true,
		},
		{
			name:    "invalid_payment_id",
			ctx:     authCtx(validUser),
			req:     tests.NewRefundRequestBuilder().WithPaymentID("bad").Build(),
			wantErr: true,
		},
		{
			name: "permission_denied_other_user",
			ctx:  authCtx(validUser),
			req:  tests.NewRefundRequestBuilder().WithPaymentID(paymentID.String()).Build(),
			setupMock: func(m *mocks.MockPaymentUsecase) {
				m.EXPECT().GetByID(gomock.Any(), paymentID).
					Return(&domain.Payment{ID: paymentID, UserID: uuid.New(), Status: domain.StatusSuccess}, nil)
			},
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name: "admin_can_refund_any",
			ctx:  authCtxWithRole(validUser, middleware.RoleAdmin),
			req:  tests.NewRefundRequestBuilder().WithPaymentID(paymentID.String()).Build(),
			setupMock: func(m *mocks.MockPaymentUsecase) {
				m.EXPECT().GetByID(gomock.Any(), paymentID).
					Return(&domain.Payment{ID: paymentID, UserID: uuid.New(), Status: domain.StatusSuccess}, nil)
				m.EXPECT().Refund(gomock.Any(), paymentID).
					Return(&domain.Payment{ID: paymentID, Status: domain.StatusRefunded}, nil)
			},
			wantCode:   codes.OK,
			wantStatus: paymentv1.PaymentStatus_PAYMENT_STATUS_REFUNDED,
			wantErr:    false,
		},
		{
			name: "usecase_error",
			ctx:  authCtx(validUser),
			req:  tests.NewRefundRequestBuilder().WithPaymentID(paymentID.String()).Build(),
			setupMock: func(m *mocks.MockPaymentUsecase) {
				m.EXPECT().GetByID(gomock.Any(), paymentID).Return(nil, assert.AnError)
			},
			wantErr: true,
		},
	}

	for _, tt := range testsCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockUC := mocks.NewMockPaymentUsecase(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockUC)
			}
			h := grpcdelivery.NewPaymentHandler(mockUC, nil)
			resp, err := h.Refund(tt.ctx, tt.req)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantCode != codes.OK {
					s, _ := status.FromError(err)
					assert.Equal(t, tt.wantCode, s.Code())
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, resp.Status)
		})
	}
}
