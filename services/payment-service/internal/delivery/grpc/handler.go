package grpc

import (
	"context"
	"errors"

	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/payment-service/internal/usecase"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
)

type PaymentHandler struct {
	paymentv1.UnimplementedPaymentServiceServer
	usecase *usecase.PaymentUsecase
}

func NewPaymentHandler(uc *usecase.PaymentUsecase) *PaymentHandler {
	return &PaymentHandler{usecase: uc}
}

func (h *PaymentHandler) ProcessPayment(ctx context.Context, req *paymentv1.ProcessPaymentRequest) (*paymentv1.ProcessPaymentResponse, error) {
	orderID, err := uuid.Parse(req.OrderId)
	if err != nil {
		return nil, err
	}
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, err
	}

	payment, err := h.usecase.ProcessPayment(ctx, orderID, userID, req.Amount)
	if err != nil {
		return nil, err
	}

	return &paymentv1.ProcessPaymentResponse{
		PaymentId: payment.ID.String(),
		Status:    payment.Status,
	}, nil
}

func (h *PaymentHandler) Refund(ctx context.Context, req *paymentv1.RefundRequest) (*paymentv1.RefundResponse, error) {
	paymentID, err := uuid.Parse(req.PaymentId)
	if err != nil {
		return nil, err
	}

	payment, err := h.usecase.Refund(ctx, paymentID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "refund: %v", err)
	}

	return &paymentv1.RefundResponse{
		Status: payment.Status,
	}, nil
}
