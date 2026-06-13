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
	"go.uber.org/zap"
)

const (
	indexName          = "products"
	DefaultCallTimeout = 5 * time.Second
)

type ProductES struct {
	client *elastic.Client
	log    *zap.Logger
}

func NewProductES(client *elastic.Client, log *zap.Logger) repository.ProductSearchRepository {
	return &ProductES{client: client, log: log}
}

func (r *ProductES) Index(ctx context.Context, product *domain.Product) error {
	ctx, cancel := context.WithTimeout(ctx, DefaultCallTimeout)
	defer cancel()

	_, err := r.client.Index().
		Index(indexName).
		Id(product.ID.String()).
		BodyJson(map[string]interface{}{
			"id":          product.ID.String(),
			"name":        product.Name,
			"description": product.Description,
			"price_cents": product.Price,
			"categories":  product.Categories,
			"created_at":  product.CreatedAt.Format(time.RFC3339),
		}).
		Do(ctx)
	if err != nil {
		return fmt.Errorf("index product: %w", err)
	}
	return nil
}

func (r *ProductES) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, DefaultCallTimeout)
	defer cancel()

	_, err := r.client.Delete().
		Index(indexName).
		Id(id.String()).
		Do(ctx)
	if err != nil && !elastic.IsNotFound(err) {
		return fmt.Errorf("delete product from index: %w", err)
	}
	return nil
}

func (r *ProductES) EnsureIndex(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, DefaultCallTimeout)
	defer cancel()

	exists, err := r.client.IndexExists(indexName).Do(ctx)
	if err != nil {
		return fmt.Errorf("check index exists: %w", err)
	}
	if exists {
		return nil
	}

	mapping := map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": map[string]interface{}{
			"properties": map[string]interface{}{
				"id":          map[string]string{"type": "keyword"},
				"name":        map[string]string{"type": "text"},
				"description": map[string]string{"type": "text"},
				"price_cents": map[string]string{"type": "long"},
				"categories":  map[string]string{"type": "keyword"},
				"created_at":  map[string]string{"type": "date"},
			},
		},
	}

	_, err = r.client.CreateIndex(indexName).BodyJson(mapping).Do(ctx)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	r.log.Info("created elasticsearch index", zap.String("index", indexName))
	return nil
}

func (r *ProductES) Search(ctx context.Context, query string, page, pageSize int) ([]*domain.Product, int, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultCallTimeout)
	defer cancel()

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
		product, err := r.mapToProduct(source)
		if err != nil {
			return nil, 0, fmt.Errorf("map hit to product: %w", err)
		}
		products = append(products, product)
	}

	total := int(res.TotalHits())
	return products, total, nil
}

func (r *ProductES) mapToProduct(source map[string]interface{}) (*domain.Product, error) {
	if source == nil {
		r.log.Warn("mapToProduct: source is nil")
		return nil, fmt.Errorf("source is nil")
	}

	idStr, ok := source["id"].(string)
	if !ok || idStr == "" {
		r.log.Warn("mapToProduct: missing or invalid id", zap.Any("value", source["id"]))
		return nil, fmt.Errorf("missing or invalid id")
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		r.log.Warn("mapToProduct: invalid id", zap.String("value", idStr), zap.Error(err))
		return nil, fmt.Errorf("parse id: %w", err)
	}

	name, ok := source["name"].(string)
	if !ok {
		r.log.Warn("mapToProduct: missing or invalid name", zap.Any("value", source["name"]))
	}

	description, ok := source["description"].(string)
	if !ok {
		r.log.Warn("mapToProduct: missing or invalid description", zap.Any("value", source["description"]))
	}

	var price int64
	if v, ok := source["price_cents"].(float64); ok {
		price = int64(v)
	} else {
		r.log.Warn("mapToProduct: missing or invalid price_cents", zap.Any("value", source["price_cents"]))
	}

	var categories []string
	if arr, ok := source["categories"].([]interface{}); ok {
		for i, c := range arr {
			if s, ok := c.(string); ok {
				categories = append(categories, s)
			} else {
				r.log.Warn("mapToProduct: invalid category element", zap.Int("index", i), zap.Any("value", c))
			}
		}
	} else {
		r.log.Warn("mapToProduct: missing or invalid categories", zap.Any("value", source["categories"]))
	}

	createdAt := time.Time{}
	if tStr, ok := source["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, tStr); err == nil {
			createdAt = t
		} else {
			r.log.Warn("mapToProduct: invalid created_at", zap.String("value", tStr), zap.Error(err))
		}
	} else {
		r.log.Warn("mapToProduct: missing or invalid created_at", zap.Any("value", source["created_at"]))
	}

	return &domain.Product{
		ID:          id,
		Name:        name,
		Description: description,
		Price:       price,
		Categories:  categories,
		CreatedAt:   createdAt,
	}, nil
}
