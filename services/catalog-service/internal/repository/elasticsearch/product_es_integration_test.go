//go:build integration

package elasticsearch

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/catalog-service/internal/domain"
	"github.com/google/uuid"
	"github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

func startElasticsearch(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "docker.elastic.co/elasticsearch/elasticsearch:8.11.0",
		ExposedPorts: []string{"9200/tcp"},
		Env: map[string]string{
			"discovery.type":         "single-node",
			"xpack.security.enabled": "false",
		},
		WaitingFor: wait.ForHTTP("/_cluster/health").WithPort("9200/tcp"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = container.Terminate(ctx)
	})

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "9200")
	require.NoError(t, err)

	return fmt.Sprintf("http://%s:%s", host, port.Port())
}

func TestProductES_Integration_IndexSearchDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	esURL := startElasticsearch(t)
	client, err := elastic.NewClient(elastic.SetURL(esURL), elastic.SetSniff(false))
	require.NoError(t, err)

	ctx := context.Background()
	repo := NewProductES(client, zap.NewNop())

	require.NoError(t, repo.EnsureIndex(ctx))

	id := uuid.New()
	product := &domain.Product{
		ID:          id,
		Name:        "Elasticsearch Product",
		Description: "Searchable product",
		Price:       9999,
		Categories:  []string{"electronics"},
		CreatedAt:   time.Now().UTC(),
	}

	require.NoError(t, repo.Index(ctx, product))
	_, err = client.Refresh(indexName).Do(ctx)
	require.NoError(t, err)

	products, total, err := repo.Search(ctx, "Elasticsearch", 1, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, 1)
	assert.GreaterOrEqual(t, len(products), 1)

	found := false
	for _, p := range products {
		if p.ID == id {
			found = true
			assert.Equal(t, product.Name, p.Name)
			assert.Equal(t, product.Price, p.Price)
		}
	}
	assert.True(t, found, "expected to find indexed product")

	require.NoError(t, repo.Delete(ctx, id))
	_, err = client.Refresh(indexName).Do(ctx)
	require.NoError(t, err)

	products, total, err = repo.Search(ctx, "Elasticsearch", 1, 10)
	require.NoError(t, err)
	for _, p := range products {
		assert.NotEqual(t, id, p.ID)
	}
	_ = total
}
