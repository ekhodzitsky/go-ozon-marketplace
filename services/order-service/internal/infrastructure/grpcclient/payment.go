package grpcclient

import (
	"context"
	"time"

	paymentv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/payment/v1"
)

type PaymentClient struct {
	client      paymentv1.PaymentServiceClient
	callTimeout time.Duration
}

func NewPaymentClient(client paymentv1.PaymentServiceClient, callTimeout time.Duration) *PaymentClient {
	return &PaymentClient{client: client, callTimeout: callTimeout}
}

func (c *PaymentClient) ProcessPayment(ctx context.Context, orderID string, amountCents int64, idempotencyKey string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	resp, err := c.client.ProcessPayment(ctx, &paymentv1.ProcessPaymentRequest{
		OrderId:        orderID,
		AmountCents:    amountCents,
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		return "", err
	}
	return resp.PaymentId, nil
}

func (c *PaymentClient) Refund(ctx context.Context, paymentID string, idempotencyKey string) error {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	_, err := c.client.Refund(ctx, &paymentv1.RefundRequest{PaymentId: paymentID, IdempotencyKey: idempotencyKey})
	return err
}
