package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/repository"
	"github.com/google/uuid"
	"github.com/olivere/elastic/v7"
)

const indexName = "products"

type ProductES struct {
	client *elastic.Client
}

func NewProductES(client *elastic.Client) repository.ProductSearchRepository {
	return &ProductES{client: client}
}

func (r *ProductES) Index(ctx context.Context, product *domain.Product) error {
	_, err := r.client.Index().
		Index(indexName).
		Id(product.ID.String()).
		BodyJson(map[string]interface{}{
			"id":          product.ID.String(),
			"name":        product.Name,
			"description": product.Description,
			"price":       product.Price,
			"stock":       product.Stock,
			"categories":  product.Categories,
			"created_at":  product.CreatedAt.Format(time.RFC3339),
		}).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("index product: %w", err)
	}
	return nil
}

func (r *ProductES) Search(ctx context.Context, query string, page, pageSize int) ([]*domain.Product, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	from := (page - 1) * pageSize

	q := elastic.NewMultiMatchQuery(query, "name", "description").
		Type("best_fields")

	res, err := r.client.Search(indexName).
		Query(q).
		From(from).
		Size(pageSize).
		Do(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("search products: %w", err)
	}

	products := make([]*domain.Product, 0, len(res.Hits.Hits))
	for _, hit := range res.Hits.Hits {
		source := make(map[string]interface{})
		if err := json.Unmarshal(hit.Source, &source); err != nil {
			return nil, 0, fmt.Errorf("unmarshal hit: %w", err)
		}
		product, err := mapToProduct(source)
		if err != nil {
			return nil, 0, fmt.Errorf("map hit to product: %w", err)
		}
		products = append(products, product)
	}

	total := int(res.TotalHits())
	return products, total, nil
}

func mapToProduct(source map[string]interface{}) (*domain.Product, error) {
	idStr, _ := source["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("parse id: %w", err)
	}

	name, _ := source["name"].(string)
	description, _ := source["description"].(string)

	var price float64
	if v, ok := source["price"].(float64); ok {
		price = v
	}

	var stock int
	if v, ok := source["stock"].(float64); ok {
		stock = int(v)
	}

	var categories []string
	if arr, ok := source["categories"].([]interface{}); ok {
		for _, c := range arr {
			if s, ok := c.(string); ok {
				categories = append(categories, s)
			}
		}
	}

	createdAt := time.Time{}
	if tStr, ok := source["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, tStr); err == nil {
			createdAt = t
		}
	}

	return &domain.Product{
		ID:          id,
		Name:        name,
		Description: description,
		Price:       price,
		Stock:       stock,
		Categories:  categories,
		CreatedAt:   createdAt,
	}, nil
}
