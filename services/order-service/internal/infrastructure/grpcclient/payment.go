package grpcclient

import (
	"context"

	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
	"google.golang.org/grpc"
)

type PaymentClient struct {
	client paymentv1.PaymentServiceClient
}

func NewPaymentClient(conn *grpc.ClientConn) *PaymentClient {
	return &PaymentClient{client: paymentv1.NewPaymentServiceClient(conn)}
}

func (c *PaymentClient) ProcessPayment(ctx context.Context, orderID, userID string, amount float64) (string, error) {
	resp, err := c.client.ProcessPayment(ctx, &paymentv1.ProcessPaymentRequest{
		OrderId: orderID,
		UserId:  userID,
		Amount:  amount,
	})
	if err != nil {
		return "", err
	}
	return resp.PaymentId, nil
}

func (c *PaymentClient) Refund(ctx context.Context, paymentID string) error {
	_, err := c.client.Refund(ctx, &paymentv1.RefundRequest{PaymentId: paymentID})
	return err
}
