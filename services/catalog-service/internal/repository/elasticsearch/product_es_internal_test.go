package elasticsearch

import (
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestProductES() *ProductES {
	return &ProductES{log: zap.NewNop()}
}

func TestProductES_mapToProduct_Success(t *testing.T) {
	es := newTestProductES()
	id := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)

	source := map[string]interface{}{
		"id":          id.String(),
		"name":        "Product",
		"description": "Description",
		"price_cents": float64(1500),
		"categories":  []interface{}{"cat1", "cat2"},
		"created_at":  now,
	}

	product, err := es.mapToProduct(source)
	require.NoError(t, err)
	assert.Equal(t, id, product.ID)
	assert.Equal(t, "Product", product.Name)
	assert.Equal(t, "Description", product.Description)
	assert.Equal(t, int64(1500), product.Price)
	assert.Equal(t, []string{"cat1", "cat2"}, product.Categories)
}

func TestProductES_mapToProduct_NilSource(t *testing.T) {
	es := newTestProductES()
	_, err := es.mapToProduct(nil)
	require.Error(t, err)
}

func TestProductES_mapToProduct_MissingID(t *testing.T) {
	es := newTestProductES()
	_, err := es.mapToProduct(map[string]interface{}{"name": "Product"})
	require.Error(t, err)
}

func TestProductES_mapToProduct_InvalidID(t *testing.T) {
	es := newTestProductES()
	_, err := es.mapToProduct(map[string]interface{}{"id": "not-a-uuid"})
	require.Error(t, err)
}

func TestProductES_mapToProduct_PartialFields(t *testing.T) {
	es := newTestProductES()
	id := uuid.New()

	source := map[string]interface{}{
		"id":          id.String(),
		"name":        "Product",
		"price_cents": float64(0),
	}

	product, err := es.mapToProduct(source)
	require.NoError(t, err)
	assert.Equal(t, id, product.ID)
	assert.Equal(t, "Product", product.Name)
	assert.Equal(t, int64(0), product.Price)
	assert.Empty(t, product.Categories)
	assert.True(t, product.CreatedAt.IsZero())
}

func TestProductES_mapToProduct_InvalidCategoryElement(t *testing.T) {
	es := newTestProductES()
	id := uuid.New()

	source := map[string]interface{}{
		"id":         id.String(),
		"categories": []interface{}{"cat1", 42},
	}

	product, err := es.mapToProduct(source)
	require.NoError(t, err)
	assert.Equal(t, domain.Product{ID: id, Categories: []string{"cat1"}}, *product)
}
