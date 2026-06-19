package grpc_test

import (
	"context"
	"testing"
	"time"

	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	grpcdelivery "github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/delivery/grpc"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func authCtx(userID string) context.Context {
	return context.WithValue(context.Background(), auth.ContextKeyUserID, userID)
}

func authCtxWithRole(userID string, role auth.Role) context.Context {
	ctx := context.WithValue(context.Background(), auth.ContextKeyUserID, userID)
	return context.WithValue(ctx, auth.ContextKeyRole, string(role))
}

func newProcessPaymentRequest(orderID string, amountCents int64) *paymentv1.ProcessPaymentRequest {
	return &paymentv1.ProcessPaymentRequest{
		OrderId:        orderID,
		AmountCents:    amountCents,
		IdempotencyKey: uuid.New().String(),
	}
}

func newRefundRequest(paymentID string) *paymentv1.RefundRequest {
	return &paymentv1.RefundRequest{
		PaymentId:      paymentID,
		IdempotencyKey: uuid.New().String(),
	}
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
			req:  newProcessPaymentRequest(orderID, 10000),
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
			req:      newProcessPaymentRequest(orderID, 10000),
			wantCode: codes.Unauthenticated,
			wantErr:  true,
		},
		{
			name:    "invalid_order_id",
			ctx:     authCtx(validUser),
			req:     newProcessPaymentRequest("bad", 10000),
			wantErr: true,
		},
		{
			name: "usecase_error",
			ctx:  authCtx(validUser),
			req:  newProcessPaymentRequest(orderID, 10000),
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
			req:  newRefundRequest(paymentID.String()),
			setupMock: func(m *mocks.MockPaymentUsecase) {
				m.EXPECT().GetByID(gomock.Any(), paymentID).
					Return(&domain.Payment{ID: paymentID, UserID: uuid.MustParse(validUser), Status: domain.StatusSuccess}, nil)
				m.EXPECT().Refund(gomock.Any(), paymentID, gomock.Any()).
					Return(&domain.Payment{ID: paymentID, Status: domain.StatusRefunded}, &domain.Refund{}, nil)
			},
			wantCode:   codes.OK,
			wantStatus: paymentv1.PaymentStatus_PAYMENT_STATUS_REFUNDED,
			wantErr:    false,
		},
		{
			name:     "unauthenticated",
			ctx:      context.Background(),
			req:      newRefundRequest(paymentID.String()),
			wantCode: codes.Unauthenticated,
			wantErr:  true,
		},
		{
			name:    "invalid_payment_id",
			ctx:     authCtx(validUser),
			req:     newRefundRequest("bad"),
			wantErr: true,
		},
		{
			name: "permission_denied_other_user",
			ctx:  authCtx(validUser),
			req:  newRefundRequest(paymentID.String()),
			setupMock: func(m *mocks.MockPaymentUsecase) {
				m.EXPECT().GetByID(gomock.Any(), paymentID).
					Return(&domain.Payment{ID: paymentID, UserID: uuid.New(), Status: domain.StatusSuccess}, nil)
			},
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name: "admin_can_refund_any",
			ctx:  authCtxWithRole(validUser, auth.RoleAdmin),
			req:  newRefundRequest(paymentID.String()),
			setupMock: func(m *mocks.MockPaymentUsecase) {
				m.EXPECT().GetByID(gomock.Any(), paymentID).
					Return(&domain.Payment{ID: paymentID, UserID: uuid.New(), Status: domain.StatusSuccess}, nil)
				m.EXPECT().Refund(gomock.Any(), paymentID, gomock.Any()).
					Return(&domain.Payment{ID: paymentID, Status: domain.StatusRefunded}, &domain.Refund{}, nil)
			},
			wantCode:   codes.OK,
			wantStatus: paymentv1.PaymentStatus_PAYMENT_STATUS_REFUNDED,
			wantErr:    false,
		},
		{
			name: "usecase_error",
			ctx:  authCtx(validUser),
			req:  newRefundRequest(paymentID.String()),
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

func TestPaymentHandler_GetRefund(t *testing.T) {
	t.Parallel()

	validUser := uuid.New().String()
	paymentID := uuid.New()
	refundID := uuid.New()

	testsCases := []struct {
		name      string
		ctx       context.Context
		setupMock func(m *mocks.MockPaymentUsecase)
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success_owner",
			ctx:  authCtx(validUser),
			setupMock: func(m *mocks.MockPaymentUsecase) {
				m.EXPECT().GetRefund(gomock.Any(), refundID).
					Return(&domain.Refund{ID: refundID, PaymentID: paymentID, Status: domain.StatusRefunded}, nil)
				m.EXPECT().GetByID(gomock.Any(), paymentID).
					Return(&domain.Payment{ID: paymentID, UserID: uuid.MustParse(validUser)}, nil)
			},
			wantErr: false,
		},
		{
			name:     "unauthenticated",
			ctx:      context.Background(),
			wantCode: codes.Unauthenticated,
			wantErr:  true,
		},
		{
			name: "permission_denied_other_user",
			ctx:  authCtx(validUser),
			setupMock: func(m *mocks.MockPaymentUsecase) {
				m.EXPECT().GetRefund(gomock.Any(), refundID).
					Return(&domain.Refund{ID: refundID, PaymentID: paymentID, Status: domain.StatusRefunded}, nil)
				m.EXPECT().GetByID(gomock.Any(), paymentID).
					Return(&domain.Payment{ID: paymentID, UserID: uuid.New()}, nil)
			},
			wantCode: codes.PermissionDenied,
			wantErr:  true,
		},
		{
			name: "admin_can_get_any",
			ctx:  authCtxWithRole(validUser, auth.RoleAdmin),
			setupMock: func(m *mocks.MockPaymentUsecase) {
				m.EXPECT().GetRefund(gomock.Any(), refundID).
					Return(&domain.Refund{ID: refundID, PaymentID: paymentID, Status: domain.StatusRefunded}, nil)
				m.EXPECT().GetByID(gomock.Any(), paymentID).
					Return(&domain.Payment{ID: paymentID, UserID: uuid.New()}, nil)
			},
			wantErr: false,
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
			resp, err := h.GetRefund(tt.ctx, &paymentv1.GetRefundRequest{RefundId: refundID.String()})
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantCode != codes.OK {
					s, _ := status.FromError(err)
					assert.Equal(t, tt.wantCode, s.Code())
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, resp.Refund)
			assert.Equal(t, refundID.String(), resp.Refund.Id)
		})
	}
}

func TestPaymentHandler_ListRefunds(t *testing.T) {
	t.Parallel()

	validUser := uuid.New().String()
	paymentID := uuid.New()

	testsCases := []struct {
		name      string
		ctx       context.Context
		setupMock func(m *mocks.MockPaymentUsecase)
		wantCode  codes.Code
		wantErr   bool
	}{
		{
			name: "success_owner",
			ctx:  authCtx(validUser),
			setupMock: func(m *mocks.MockPaymentUsecase) {
				m.EXPECT().GetByID(gomock.Any(), paymentID).
					Return(&domain.Payment{ID: paymentID, UserID: uuid.MustParse(validUser)}, nil)
				m.EXPECT().ListRefunds(gomock.Any(), paymentID).
					Return([]*domain.Refund{{ID: uuid.New(), PaymentID: paymentID, Status: domain.StatusRefunded, CreatedAt: time.Now()}}, nil)
			},
			wantErr: false,
		},
		{
			name:     "unauthenticated",
			ctx:      context.Background(),
			wantCode: codes.Unauthenticated,
			wantErr:  true,
		},
		{
			name: "permission_denied_other_user",
			ctx:  authCtx(validUser),
			setupMock: func(m *mocks.MockPaymentUsecase) {
				m.EXPECT().GetByID(gomock.Any(), paymentID).
					Return(&domain.Payment{ID: paymentID, UserID: uuid.New()}, nil)
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
			mockUC := mocks.NewMockPaymentUsecase(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockUC)
			}
			h := grpcdelivery.NewPaymentHandler(mockUC, nil)
			resp, err := h.ListRefunds(tt.ctx, &paymentv1.ListRefundsRequest{PaymentId: paymentID.String()})
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantCode != codes.OK {
					s, _ := status.FromError(err)
					assert.Equal(t, tt.wantCode, s.Code())
				}
				return
			}
			require.NoError(t, err)
			assert.Len(t, resp.Refunds, 1)
		})
	}
}
