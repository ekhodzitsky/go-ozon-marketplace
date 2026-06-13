package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/services/api-gateway/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-must-be-long-enough!")
	// Clear any env vars that could interfere with defaults.
	for _, key := range []string{
		"USER_SERVICE_ADDR", "CATALOG_SERVICE_ADDR", "ORDER_SERVICE_ADDR",
		"INVENTORY_SERVICE_ADDR", "PAYMENT_SERVICE_ADDR", "ANALYTICS_SERVICE_ADDR",
		"PORT", "METRICS_PORT", "LOG_LEVEL", "LOG_FORMAT", "OTEL_EXPORTER_OTLP_ENDPOINT",
		"REDIS_ADDR", "RATE_LIMIT_RPS", "RATE_LIMIT_WINDOW", "RATE_LIMIT_USER_RPS",
		"RATE_LIMIT_ADMIN_RPS", "TRUSTED_PROXIES", "MAX_BODY_SIZE_BYTES",
		"DEFAULT_CALL_TIMEOUT", "DEFAULT_QUERY_TIMEOUT", "CERT_PATH",
		"INSECURE_SKIP_TLS", "CORS_ALLOWED_ORIGINS",
	} {
		t.Setenv(key, "")
	}

	cfg := config.Load()

	assert.Equal(t, "localhost:50051", cfg.UserServiceAddr)
	assert.Equal(t, "localhost:50052", cfg.CatalogServiceAddr)
	assert.Equal(t, "localhost:50055", cfg.OrderServiceAddr)
	assert.Equal(t, "localhost:50053", cfg.InventoryServiceAddr)
	assert.Equal(t, "localhost:50054", cfg.PaymentServiceAddr)
	assert.Equal(t, "localhost:50056", cfg.AnalyticsServiceAddr)
	assert.Equal(t, "8080", cfg.HTTPPort)
	assert.Equal(t, 9080, cfg.MetricsPort)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "http://localhost:4318", cfg.OTELExporterOTLPEndpoint)
	assert.Equal(t, "localhost:6379", cfg.RedisAddr)
	assert.Equal(t, 10, cfg.RateLimitRPS)
	assert.Equal(t, time.Second, cfg.RateLimitWindow)
	assert.Equal(t, 100, cfg.RateLimitUserRPS)
	assert.Equal(t, 1000, cfg.RateLimitAdminRPS)
	assert.Empty(t, cfg.TrustedProxies)
	assert.Equal(t, int64(1<<20), cfg.MaxBodySizeBytes)
	assert.Equal(t, 5*time.Second, cfg.DefaultCallTimeout)
	assert.Equal(t, 3*time.Second, cfg.DefaultQueryTimeout)
	assert.Empty(t, cfg.CertPath)
	assert.False(t, cfg.InsecureSkipTLS)
	assert.Equal(t, "test-secret-must-be-long-enough!", cfg.JWTSecret)
	assert.Empty(t, cfg.CORSAllowedOrigins)
}

func TestLoad_EnvOverrides(t *testing.T) {
	t.Setenv("USER_SERVICE_ADDR", "user:50051")
	t.Setenv("CATALOG_SERVICE_ADDR", "catalog:50052")
	t.Setenv("ORDER_SERVICE_ADDR", "order:50055")
	t.Setenv("INVENTORY_SERVICE_ADDR", "inventory:50053")
	t.Setenv("PAYMENT_SERVICE_ADDR", "payment:50054")
	t.Setenv("ANALYTICS_SERVICE_ADDR", "analytics:50056")
	t.Setenv("PORT", "9090")
	t.Setenv("METRICS_PORT", "9091")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "console")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://otel:4318")
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("RATE_LIMIT_RPS", "20")
	t.Setenv("RATE_LIMIT_WINDOW", "2s")
	t.Setenv("RATE_LIMIT_USER_RPS", "200")
	t.Setenv("RATE_LIMIT_ADMIN_RPS", "2000")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8, 172.16.0.0/12")
	t.Setenv("MAX_BODY_SIZE_BYTES", "2097152")
	t.Setenv("DEFAULT_CALL_TIMEOUT", "10s")
	t.Setenv("DEFAULT_QUERY_TIMEOUT", "5s")
	t.Setenv("CERT_PATH", "/certs")
	t.Setenv("INSECURE_SKIP_TLS", "true")
	t.Setenv("JWT_SECRET", "another-secret-must-be-long-enough")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com, https://admin.example.com")

	cfg := config.Load()

	assert.Equal(t, "user:50051", cfg.UserServiceAddr)
	assert.Equal(t, "catalog:50052", cfg.CatalogServiceAddr)
	assert.Equal(t, "order:50055", cfg.OrderServiceAddr)
	assert.Equal(t, "inventory:50053", cfg.InventoryServiceAddr)
	assert.Equal(t, "payment:50054", cfg.PaymentServiceAddr)
	assert.Equal(t, "analytics:50056", cfg.AnalyticsServiceAddr)
	assert.Equal(t, "9090", cfg.HTTPPort)
	assert.Equal(t, 9091, cfg.MetricsPort)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "console", cfg.LogFormat)
	assert.Equal(t, "http://otel:4318", cfg.OTELExporterOTLPEndpoint)
	assert.Equal(t, "redis:6379", cfg.RedisAddr)
	assert.Equal(t, 20, cfg.RateLimitRPS)
	assert.Equal(t, 2*time.Second, cfg.RateLimitWindow)
	assert.Equal(t, 200, cfg.RateLimitUserRPS)
	assert.Equal(t, 2000, cfg.RateLimitAdminRPS)
	assert.Equal(t, []string{"10.0.0.0/8", "172.16.0.0/12"}, cfg.TrustedProxies)
	assert.Equal(t, int64(2097152), cfg.MaxBodySizeBytes)
	assert.Equal(t, 10*time.Second, cfg.DefaultCallTimeout)
	assert.Equal(t, 5*time.Second, cfg.DefaultQueryTimeout)
	assert.Equal(t, "/certs", cfg.CertPath)
	assert.True(t, cfg.InsecureSkipTLS)
	assert.Equal(t, "another-secret-must-be-long-enough", cfg.JWTSecret)
	assert.Equal(t, []string{"https://app.example.com", "https://admin.example.com"}, cfg.CORSAllowedOrigins)
}

func TestLoad_METRICS_PORT_DerivedFromPORT(t *testing.T) {
	t.Setenv("PORT", "7070")
	t.Setenv("METRICS_PORT", "")

	cfg := config.Load()

	assert.Equal(t, "7070", cfg.HTTPPort)
	assert.Equal(t, 8070, cfg.MetricsPort)
}

func TestLoad_InvalidValuesFallBackToDefaults(t *testing.T) {
	t.Setenv("PORT", "not-a-number")
	t.Setenv("METRICS_PORT", "also-not-a-number")
	t.Setenv("RATE_LIMIT_RPS", "bad")
	t.Setenv("RATE_LIMIT_WINDOW", "bad")
	t.Setenv("MAX_BODY_SIZE_BYTES", "bad")
	t.Setenv("DEFAULT_CALL_TIMEOUT", "bad")
	t.Setenv("DEFAULT_QUERY_TIMEOUT", "bad")
	t.Setenv("INSECURE_SKIP_TLS", "not-a-bool")

	cfg := config.Load()

	assert.Equal(t, "8080", cfg.HTTPPort)
	assert.Equal(t, 8080+1000, cfg.MetricsPort)
	assert.Equal(t, 10, cfg.RateLimitRPS)
	assert.Equal(t, time.Second, cfg.RateLimitWindow)
	assert.Equal(t, int64(1<<20), cfg.MaxBodySizeBytes)
	assert.Equal(t, 5*time.Second, cfg.DefaultCallTimeout)
	assert.Equal(t, 3*time.Second, cfg.DefaultQueryTimeout)
	assert.False(t, cfg.InsecureSkipTLS)
}

func TestLoad_ParseSlicesWithEmptyEntries(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8,, 172.16.0.0/12, ")
	t.Setenv("CORS_ALLOWED_ORIGINS", ",https://app.example.com,")

	cfg := config.Load()

	assert.Equal(t, []string{"10.0.0.0/8", "172.16.0.0/12"}, cfg.TrustedProxies)
	assert.Equal(t, []string{"https://app.example.com"}, cfg.CORSAllowedOrigins)
}

// TestLoad uses os environment, so we restore after the subtest.
func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
