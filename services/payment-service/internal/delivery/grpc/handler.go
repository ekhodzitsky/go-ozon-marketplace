package grpc

import (
	"context"
	"time"

	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
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

	if req.AmountCents <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount_cents must be greater than 0")
	}
	if req.IdempotencyKey == "" {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key is required")
	}

	orderID, err := uuid.Parse(req.OrderId)
	if err != nil {
		return nil, apperrors.ToStatus(err)
	}
	userID, err := uuid.Parse(authUserID)
	if err != nil {
		return nil, apperrors.ToStatus(err)
	}

	payment, err := h.usecase.ProcessPayment(ctx, orderID, userID, req.AmountCents)
	if err != nil {
		if h.dlq != nil {
			h.dlq.SendToDLQ("ProcessPaymentFailed", req.String(), err.Error())
		}
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

	if req.IdempotencyKey == "" {
		return nil, status.Error(codes.InvalidArgument, "idempotency_key is required")
	}

	paymentID, err := uuid.Parse(req.PaymentId)
	if err != nil {
		return nil, apperrors.ToStatus(err)
	}

	payment, err := h.usecase.GetByID(ctx, paymentID)
	if err != nil {
		return nil, apperrors.ToStatus(err)
	}

	if role != auth.RoleAdmin && payment.UserID.String() != authUserID {
		return nil, status.Error(codes.PermissionDenied, "payment does not belong to user")
	}

	payment, _, err = h.usecase.Refund(ctx, paymentID, req.IdempotencyKey)
	if err != nil {
		return nil, apperrors.ToStatus(err)
	}

	return &paymentv1.RefundResponse{
		Status: statusToProto(payment.Status),
	}, nil
}

func (h *PaymentHandler) GetRefund(ctx context.Context, req *paymentv1.GetRefundRequest) (*paymentv1.GetRefundResponse, error) {
	authUserID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user_id in context")
	}
	role, _ := middleware.GetRole(ctx)

	refundID, err := uuid.Parse(req.RefundId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid refund_id")
	}
	refund, err := h.usecase.GetRefund(ctx, refundID)
	if err != nil {
		return nil, apperrors.ToStatus(err)
	}

	payment, err := h.usecase.GetByID(ctx, refund.PaymentID)
	if err != nil {
		return nil, apperrors.ToStatus(err)
	}

	if role != auth.RoleAdmin && payment.UserID.String() != authUserID {
		return nil, status.Error(codes.PermissionDenied, "refund does not belong to user")
	}

	return &paymentv1.GetRefundResponse{
		Refund: refundToProto(refund),
	}, nil
}

func (h *PaymentHandler) ListRefunds(ctx context.Context, req *paymentv1.ListRefundsRequest) (*paymentv1.ListRefundsResponse, error) {
	authUserID, ok := middleware.GetUserID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user_id in context")
	}
	role, _ := middleware.GetRole(ctx)

	paymentID, err := uuid.Parse(req.PaymentId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid payment_id")
	}

	payment, err := h.usecase.GetByID(ctx, paymentID)
	if err != nil {
		return nil, apperrors.ToStatus(err)
	}

	if role != auth.RoleAdmin && payment.UserID.String() != authUserID {
		return nil, status.Error(codes.PermissionDenied, "payment does not belong to user")
	}

	refunds, err := h.usecase.ListRefunds(ctx, paymentID)
	if err != nil {
		return nil, apperrors.ToStatus(err)
	}
	protoRefunds := make([]*paymentv1.Refund, len(refunds))
	for i, r := range refunds {
		protoRefunds[i] = refundToProto(r)
	}
	return &paymentv1.ListRefundsResponse{Refunds: protoRefunds}, nil
}

func refundToProto(r *domain.Refund) *paymentv1.Refund {
	if r == nil {
		return nil
	}
	return &paymentv1.Refund{
		Id:        r.ID.String(),
		PaymentId: r.PaymentID.String(),
		Amount:    r.Amount,
		Reason:    r.Reason,
		Status:    statusToProto(r.Status),
		CreatedAt: r.CreatedAt.Format(time.RFC3339),
	}
}
