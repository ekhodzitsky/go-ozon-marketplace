package app_test

import (
	"net"
	"testing"
	"time"

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

func TestNew(t *testing.T) {
	cfg := &config.Config{
		HTTPPort:    "8080",
		MetricsPort: 9080,
		JWTSecret:   testJWTSecret,
	}
	a := app.New(cfg)
	require.NotNil(t, a)
}

func TestRun_MissingTLSConfig(t *testing.T) {
	cfg := &config.Config{
		HTTPPort:             "18080",
		MetricsPort:          19080,
		UserServiceAddr:      "localhost:50051",
		CatalogServiceAddr:   "localhost:50052",
		OrderServiceAddr:     "localhost:50055",
		InventoryServiceAddr: "localhost:50053",
		PaymentServiceAddr:   "localhost:50054",
		AnalyticsServiceAddr: "localhost:50056",
		RedisAddr:            "localhost:6379",
		JWTSecret:            testJWTSecret,
		DefaultCallTimeout:   5 * time.Second,
		DefaultQueryTimeout:  3 * time.Second,
	}

	a := app.New(cfg)
	err := a.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no CERT_PATH configured")
}

func TestRun_InsecureSkipTLS_ReturnsWithoutDialErrors(t *testing.T) {
	if redisReachable("localhost:6379") {
		t.Skip("local Redis is reachable; this test expects Redis to be unavailable")
	}

	cfg := &config.Config{
		HTTPPort:             "18081",
		MetricsPort:          19081,
		UserServiceAddr:      "localhost:50051",
		CatalogServiceAddr:   "localhost:50052",
		OrderServiceAddr:     "localhost:50055",
		InventoryServiceAddr: "localhost:50053",
		PaymentServiceAddr:   "localhost:50054",
		AnalyticsServiceAddr: "localhost:50056",
		RedisAddr:            "localhost:6379",
		JWTSecret:            testJWTSecret,
		InsecureSkipTLS:      true,
		DefaultCallTimeout:   5 * time.Second,
		DefaultQueryTimeout:  3 * time.Second,
	}

	a := app.New(cfg)
	err := a.Run()
	// The Redis connection will fail because there is no Redis server.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis")
}
