package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func computeTestSignature(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyHMAC_ValidSignature(t *testing.T) {
	secret := []byte("test-secret-key")
	body := []byte(`{"test": "payload"}`)

	// Compute valid signature
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	validSignature := hex.EncodeToString(mac.Sum(nil))

	// Should return true for valid signature
	assert.True(t, VerifyHMAC(body, validSignature, secret))
}

func TestVerifyHMAC_InvalidSignature(t *testing.T) {
	secret := []byte("test-secret-key")
	body := []byte(`{"test": "payload"}`)
	invalidSignature := "invalid-signature-hex"

	// Should return false for invalid signature
	assert.False(t, VerifyHMAC(body, invalidSignature, secret))
}

func TestVerifyHMAC_WrongSecret(t *testing.T) {
	secret := []byte("test-secret-key")
	wrongSecret := []byte("wrong-secret-key")
	body := []byte(`{"test": "payload"}`)

	// Compute signature with correct secret
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	// Should return false when verifying with wrong secret
	assert.False(t, VerifyHMAC(body, signature, wrongSecret))
}

func TestVerifyHMAC_ModifiedBody(t *testing.T) {
	secret := []byte("test-secret-key")
	originalBody := []byte(`{"test": "payload"}`)
	modifiedBody := []byte(`{"test": "modified"}`)

	// Compute signature with original body
	mac := hmac.New(sha256.New, secret)
	mac.Write(originalBody)
	signature := hex.EncodeToString(mac.Sum(nil))

	// Should return false when body has been modified
	assert.False(t, VerifyHMAC(modifiedBody, signature, secret))
}

func TestVerifyHMAC_EmptyBody(t *testing.T) {
	secret := []byte("test-secret-key")
	body := []byte{}

	// Compute valid signature for empty body
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	validSignature := hex.EncodeToString(mac.Sum(nil))

	// Should return true for valid signature of empty body
	assert.True(t, VerifyHMAC(body, validSignature, secret))
}

func TestComputeHMAC(t *testing.T) {
	secret := []byte("test-secret-key")
	body := []byte(`{"test": "payload"}`)

	// Compute signature using our function
	signature := ComputeHMAC(body, secret)

	// Verify it matches expected computation
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	assert.Equal(t, expected, signature)
}

func TestDefaultAlertmanagerConfig(t *testing.T) {
	secret := "my-alertmanager-secret"
	config := DefaultAlertmanagerConfig(secret)

	assert.Equal(t, "X-Alertmanager-Signature", config.SignatureHeader)
	assert.Equal(t, "sha256=", config.SignaturePrefix)
	assert.Equal(t, []byte(secret), config.Secret)
}

func TestDefaultGrafanaConfig(t *testing.T) {
	secret := "my-grafana-secret"
	config := DefaultGrafanaConfig(secret)

	assert.Equal(t, "X-Grafana-Signature", config.SignatureHeader)
	assert.Equal(t, "sha256=", config.SignaturePrefix)
	assert.Equal(t, []byte(secret), config.Secret)
}

func TestDefaultGenericConfig(t *testing.T) {
	secret := "my-generic-secret"
	config := DefaultGenericConfig(secret)

	assert.Equal(t, "X-Webhook-Signature", config.SignatureHeader)
	assert.Equal(t, "sha256=", config.SignaturePrefix)
	assert.Equal(t, []byte(secret), config.Secret)
}

func setupTestRouter(middleware gin.HandlerFunc) *gin.Engine {
	router := gin.New()
	router.POST("/test", middleware, func(c *gin.Context) {
		// Read body to verify it was restored
		body, _ := io.ReadAll(c.Request.Body)
		c.JSON(http.StatusOK, gin.H{
			"message": "success",
			"body":    string(body),
		})
	})
	return router
}

func TestHMACMiddleware_ValidSignature(t *testing.T) {
	secret := "test-secret"
	config := HMACConfig{
		SignatureHeader: "X-Test-Signature",
		SignaturePrefix: "sha256=",
		Secret:          []byte(secret),
	}

	router := setupTestRouter(HMACMiddleware(config))

	body := []byte(`{"test": "data"}`)
	signature := "sha256=" + computeTestSignature(body, secret)

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Signature", signature)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHMACMiddleware_MissingSignatureHeader(t *testing.T) {
	secret := "test-secret"
	config := HMACConfig{
		SignatureHeader: "X-Test-Signature",
		SignaturePrefix: "sha256=",
		Secret:          []byte(secret),
	}

	router := setupTestRouter(HMACMiddleware(config))

	body := []byte(`{"test": "data"}`)

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Missing signature header

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "missing signature header")
}

func TestHMACMiddleware_InvalidSignatureFormat(t *testing.T) {
	secret := "test-secret"
	config := HMACConfig{
		SignatureHeader: "X-Test-Signature",
		SignaturePrefix: "sha256=",
		Secret:          []byte(secret),
	}

	router := setupTestRouter(HMACMiddleware(config))

	body := []byte(`{"test": "data"}`)

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Missing prefix in signature
	req.Header.Set("X-Test-Signature", "invalid-no-prefix")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "invalid signature format")
}

func TestHMACMiddleware_InvalidSignature(t *testing.T) {
	secret := "test-secret"
	config := HMACConfig{
		SignatureHeader: "X-Test-Signature",
		SignaturePrefix: "sha256=",
		Secret:          []byte(secret),
	}

	router := setupTestRouter(HMACMiddleware(config))

	body := []byte(`{"test": "data"}`)

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Signature", "sha256=wrongsignature123456")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "invalid signature")
}

func TestHMACMiddleware_WrongSecret(t *testing.T) {
	secret := "correct-secret"
	wrongSecret := "wrong-secret"
	config := HMACConfig{
		SignatureHeader: "X-Test-Signature",
		SignaturePrefix: "sha256=",
		Secret:          []byte(secret),
	}

	router := setupTestRouter(HMACMiddleware(config))

	body := []byte(`{"test": "data"}`)
	// Sign with wrong secret
	signature := "sha256=" + computeTestSignature(body, wrongSecret)

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Signature", signature)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "invalid signature")
}

func TestHMACMiddleware_EmptySecret_SkipsVerification(t *testing.T) {
	config := HMACConfig{
		SignatureHeader: "X-Test-Signature",
		SignaturePrefix: "sha256=",
		Secret:          []byte{}, // Empty secret - development mode
	}

	router := setupTestRouter(HMACMiddleware(config))

	body := []byte(`{"test": "data"}`)

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No signature header needed in dev mode

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should pass without signature
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHMACMiddleware_BodyIsRestoredForDownstreamHandlers(t *testing.T) {
	secret := "test-secret"
	config := HMACConfig{
		SignatureHeader: "X-Test-Signature",
		SignaturePrefix: "sha256=",
		Secret:          []byte(secret),
	}

	router := setupTestRouter(HMACMiddleware(config))

	body := []byte(`{"test": "data"}`)
	signature := "sha256=" + computeTestSignature(body, secret)

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Signature", signature)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	// Verify the downstream handler could read the body
	assert.Contains(t, w.Body.String(), `"body":"{\"test\": \"data\"}"`)
}

func TestHMACMiddleware_LargePayload(t *testing.T) {
	secret := "test-secret"
	config := HMACConfig{
		SignatureHeader: "X-Test-Signature",
		SignaturePrefix: "sha256=",
		Secret:          []byte(secret),
	}

	router := setupTestRouter(HMACMiddleware(config))

	// Create a large payload (1MB)
	largeBody := make([]byte, 1024*1024)
	for i := range largeBody {
		largeBody[i] = byte('a' + (i % 26))
	}
	signature := "sha256=" + computeTestSignature(largeBody, secret)

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Test-Signature", signature)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHMACMiddleware_AlertmanagerIntegration(t *testing.T) {
	secret := "alertmanager-webhook-secret"
	config := DefaultAlertmanagerConfig(secret)

	router := setupTestRouter(HMACMiddleware(config))

	// Realistic Alertmanager payload
	body := []byte(`{
		"version": "4",
		"groupKey": "alertname",
		"status": "firing",
		"receiver": "web.hook",
		"alerts": [
			{
				"status": "firing",
				"labels": {"alertname": "HighCPU"},
				"annotations": {"summary": "CPU is high"},
				"startsAt": "2024-01-01T00:00:00Z",
				"fingerprint": "abc123"
			}
		]
	}`)
	signature := "sha256=" + computeTestSignature(body, secret)

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Alertmanager-Signature", signature)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHMACMiddleware_GrafanaIntegration(t *testing.T) {
	secret := "grafana-webhook-secret"
	config := DefaultGrafanaConfig(secret)

	router := setupTestRouter(HMACMiddleware(config))

	// Realistic Grafana payload
	body := []byte(`{
		"title": "High CPU Alert",
		"ruleId": 123,
		"ruleName": "CPU Alert",
		"state": "alerting",
		"message": "CPU usage is above 90%"
	}`)
	signature := "sha256=" + computeTestSignature(body, secret)

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Grafana-Signature", signature)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHMACMiddleware_GenericIntegration(t *testing.T) {
	secret := "generic-webhook-secret"
	config := DefaultGenericConfig(secret)

	router := setupTestRouter(HMACMiddleware(config))

	body := []byte(`{"summary": "Test alert", "severity": "high"}`)
	signature := "sha256=" + computeTestSignature(body, secret)

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWebhookConfig(t *testing.T) {
	config := WebhookConfig{
		AlertmanagerSecret: "am-secret",
		GrafanaSecret:      "grafana-secret",
		GenericSecret:      "generic-secret",
	}

	assert.Equal(t, "am-secret", config.AlertmanagerSecret)
	assert.Equal(t, "grafana-secret", config.GrafanaSecret)
	assert.Equal(t, "generic-secret", config.GenericSecret)
}
