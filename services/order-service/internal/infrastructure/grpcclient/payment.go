package grpcclient

import (
	"context"
	"time"

	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	"google.golang.org/grpc"
)

type PaymentClient struct {
	client      paymentv1.PaymentServiceClient
	callTimeout time.Duration
}

func NewPaymentClient(conn *grpc.ClientConn, callTimeout time.Duration) *PaymentClient {
	return &PaymentClient{client: paymentv1.NewPaymentServiceClient(conn), callTimeout: callTimeout}
}

func (c *PaymentClient) ProcessPayment(ctx context.Context, orderID, userID string, amount int64) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	resp, err := c.client.ProcessPayment(ctx, &paymentv1.ProcessPaymentRequest{
		OrderId: orderID,
		UserId:  userID,
		Amount:  float64(amount),
	})
	if err != nil {
		return "", err
	}
	return resp.PaymentId, nil
}

func (c *PaymentClient) Refund(ctx context.Context, paymentID string) error {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	_, err := c.client.Refund(ctx, &paymentv1.RefundRequest{PaymentId: paymentID})
	return err
}
