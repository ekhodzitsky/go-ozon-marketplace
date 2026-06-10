package grpc

import (
	"context"
	"errors"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/usecase"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
)

type InventoryHandler struct {
	inventoryv1.UnimplementedInventoryServiceServer
	usecase *usecase.InventoryUsecase
}

func NewInventoryHandler(uc *usecase.InventoryUsecase) *InventoryHandler {
	return &InventoryHandler{usecase: uc}
}

func (h *InventoryHandler) Reserve(ctx context.Context, req *inventoryv1.ReserveRequest) (*inventoryv1.ReserveResponse, error) {
	productID, err := uuid.Parse(req.ProductId)
	if err != nil {
		return nil, err
	}
	if err := h.usecase.Reserve(ctx, productID, int(req.Quantity), req.OrderId); err != nil {
		return nil, err
	}
	return &inventoryv1.ReserveResponse{Success: true}, nil
}

func (h *InventoryHandler) Release(ctx context.Context, req *inventoryv1.ReleaseRequest) (*inventoryv1.ReleaseResponse, error) {
	productID, err := uuid.Parse(req.ProductId)
	if err != nil {
		return nil, err
	}
	if err := h.usecase.Release(ctx, productID, int(req.Quantity), req.OrderId); err != nil {
		return nil, err
	}
	return &inventoryv1.ReleaseResponse{Success: true}, nil
}

func (h *InventoryHandler) GetStock(ctx context.Context, req *inventoryv1.GetStockRequest) (*inventoryv1.GetStockResponse, error) {
	productID, err := uuid.Parse(req.ProductId)
	if err != nil {
		return nil, err
	}
	stock, err := h.usecase.GetStock(ctx, productID)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "get stock: %v", err)
	}
	return &inventoryv1.GetStockResponse{
		Available: int32(stock.Available),
		Reserved:  int32(stock.Reserved),
	}, nil
}
