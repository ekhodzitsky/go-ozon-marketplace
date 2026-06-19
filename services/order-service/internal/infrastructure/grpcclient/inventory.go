package grpcclient

import (
	"context"
	"time"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
)

type InventoryClient struct {
	client      inventoryv1.InventoryServiceClient
	callTimeout time.Duration
}

func NewInventoryClient(client inventoryv1.InventoryServiceClient, callTimeout time.Duration) *InventoryClient {
	return &InventoryClient{client: client, callTimeout: callTimeout}
}

func (c *InventoryClient) Reserve(ctx context.Context, productID string, quantity int32, orderID string, idempotencyKey string) error {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	_, err := c.client.Reserve(ctx, &inventoryv1.ReserveRequest{
		ProductId:      productID,
		Quantity:       quantity,
		OrderId:        orderID,
		IdempotencyKey: idempotencyKey,
	})
	return err
}

func (c *InventoryClient) Release(ctx context.Context, productID string, quantity int32, orderID string, idempotencyKey string) error {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	_, err := c.client.Release(ctx, &inventoryv1.ReleaseRequest{
		ProductId:      productID,
		Quantity:       quantity,
		OrderId:        orderID,
		IdempotencyKey: idempotencyKey,
	})
	return err
}
