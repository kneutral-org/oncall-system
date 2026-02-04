// Package webhook provides HTTP handlers for ingesting alerts from various sources.
package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	// ErrMissingSignature is returned when the signature header is missing.
	ErrMissingSignature = errors.New("missing signature header")
	// ErrInvalidSignature is returned when the signature verification fails.
	ErrInvalidSignature = errors.New("invalid signature")
	// ErrInvalidSignatureFormat is returned when the signature format is invalid.
	ErrInvalidSignatureFormat = errors.New("invalid signature format")
)

// HMACConfig holds HMAC verification settings.
type HMACConfig struct {
	// SignatureHeader is the name of the header containing the signature (e.g., "X-Hub-Signature-256").
	SignatureHeader string
	// SignaturePrefix is the prefix in header value (e.g., "sha256=").
	SignaturePrefix string
	// Secret is the secret key for HMAC verification.
	Secret []byte
}

// DefaultAlertmanagerConfig returns HMAC config for Alertmanager webhooks.
func DefaultAlertmanagerConfig(secret string) HMACConfig {
	return HMACConfig{
		SignatureHeader: "X-Alertmanager-Signature",
		SignaturePrefix: "sha256=",
		Secret:          []byte(secret),
	}
}

// DefaultGrafanaConfig returns HMAC config for Grafana webhooks.
func DefaultGrafanaConfig(secret string) HMACConfig {
	return HMACConfig{
		SignatureHeader: "X-Grafana-Signature",
		SignaturePrefix: "sha256=",
		Secret:          []byte(secret),
	}
}

// DefaultGenericConfig returns HMAC config for generic webhooks.
func DefaultGenericConfig(secret string) HMACConfig {
	return HMACConfig{
		SignatureHeader: "X-Webhook-Signature",
		SignaturePrefix: "sha256=",
		Secret:          []byte(secret),
	}
}

// VerifyHMAC verifies the HMAC-SHA256 signature of a request body.
// It compares the expected signature with the provided signature using constant-time comparison.
func VerifyHMAC(body []byte, signature string, secret []byte) bool {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// ComputeHMAC computes the HMAC-SHA256 signature of the given body using the provided secret.
// Returns the hex-encoded signature.
func ComputeHMAC(body []byte, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// HMACMiddleware creates a Gin middleware for HMAC signature verification.
// If the secret is empty, the middleware will skip verification (development mode).
func HMACMiddleware(config HMACConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip if no secret configured (development mode)
		if len(config.Secret) == 0 {
			c.Next()
			return
		}

		// Get signature from header
		sigHeader := c.GetHeader(config.SignatureHeader)
		if sigHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "missing signature header",
			})
			return
		}

		// Extract signature value (remove prefix)
		signature := strings.TrimPrefix(sigHeader, config.SignaturePrefix)
		if signature == sigHeader {
			// Prefix was not present
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "invalid signature format",
			})
			return
		}

		// Read body
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error":   "badRequest",
				"message": "failed to read request body",
			})
			return
		}

		// Restore body for downstream handlers
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		// Verify signature
		if !VerifyHMAC(body, signature, config.Secret) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "invalid signature",
			})
			return
		}

		c.Next()
	}
}

// WebhookConfig holds the HMAC secrets for different webhook sources.
type WebhookConfig struct {
	// AlertmanagerSecret is the HMAC secret for Alertmanager webhooks.
	AlertmanagerSecret string
	// GrafanaSecret is the HMAC secret for Grafana webhooks.
	GrafanaSecret string
	// GenericSecret is the HMAC secret for generic webhooks.
	GenericSecret string
}
