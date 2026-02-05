package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any existing env vars
	_ = os.Unsetenv("PORT")
	_ = os.Unsetenv("WEBHOOK_MAX_PAYLOAD_SIZE")
	_ = os.Unsetenv("ADMIN_MAX_PAYLOAD_SIZE")
	_ = os.Unsetenv("GRPC_MAX_MESSAGE_SIZE")

	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("expected default port '8080', got '%s'", cfg.Port)
	}

	if cfg.WebhookMaxPayloadSize != DefaultWebhookMaxPayloadSize {
		t.Errorf("expected default webhook payload size %d, got %d", DefaultWebhookMaxPayloadSize, cfg.WebhookMaxPayloadSize)
	}

	if cfg.AdminMaxPayloadSize != DefaultAdminMaxPayloadSize {
		t.Errorf("expected default admin payload size %d, got %d", DefaultAdminMaxPayloadSize, cfg.AdminMaxPayloadSize)
	}

	if cfg.GRPCMaxMessageSize != DefaultGRPCMaxMessageSize {
		t.Errorf("expected default gRPC message size %d, got %d", DefaultGRPCMaxMessageSize, cfg.GRPCMaxMessageSize)
	}
}

func TestLoad_CustomValues(t *testing.T) {
	// Set custom env vars
	t.Setenv("PORT", "9090")
	t.Setenv("WEBHOOK_MAX_PAYLOAD_SIZE", "2097152")  // 2MB
	t.Setenv("ADMIN_MAX_PAYLOAD_SIZE", "204800")     // 200KB
	t.Setenv("GRPC_MAX_MESSAGE_SIZE", "8388608")     // 8MB

	cfg := Load()

	if cfg.Port != "9090" {
		t.Errorf("expected port '9090', got '%s'", cfg.Port)
	}

	if cfg.WebhookMaxPayloadSize != 2097152 {
		t.Errorf("expected webhook payload size 2097152, got %d", cfg.WebhookMaxPayloadSize)
	}

	if cfg.AdminMaxPayloadSize != 204800 {
		t.Errorf("expected admin payload size 204800, got %d", cfg.AdminMaxPayloadSize)
	}

	if cfg.GRPCMaxMessageSize != 8388608 {
		t.Errorf("expected gRPC message size 8388608, got %d", cfg.GRPCMaxMessageSize)
	}
}

func TestLoad_InvalidInt64Values(t *testing.T) {
	// Set invalid values
	t.Setenv("WEBHOOK_MAX_PAYLOAD_SIZE", "invalid")
	t.Setenv("ADMIN_MAX_PAYLOAD_SIZE", "not-a-number")

	cfg := Load()

	// Should fall back to defaults for invalid values
	if cfg.WebhookMaxPayloadSize != DefaultWebhookMaxPayloadSize {
		t.Errorf("expected default for invalid webhook payload size, got %d", cfg.WebhookMaxPayloadSize)
	}

	if cfg.AdminMaxPayloadSize != DefaultAdminMaxPayloadSize {
		t.Errorf("expected default for invalid admin payload size, got %d", cfg.AdminMaxPayloadSize)
	}
}

func TestLoad_InvalidIntValues(t *testing.T) {
	// Set invalid value for int
	t.Setenv("GRPC_MAX_MESSAGE_SIZE", "invalid")

	cfg := Load()

	// Should fall back to default for invalid value
	if cfg.GRPCMaxMessageSize != DefaultGRPCMaxMessageSize {
		t.Errorf("expected default for invalid gRPC message size, got %d", cfg.GRPCMaxMessageSize)
	}
}

func TestDefaultConstants(t *testing.T) {
	// Verify the default constants are set correctly
	if DefaultWebhookMaxPayloadSize != 1<<20 {
		t.Errorf("expected DefaultWebhookMaxPayloadSize to be 1MB (1048576), got %d", DefaultWebhookMaxPayloadSize)
	}

	if DefaultAdminMaxPayloadSize != 100*1024 {
		t.Errorf("expected DefaultAdminMaxPayloadSize to be 100KB (102400), got %d", DefaultAdminMaxPayloadSize)
	}

	if DefaultGRPCMaxMessageSize != 4<<20 {
		t.Errorf("expected DefaultGRPCMaxMessageSize to be 4MB (4194304), got %d", DefaultGRPCMaxMessageSize)
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		defaultValue string
		expected     string
	}{
		{"env set", "TEST_KEY", "env_value", "default", "env_value"},
		{"env not set", "TEST_KEY_MISSING", "", "default", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv(tt.key, tt.envValue)
			}

			result := getEnvOrDefault(tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestGetEnvInt64OrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		defaultValue int64
		expected     int64
	}{
		{"valid int64", "TEST_INT64", "12345", 0, 12345},
		{"invalid int64", "TEST_INT64_INVALID", "abc", 999, 999},
		{"not set", "TEST_INT64_MISSING", "", 888, 888},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Unsetenv(tt.key)
			if tt.envValue != "" {
				t.Setenv(tt.key, tt.envValue)
			}

			result := getEnvInt64OrDefault(tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestGetEnvIntOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		defaultValue int
		expected     int
	}{
		{"valid int", "TEST_INT", "12345", 0, 12345},
		{"invalid int", "TEST_INT_INVALID", "abc", 999, 999},
		{"not set", "TEST_INT_MISSING", "", 888, 888},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Unsetenv(tt.key)
			if tt.envValue != "" {
				t.Setenv(tt.key, tt.envValue)
			}

			result := getEnvIntOrDefault(tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}
