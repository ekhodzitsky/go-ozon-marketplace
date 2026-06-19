package app_test

import (
	"net"
	"testing"
	"time"

	pkgconfig "github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/app"
	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testJWTSecret = "test-secret-must-be-long-enough!" // 32 bytes

// redisReachable reports whether a Redis server is listening on addr.
func redisReachable(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func requireRedis(t *testing.T) {
	t.Helper()
	if !redisReachable("localhost:6379") {
		t.Skip("local Redis is not reachable")
	}
}

func testConfig() *config.Config {
	return &config.Config{
		Base: pkgconfig.Base{
			JWTSecret:          testJWTSecret,
			DefaultCallTimeout: 5 * time.Second,
			DefaultQueryTimeout: 3 * time.Second,
		},
		HTTPPort:             "8080",
		MetricsPort:          9080,
		UserServiceAddr:      "localhost:50051",
		CatalogServiceAddr:   "localhost:50052",
		OrderServiceAddr:     "localhost:50055",
		InventoryServiceAddr: "localhost:50053",
		PaymentServiceAddr:   "localhost:50054",
		AnalyticsServiceAddr: "localhost:50056",
		RedisAddr:            "localhost:6379",
		InsecureSkipTLS:      true,
	}
}

func TestNew(t *testing.T) {
	requireRedis(t)

	a := app.New(testConfig())
	require.NotNil(t, a)
}

func TestNew_MissingTLSConfig(t *testing.T) {
	requireRedis(t)

	cfg := testConfig()
	cfg.InsecureSkipTLS = false
	cfg.CertPath = ""

	a := app.New(cfg)
	err := a.Err()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no CERT_PATH configured")
}

func TestNew_InsecureSkipTLS_NoRedis(t *testing.T) {
	if redisReachable("localhost:6379") {
		t.Skip("local Redis is reachable; this test expects Redis to be unavailable")
	}

	a := app.New(testConfig())
	err := a.Err()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis")
}
