// Package config provides configuration management for the alerting-system.
package config

import (
	"os"
	"strconv"
)

const (
	// DefaultWebhookMaxPayloadSize is the default max payload size for webhook endpoints (1MB).
	DefaultWebhookMaxPayloadSize int64 = 1 << 20 // 1048576 bytes

	// DefaultAdminMaxPayloadSize is the default max payload size for admin endpoints (100KB).
	DefaultAdminMaxPayloadSize int64 = 100 * 1024 // 102400 bytes

	// DefaultGRPCMaxMessageSize is the default max message size for gRPC (4MB).
	DefaultGRPCMaxMessageSize int = 4 << 20 // 4194304 bytes
)

// Config holds the application configuration.
type Config struct {
	// Port is the HTTP server port.
	Port string

	// WebhookMaxPayloadSize is the maximum payload size for webhook endpoints in bytes.
	WebhookMaxPayloadSize int64

	// AdminMaxPayloadSize is the maximum payload size for admin endpoints in bytes.
	AdminMaxPayloadSize int64

	// GRPCMaxMessageSize is the maximum message size for gRPC in bytes.
	GRPCMaxMessageSize int
}

// Load loads configuration from environment variables with defaults.
func Load() *Config {
	cfg := &Config{
		Port:                  getEnvOrDefault("PORT", "8080"),
		WebhookMaxPayloadSize: getEnvInt64OrDefault("WEBHOOK_MAX_PAYLOAD_SIZE", DefaultWebhookMaxPayloadSize),
		AdminMaxPayloadSize:   getEnvInt64OrDefault("ADMIN_MAX_PAYLOAD_SIZE", DefaultAdminMaxPayloadSize),
		GRPCMaxMessageSize:    getEnvIntOrDefault("GRPC_MAX_MESSAGE_SIZE", DefaultGRPCMaxMessageSize),
	}

	return cfg
}

// getEnvOrDefault returns the environment variable value or the default if not set.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt64OrDefault returns the environment variable value as int64 or the default if not set or invalid.
func getEnvInt64OrDefault(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getEnvIntOrDefault returns the environment variable value as int or the default if not set or invalid.
func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}
