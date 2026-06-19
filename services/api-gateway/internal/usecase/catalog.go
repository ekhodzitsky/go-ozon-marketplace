package usecase

import (
	"context"
	"fmt"
	"math"
	"time"

	catalogv1 "github.com/ekhodzitsky/go-ozon-marketplace/api/gen/go/catalog/v1"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/graph/model"
	"github.com/google/uuid"
)

func CreateProduct(ctx context.Context, client catalogv1.CatalogServiceClient, name, description string, price float64, categories []string, timeout time.Duration) (string, error) {
	if _, err := requireAuth(ctx); err != nil {
		return "", err
	}
	if !isAdmin(ctx) {
		return "", fmt.Errorf("forbidden")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := client.CreateProduct(ctx, &catalogv1.CreateProductRequest{
		Name:           name,
		Description:    description,
		PriceCents:     dollarsToCents(price),
		Categories:     categories,
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		return "", err
	}
	return resp.ProductId, nil
}

func GetProduct(ctx context.Context, client catalogv1.CatalogServiceClient, id string, timeout time.Duration) (*model.Product, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := client.GetProduct(ctx, &catalogv1.GetProductRequest{ProductId: id})
	if err != nil {
		return nil, err
	}
	return protoProductToModel(resp.Product), nil
}

func SearchProducts(ctx context.Context, client catalogv1.CatalogServiceClient, query string, page, pageSize *int32, timeout time.Duration) (*model.ProductConnection, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req := &catalogv1.SearchProductsRequest{Query: query, Page: 1, PageSize: 10}
	if page != nil && *page > 0 {
		req.Page = *page
	}
	if pageSize != nil && *pageSize > 0 {
		req.PageSize = *pageSize
	}
	resp, err := client.SearchProducts(ctx, req)
	if err != nil {
		return nil, err
	}
	products := make([]*model.Product, len(resp.Products))
	for i, p := range resp.Products {
		products[i] = protoProductToModel(p)
	}
	return &model.ProductConnection{Products: products, Total: resp.Total}, nil
}

func dollarsToCents(dollars float64) int64 {
	return int64(math.Round(dollars * 100))
}

func centsToDollars(cents int64) float64 {
	return float64(cents) / 100.0
}

func protoProductToModel(p *catalogv1.Product) *model.Product {
	if p == nil {
		return nil
	}
	return &model.Product{
		ID:          p.ProductId,
		Name:        p.Name,
		Description: p.Description,
		Price:       centsToDollars(p.PriceCents),
		Categories:  p.Categories,
		CreatedAt:   p.CreatedAt,
	}
}
