package grpc

import (
	"context"

	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/dlq"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/usecase"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PaymentHandler struct {
	paymentv1.UnimplementedPaymentServiceServer
	usecase usecase.PaymentUsecase
	dlq     *dlq.Producer
}

func NewPaymentHandler(uc usecase.PaymentUsecase, dlqProducer *dlq.Producer) *PaymentHandler {
	return &PaymentHandler{usecase: uc, dlq: dlqProducer}
}

func statusToProto(s domain.Status) paymentv1.PaymentStatus {
	switch s {
	case domain.StatusPending:
		return paymentv1.PaymentStatus_PAYMENT_STATUS_PENDING
	case domain.StatusSuccess:
		return paymentv1.PaymentStatus_PAYMENT_STATUS_SUCCESS
	case domain.StatusFailed:
		return paymentv1.PaymentStatus_PAYMENT_STATUS_FAILED
	case domain.StatusRefunded:
		return paymentv1.PaymentStatus_PAYMENT_STATUS_REFUNDED
	default:
		return paymentv1.PaymentStatus_PAYMENT_STATUS_UNSPECIFIED
	}
}

func (h *PaymentHandler) ProcessPayment(ctx context.Context, req *paymentv1.ProcessPaymentRequest) (*paymentv1.ProcessPaymentResponse, error) {
	authUserID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user_id in context")
	}
	role, _ := middleware.GetRole(ctx)

	orderID, err := uuid.Parse(req.OrderId)
	if err != nil {
		return nil, apperrors.ToStatus(err)
	}
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, apperrors.ToStatus(err)
	}

	if req.UserId != authUserID && role != middleware.RoleAdmin {
		return nil, status.Error(codes.PermissionDenied, "user_id mismatch")
	}

	payment, err := h.usecase.ProcessPayment(ctx, orderID, userID, int64(req.Amount*100))
	if err != nil {
		_ = h.dlq.SendToDLQ("ProcessPaymentFailed", req.String(), err.Error())
		return nil, apperrors.ToStatus(err)
	}

	return &paymentv1.ProcessPaymentResponse{
		PaymentId: payment.ID.String(),
		Status:    statusToProto(payment.Status),
	}, nil
}

func (h *PaymentHandler) Refund(ctx context.Context, req *paymentv1.RefundRequest) (*paymentv1.RefundResponse, error) {
	authUserID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user_id in context")
	}
	role, _ := middleware.GetRole(ctx)

	paymentID, err := uuid.Parse(req.PaymentId)
	if err != nil {
		return nil, apperrors.ToStatus(err)
	}

	payment, err := h.usecase.GetByID(ctx, paymentID)
	if err != nil {
		return nil, apperrors.ToStatus(err)
	}

	if role != middleware.RoleAdmin && payment.UserID.String() != authUserID {
		return nil, status.Error(codes.PermissionDenied, "payment does not belong to user")
	}

	payment, err = h.usecase.Refund(ctx, paymentID)
	if err != nil {
		return nil, apperrors.ToStatus(err)
	}

	return &paymentv1.RefundResponse{
		Status: statusToProto(payment.Status),
	}, nil
}
