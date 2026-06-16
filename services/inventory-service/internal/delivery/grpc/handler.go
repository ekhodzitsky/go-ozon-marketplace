package grpc

import (
	"context"
	"errors"
	"time"

	inventoryv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/inventory/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/auth"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/validation"
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
	if err := middleware.RequireRole(ctx, auth.RoleService); err != nil {
		return nil, err
	}

	if err := validation.ValidateQuantity(req.Quantity); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if req.IdempotencyKey == "" {
		return nil, status.Error(codes.InvalidArgument, "missing idempotency_key")
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
	if err := middleware.RequireRole(ctx, auth.RoleService); err != nil {
		return nil, err
	}

	if err := validation.ValidateQuantity(req.Quantity); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if req.IdempotencyKey == "" {
		return nil, status.Error(codes.InvalidArgument, "missing idempotency_key")
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

func (h *InventoryHandler) GetLedger(ctx context.Context, req *inventoryv1.GetLedgerRequest) (*inventoryv1.GetLedgerResponse, error) {
	productID, err := uuid.Parse(req.ProductId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid product_id")
	}
	entries, err := h.usecase.GetLedger(ctx, productID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get ledger: %v", err)
	}
	protoEntries := make([]*inventoryv1.LedgerEntry, len(entries))
	for i, e := range entries {
		protoEntries[i] = &inventoryv1.LedgerEntry{
			Id:             e.ID.String(),
			ProductId:      e.ProductID.String(),
			QuantityChange: int32(e.QuantityChange),
			OperationType:  inventoryv1.LedgerOperation(inventoryv1.LedgerOperation_value[e.OperationType]),
			CreatedAt:      e.CreatedAt.Format(time.RFC3339),
		}
		if e.OrderID != nil {
			protoEntries[i].OrderId = e.OrderID.String()
		}
	}
	return &inventoryv1.GetLedgerResponse{Entries: protoEntries}, nil
}

func mapError(err error) error {
	s := apperrors.ToStatus(err)
	if s != nil {
		return s
	}
	return status.Error(codes.Internal, "internal error")
}
