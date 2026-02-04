// Package logging provides structured logging utilities.
package logging

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger(t *testing.T) {
	logger := NewLogger("test-service", "info")

	assert.NotNil(t, logger)
}

func TestNewLogger_ParseLevel(t *testing.T) {
	tests := []struct {
		level    string
		expected zerolog.Level
	}{
		{"debug", zerolog.DebugLevel},
		{"info", zerolog.InfoLevel},
		{"warn", zerolog.WarnLevel},
		{"error", zerolog.ErrorLevel},
		{"invalid", zerolog.InfoLevel}, // defaults to info
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			logger := NewLogger("test-service", tt.level)
			assert.Equal(t, tt.expected, logger.GetLevel())
		})
	}
}

func TestNewPrettyLogger(t *testing.T) {
	logger := NewPrettyLogger("test-service", "debug")

	assert.NotNil(t, logger)
}

func TestContextWithLogger(t *testing.T) {
	logger := NewLogger("test-service", "info")
	ctx := context.Background()

	ctxWithLogger := ContextWithLogger(ctx, logger)

	assert.NotNil(t, ctxWithLogger)
}

func TestLoggerFromContext(t *testing.T) {
	logger := NewLogger("test-service", "info")
	ctx := ContextWithLogger(context.Background(), logger)

	extracted := LoggerFromContext(ctx)

	assert.NotNil(t, extracted)
}

func TestNotificationLogger(t *testing.T) {
	baseLogger := NewLogger("test-service", "info")

	notifLogger := NotificationLogger(baseLogger, "notif-123", "slack")

	assert.NotNil(t, notifLogger)
}

func TestAlertLogger(t *testing.T) {
	baseLogger := NewLogger("test-service", "info")

	alertLogger := AlertLogger(baseLogger, "alert-456", "critical")

	assert.NotNil(t, alertLogger)
}

func TestHTTPMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	wrapped := HTTPMiddleware(logger, handler)

	req := httptest.NewRequest("GET", "/test/path?query=value", nil)
	req.Header.Set("User-Agent", "test-agent")
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "OK", rec.Body.String())

	// Verify log output contains expected fields
	logOutput := buf.String()
	assert.Contains(t, logOutput, "http_request")
	assert.Contains(t, logOutput, "GET")
	assert.Contains(t, logOutput, "/test/path")
}

func TestHTTPMiddleware_StatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"success", http.StatusOK},
		{"client_error", http.StatusBadRequest},
		{"server_error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := zerolog.New(&buf).With().Timestamp().Logger()

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			})

			wrapped := HTTPMiddleware(logger, handler)

			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			assert.Equal(t, tt.statusCode, rec.Code)
		})
	}
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	rw.WriteHeader(http.StatusNotFound)

	assert.Equal(t, http.StatusNotFound, rw.statusCode)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestResponseWriter_DefaultStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	// Without calling WriteHeader, status should remain at default
	assert.Equal(t, http.StatusOK, rw.statusCode)
}

func TestRequestLogger_Integration(t *testing.T) {
	// This test verifies RequestLogger doesn't panic
	// Full integration testing requires a Gin test server
	logger := NewLogger("test-service", "info")

	middleware := RequestLogger(logger)

	assert.NotNil(t, middleware)
}

func TestGRPCLogger_Integration(t *testing.T) {
	// This test verifies GRPCLogger doesn't panic
	logger := NewLogger("test-service", "info")

	interceptor := GRPCLogger(logger)

	assert.NotNil(t, interceptor)
}

func TestGRPCStreamLogger_Integration(t *testing.T) {
	// This test verifies GRPCStreamLogger doesn't panic
	logger := NewLogger("test-service", "info")

	interceptor := GRPCStreamLogger(logger)

	assert.NotNil(t, interceptor)
}

func TestLoggerOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).With().Timestamp().Str("service", "test").Logger()

	logger.Info().Str("key", "value").Msg("test message")

	output := buf.String()
	require.NotEmpty(t, output)
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "key")
	assert.Contains(t, output, "value")
	assert.Contains(t, output, "service")
	assert.Contains(t, output, "test")
}
