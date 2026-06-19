package usecase

import (
	"context"
	"time"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph/model"
)

func GetInventory(ctx context.Context, client inventoryv1.InventoryServiceClient, productID string, timeout time.Duration) (*model.Inventory, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := client.GetStock(ctx, &inventoryv1.GetStockRequest{ProductId: productID})
	if err != nil {
		return nil, err
	}
	return &model.Inventory{
		ProductID: productID,
		Available: resp.Available,
		Reserved:  resp.Reserved,
	}, nil
}
