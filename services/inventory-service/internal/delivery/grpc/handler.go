package grpc

import (
	"context"
	"errors"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/inventory-service/internal/usecase"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
)

type InventoryHandler struct {
	inventoryv1.UnimplementedInventoryServiceServer
	usecase usecase.InventoryUsecase
}

func NewInventoryHandler(uc usecase.InventoryUsecase) *InventoryHandler {
	return &InventoryHandler{usecase: uc}
}

func (h *InventoryHandler) Reserve(ctx context.Context, req *inventoryv1.ReserveRequest) (*inventoryv1.ReserveResponse, error) {
	if err := middleware.RequireRole(ctx, middleware.RoleService); err != nil {
		return nil, err
	}

	productID, err := uuid.Parse(req.ProductId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid product_id")
	}
	orderID, err := uuid.Parse(req.OrderId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_id")
	}
	if err := h.usecase.Reserve(ctx, productID, int(req.Quantity), orderID.String()); err != nil {
		return nil, mapError(err)
	}
	return &inventoryv1.ReserveResponse{Success: true}, nil
}

func (h *InventoryHandler) Release(ctx context.Context, req *inventoryv1.ReleaseRequest) (*inventoryv1.ReleaseResponse, error) {
	if err := middleware.RequireRole(ctx, middleware.RoleService); err != nil {
		return nil, err
	}

	productID, err := uuid.Parse(req.ProductId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid product_id")
	}
	orderID, err := uuid.Parse(req.OrderId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order_id")
	}
	if err := h.usecase.Release(ctx, productID, int(req.Quantity), orderID.String()); err != nil {
		return nil, mapError(err)
	}
	return &inventoryv1.ReleaseResponse{Success: true}, nil
}

func (h *InventoryHandler) GetStock(ctx context.Context, req *inventoryv1.GetStockRequest) (*inventoryv1.GetStockResponse, error) {
	productID, err := uuid.Parse(req.ProductId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid product_id")
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

func mapError(err error) error {
	s := apperrors.ToStatus(err)
	if s != nil {
		return s
	}
	return status.Error(codes.Internal, "internal error")
}
