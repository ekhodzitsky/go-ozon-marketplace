package grpcclient

import (
	"context"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	"google.golang.org/grpc"
)

type InventoryClient struct {
	client inventoryv1.InventoryServiceClient
}

func NewInventoryClient(conn *grpc.ClientConn) *InventoryClient {
	return &InventoryClient{client: inventoryv1.NewInventoryServiceClient(conn)}
}

func (c *InventoryClient) Reserve(ctx context.Context, productID string, quantity int32, orderID string) error {
	_, err := c.client.Reserve(ctx, &inventoryv1.ReserveRequest{
		ProductId: productID,
		Quantity:  quantity,
		OrderId:   orderID,
	})
	return err
}

func (c *InventoryClient) Release(ctx context.Context, productID string, quantity int32, orderID string) error {
	_, err := c.client.Release(ctx, &inventoryv1.ReleaseRequest{
		ProductId: productID,
		Quantity:  quantity,
		OrderId:   orderID,
	})
	return err
}
