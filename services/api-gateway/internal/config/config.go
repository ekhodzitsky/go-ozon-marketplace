package config

import (
	"strconv"
	"strings"
	"time"

	"github.com/ekhodzitsky/go-ozon-marketplace/pkg/config"
)

type Config struct {
	config.Base

	UserServiceAddr      string
	CatalogServiceAddr   string
	OrderServiceAddr     string
	InventoryServiceAddr string
	PaymentServiceAddr   string
	AnalyticsServiceAddr string
	HTTPPort             string
	MetricsPort          int
	RedisAddr            string
	RateLimitRPS         int
	RateLimitWindow      time.Duration
	RateLimitUserRPS     int
	RateLimitAdminRPS    int
	TrustedProxies       []string
	MaxBodySizeBytes     int64
	InsecureSkipTLS      bool
	CORSAllowedOrigins   []string
}

func Load() (*Config, error) {
	trusted := []string{}
	if v := config.GetEnv("TRUSTED_PROXIES", ""); v != "" {
		for _, s := range strings.Split(v, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				trusted = append(trusted, s)
			}
		}
	}
	corsOrigins := []string{}
	if v := config.GetEnv("CORS_ALLOWED_ORIGINS", ""); v != "" {
		for _, s := range strings.Split(v, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				corsOrigins = append(corsOrigins, s)
			}
		}
	}
	httpPort := config.GetEnvInt("PORT", 8080)
	base := config.LoadBase()
	return &Config{
		Base:                 base,
		UserServiceAddr:      config.GetEnv("USER_SERVICE_ADDR", "localhost:50051"),
		CatalogServiceAddr:   config.GetEnv("CATALOG_SERVICE_ADDR", "localhost:50052"),
		OrderServiceAddr:     config.GetEnv("ORDER_SERVICE_ADDR", "localhost:50055"),
		InventoryServiceAddr: config.GetEnv("INVENTORY_SERVICE_ADDR", "localhost:50053"),
		PaymentServiceAddr:   config.GetEnv("PAYMENT_SERVICE_ADDR", "localhost:50054"),
		AnalyticsServiceAddr: config.GetEnv("ANALYTICS_SERVICE_ADDR", "localhost:50056"),
		HTTPPort:             strconv.Itoa(httpPort),
		MetricsPort:          config.GetEnvInt("METRICS_PORT", httpPort+1000),
		RedisAddr:            config.GetEnv("REDIS_ADDR", "localhost:6379"),
		RateLimitRPS:         config.GetEnvInt("RATE_LIMIT_RPS", 10),
		RateLimitWindow:      config.GetEnvDuration("RATE_LIMIT_WINDOW", time.Second),
		RateLimitUserRPS:     config.GetEnvInt("RATE_LIMIT_USER_RPS", 100),
		RateLimitAdminRPS:    config.GetEnvInt("RATE_LIMIT_ADMIN_RPS", 1000),
		TrustedProxies:       trusted,
		MaxBodySizeBytes:     config.GetEnvInt64("MAX_BODY_SIZE_BYTES", 1<<20),
		InsecureSkipTLS:      parseBool(config.GetEnv("INSECURE_SKIP_TLS", "false")),
		CORSAllowedOrigins:   corsOrigins,
	}, nil
}

func parseBool(s string) bool {
	b, _ := strconv.ParseBool(strings.ToLower(strings.TrimSpace(s)))
	return b
}

func (c *Config) GetRedisAddr() string     { return c.RedisAddr }
func (c *Config) GetInsecureSkipTLS() bool { return c.InsecureSkipTLS }
