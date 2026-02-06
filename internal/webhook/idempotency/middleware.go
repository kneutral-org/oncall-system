// Package idempotency provides mechanisms to prevent duplicate webhook processing.
package idempotency

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// KeyExtractor is a function that extracts an idempotency key from a request.
type KeyExtractor func(*gin.Context) string

// Config holds the configuration for the idempotency middleware.
type Config struct {
	// Store is the idempotency key store.
	Store Store
	// TTL is how long to remember processed requests.
	TTL time.Duration
	// KeyExtractor extracts the idempotency key from the request.
	// If nil, DefaultKeyExtractor is used.
	KeyExtractor KeyExtractor
	// Logger for logging duplicate requests.
	Logger zerolog.Logger
	// DeleteKeyOnError if true, deletes the key when the request fails,
	// allowing the client to retry.
	DeleteKeyOnError bool
}

// DefaultTTL is the default TTL for idempotency keys (24 hours).
const DefaultTTL = 24 * time.Hour

// IdempotencyKeyHeader is the standard header for client-provided idempotency keys.
const IdempotencyKeyHeader = "X-Idempotency-Key"

// DefaultKeyExtractor extracts the idempotency key using the following strategy:
// 1. Use X-Idempotency-Key header if present
// 2. Fall back to hash of request body + integration key from URL
func DefaultKeyExtractor(c *gin.Context) string {
	// Check for explicit idempotency key header
	if key := c.GetHeader(IdempotencyKeyHeader); key != "" {
		// Include integration key to namespace by service
		integrationKey := c.Param("integration_key")
		if integrationKey != "" {
			return integrationKey + ":" + key
		}
		return key
	}

	// Fall back to body hash
	return extractBodyHash(c)
}

// extractBodyHash creates a hash from the request body and integration key.
func extractBodyHash(c *gin.Context) string {
	integrationKey := c.Param("integration_key")

	// Read and restore the body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	// Create hash of integration key + body
	h := sha256.New()
	h.Write([]byte(integrationKey))
	h.Write([]byte(":"))
	h.Write(body)

	return hex.EncodeToString(h.Sum(nil))
}

// Middleware creates a Gin middleware that enforces idempotency.
// Duplicate requests within the TTL window receive a 409 Conflict response.
func Middleware(cfg Config) gin.HandlerFunc {
	if cfg.Store == nil {
		panic("idempotency: store is required")
	}

	if cfg.TTL == 0 {
		cfg.TTL = DefaultTTL
	}

	if cfg.KeyExtractor == nil {
		cfg.KeyExtractor = DefaultKeyExtractor
	}

	return func(c *gin.Context) {
		// Extract idempotency key
		key := cfg.KeyExtractor(c)
		if key == "" {
			// No key could be extracted, proceed without idempotency check
			c.Next()
			return
		}

		// Try to set the key
		isNew, err := cfg.Store.CheckAndSet(c.Request.Context(), key, cfg.TTL)
		if err != nil {
			cfg.Logger.Error().
				Err(err).
				Str("idempotencyKey", key).
				Msg("failed to check idempotency key")
			// On store error, proceed with the request to avoid blocking legitimate traffic
			c.Next()
			return
		}

		if !isNew {
			// Duplicate request
			cfg.Logger.Info().
				Str("idempotencyKey", key).
				Str("path", c.Request.URL.Path).
				Str("method", c.Request.Method).
				Msg("duplicate request detected")

			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error":   "conflict",
				"message": "duplicate request detected",
				"details": "This request has already been processed. If you need to retry, use a new X-Idempotency-Key.",
			})
			return
		}

		// Store the key in context for potential cleanup on error
		c.Set("idempotencyKey", key)

		// Process the request
		c.Next()

		// If configured and request failed, delete the key to allow retry
		if cfg.DeleteKeyOnError && c.Writer.Status() >= 500 {
			if err := cfg.Store.Delete(c.Request.Context(), key); err != nil {
				cfg.Logger.Error().
					Err(err).
					Str("idempotencyKey", key).
					Msg("failed to delete idempotency key after error")
			}
		}
	}
}

// NewConfig creates a default configuration with the provided store.
func NewConfig(store Store) Config {
	return Config{
		Store:            store,
		TTL:              DefaultTTL,
		KeyExtractor:     DefaultKeyExtractor,
		Logger:           zerolog.Nop(),
		DeleteKeyOnError: true,
	}
}

// WithTTL sets the TTL for idempotency keys.
func (c Config) WithTTL(ttl time.Duration) Config {
	c.TTL = ttl
	return c
}

// WithKeyExtractor sets a custom key extractor.
func (c Config) WithKeyExtractor(extractor KeyExtractor) Config {
	c.KeyExtractor = extractor
	return c
}

// WithLogger sets the logger.
func (c Config) WithLogger(logger zerolog.Logger) Config {
	c.Logger = logger
	return c
}

// WithDeleteKeyOnError sets whether to delete the key on error.
func (c Config) WithDeleteKeyOnError(delete bool) Config {
	c.DeleteKeyOnError = delete
	return c
}
