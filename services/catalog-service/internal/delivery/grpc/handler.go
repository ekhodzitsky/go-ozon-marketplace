package grpc

import (
	"context"
	"errors"
	"time"

	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/middleware"
	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/validation"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/usecase"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apperrors "github.com/ekhodzitsky/go-ozon-marketplace/pkg/errors"
)

type CatalogHandler struct {
	catalogv1.UnimplementedCatalogServiceServer
	usecase usecase.CatalogUsecase
}

func NewCatalogHandler(uc usecase.CatalogUsecase) *CatalogHandler {
	return &CatalogHandler{usecase: uc}
}

func (h *CatalogHandler) CreateProduct(ctx context.Context, req *catalogv1.CreateProductRequest) (*catalogv1.CreateProductResponse, error) {
	if err := validation.ValidateName(req.Name); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := validation.ValidatePrice(req.Price); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	id, err := h.usecase.CreateProduct(ctx, req.Name, req.Description, int64(req.Price*100), req.Categories)
	if err != nil {
		return nil, err
	}
	return &catalogv1.CreateProductResponse{ProductId: id.String()}, nil
}

func (h *CatalogHandler) GetProduct(ctx context.Context, req *catalogv1.GetProductRequest) (*catalogv1.GetProductResponse, error) {
	id, err := uuid.Parse(req.ProductId)
	if err != nil {
		return nil, err
	}
	product, err := h.usecase.GetProduct(ctx, id)
	if err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "get product: %v", err)
	}
	return &catalogv1.GetProductResponse{Product: toProtoProduct(product)}, nil
}

func (h *CatalogHandler) ListProducts(ctx context.Context, req *catalogv1.ListProductsRequest) (*catalogv1.ListProductsResponse, error) {
	if err := validation.ValidatePageSize(req.PageSize); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	products, total, err := h.usecase.ListProducts(ctx, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, err
	}
	return &catalogv1.ListProductsResponse{
		Products: toProtoProducts(products),
		Total:    int32(total),
	}, nil
}

func (h *CatalogHandler) SearchProducts(ctx context.Context, req *catalogv1.SearchProductsRequest) (*catalogv1.SearchProductsResponse, error) {
	if err := validation.ValidatePageSize(req.PageSize); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	products, total, err := h.usecase.SearchProducts(ctx, req.Query, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, err
	}
	return &catalogv1.SearchProductsResponse{
		Products: toProtoProducts(products),
		Total:    int32(total),
	}, nil
}

func (h *CatalogHandler) UpdateProduct(ctx context.Context, req *catalogv1.UpdateProductRequest) (*catalogv1.UpdateProductResponse, error) {
	if err := middleware.RequireRole(ctx, middleware.RoleAdmin); err != nil {
		return nil, err
	}

	if req.Name != "" {
		if err := validation.ValidateName(req.Name); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}
	if req.Price != 0 {
		if err := validation.ValidatePrice(req.Price); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	id, err := uuid.Parse(req.ProductId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid product_id")
	}

	if err := h.usecase.UpdateProduct(ctx, id, req.Name, req.Description, int64(req.Price*100), req.Categories); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "update product: %v", err)
	}

	return &catalogv1.UpdateProductResponse{Success: true}, nil
}

func (h *CatalogHandler) DeleteProduct(ctx context.Context, req *catalogv1.DeleteProductRequest) (*catalogv1.DeleteProductResponse, error) {
	if err := middleware.RequireRole(ctx, middleware.RoleAdmin); err != nil {
		return nil, err
	}

	id, err := uuid.Parse(req.ProductId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid product_id")
	}

	if err := h.usecase.DeleteProduct(ctx, id); err != nil {
		if errors.Is(err, apperrors.ErrNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "delete product: %v", err)
	}

	return &catalogv1.DeleteProductResponse{Success: true}, nil
}

func toProtoProduct(p *domain.Product) *catalogv1.Product {
	if p == nil {
		return nil
	}
	return &catalogv1.Product{
		ProductId:   p.ID.String(),
		Name:        p.Name,
		Description: p.Description,
		Price:       float64(p.Price),
		Categories:  p.Categories,
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
	}
}

func toProtoProducts(products []*domain.Product) []*catalogv1.Product {
	result := make([]*catalogv1.Product, len(products))
	for i, p := range products {
		result[i] = toProtoProduct(p)
	}
	return result
}
