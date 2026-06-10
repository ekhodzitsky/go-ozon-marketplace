package config

import (
	"fmt"
	"os"
	"strconv"
)

// MustGetEnv returns env var or panics if not set
func MustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %s is not set", key))
	}
	return v
}

// GetEnv returns env var or default
func GetEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// MustGetEnvInt returns env var as int or panics
func MustGetEnvInt(key string) int {
	v := MustGetEnv(key)
	n, err := strconv.Atoi(v)
	if err != nil {
		panic(fmt.Sprintf("invalid integer value for %s: %v", key, err))
	}
	return n
}

// GetEnvInt returns env var as int or default
func GetEnvInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}
