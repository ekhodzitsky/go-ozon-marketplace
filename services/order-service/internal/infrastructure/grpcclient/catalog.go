package grpcclient

import (
	"context"
	"time"

	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	"google.golang.org/grpc"
)

// CatalogClient is a thin client for catalog-service.
type CatalogClient interface {
	GetProduct(ctx context.Context, productID string) (*catalogv1.Product, error)
}

type catalogClient struct {
	client      catalogv1.CatalogServiceClient
	callTimeout time.Duration
}

func NewCatalogClient(conn *grpc.ClientConn, callTimeout time.Duration) CatalogClient {
	return &catalogClient{
		client:      catalogv1.NewCatalogServiceClient(conn),
		callTimeout: callTimeout,
	}
}

func (c *catalogClient) GetProduct(ctx context.Context, productID string) (*catalogv1.Product, error) {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	resp, err := c.client.GetProduct(ctx, &catalogv1.GetProductRequest{ProductId: productID})
	if err != nil {
		return nil, err
	}
	return resp.Product, nil
}
