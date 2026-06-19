package config

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrMissingJWTSecret     = errors.New("JWT_SECRET is required")
	ErrJWTSecretTooShort    = errors.New("JWT_SECRET is too short")
	ErrMissingPostgresDSN   = errors.New("POSTGRES_DSN is required")
	ErrInvalidPostgresDSN   = errors.New("POSTGRES_DSN is invalid")
	ErrMissingKafkaBrokers  = errors.New("KAFKA_BROKERS is required")
	ErrMissingKafkaTopics   = errors.New("KAFKA_TOPICS is required")
	ErrMissingClickHouseDSN = errors.New("CLICKHOUSE_DSN is required")
	ErrInvalidClickHouseDSN = errors.New("CLICKHOUSE_DSN is invalid")
)

// ValidateJWTSecret checks that the JWT secret is present and meets the minimum length.
func ValidateJWTSecret(secret string, minLen int) error {
	if secret == "" {
		return ErrMissingJWTSecret
	}
	if len(secret) < minLen {
		return fmt.Errorf("%w: got %d characters, need at least %d", ErrJWTSecretTooShort, len(secret), minLen)
	}
	return nil
}

// ValidatePostgresDSN checks that the Postgres DSN is present and parseable.
func ValidatePostgresDSN(dsn string) error {
	if strings.TrimSpace(dsn) == "" {
		return ErrMissingPostgresDSN
	}
	if !strings.HasPrefix(dsn, "postgres://") && !strings.HasPrefix(dsn, "postgresql://") {
		return fmt.Errorf("%w: must start with postgres:// or postgresql://", ErrInvalidPostgresDSN)
	}
	if _, err := pgxpool.ParseConfig(dsn); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPostgresDSN, err)
	}
	return nil
}

// ValidateKafkaBrokers checks that at least one Kafka broker is configured.
func ValidateKafkaBrokers(brokers []string) error {
	if len(brokers) == 0 {
		return ErrMissingKafkaBrokers
	}
	return nil
}

// ValidateKafkaTopics checks that at least one Kafka topic is configured.
func ValidateKafkaTopics(topics []string) error {
	if len(topics) == 0 {
		return ErrMissingKafkaTopics
	}
	return nil
}

// ValidateClickHouseAddr checks that the ClickHouse address is present and has a port.
func ValidateClickHouseAddr(addr string) error {
	if addr == "" {
		return ErrMissingClickHouseDSN
	}
	if !strings.Contains(addr, ":") {
		return fmt.Errorf("%w: must be in host:port form", ErrInvalidClickHouseDSN)
	}
	return nil
}

// ValidateSMTP checks that SMTP settings are consistent when SMTP_HOST is set.
func ValidateSMTP(host string, port int, from string) error {
	if host == "" {
		return nil
	}
	if port <= 0 || port > 65535 {
		return fmt.Errorf("SMTP_PORT must be a valid port number")
	}
	if _, err := mail.ParseAddress(from); err != nil {
		return fmt.Errorf("SMTP_FROM must be a valid email address")
	}
	return nil
}
