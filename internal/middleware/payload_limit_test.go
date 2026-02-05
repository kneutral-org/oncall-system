package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestRouter(maxBytes int64) *gin.Engine {
	logger := zerolog.Nop()
	router := gin.New()
	router.Use(PayloadLimitErrorHandler(logger))
	router.Use(PayloadLimit(maxBytes, logger))

	router.POST("/test", func(c *gin.Context) {
		// Read the body to trigger any MaxBytesError
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			_ = c.Error(err)
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"received": len(body)})
	})

	return router
}

func TestPayloadLimit_UnderLimit(t *testing.T) {
	router := setupTestRouter(1024) // 1KB limit

	body := strings.Repeat("a", 500) // 500 bytes
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]int
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["received"] != 500 {
		t.Errorf("expected received=500, got %d", resp["received"])
	}
}

func TestPayloadLimit_AtExactLimit(t *testing.T) {
	router := setupTestRouter(100) // 100 bytes limit

	body := strings.Repeat("x", 100) // Exactly 100 bytes
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPayloadLimit_OverLimit_ContentLength(t *testing.T) {
	router := setupTestRouter(100) // 100 bytes limit

	body := strings.Repeat("x", 200) // 200 bytes, over limit
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = 200 // Explicitly set Content-Length

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413, got %d: %s", w.Code, w.Body.String())
	}

	var resp PayloadLimitErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error != "payloadTooLarge" {
		t.Errorf("expected error='payloadTooLarge', got '%s'", resp.Error)
	}

	if resp.MaxBytes != 100 {
		t.Errorf("expected maxBytes=100, got %d", resp.MaxBytes)
	}

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("expected statusCode=413, got %d", resp.StatusCode)
	}
}

func TestPayloadLimit_OverLimit_StreamedBody(t *testing.T) {
	router := setupTestRouter(100) // 100 bytes limit

	// Create a reader without Content-Length (simulating chunked encoding)
	body := bytes.NewReader([]byte(strings.Repeat("x", 200)))
	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = -1 // Unknown content length

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should get 413 or 400 (from MaxBytesReader)
	if w.Code != http.StatusRequestEntityTooLarge && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 413 or 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPayloadLimit_EmptyBody(t *testing.T) {
	router := setupTestRouter(100) // 100 bytes limit

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for empty body, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPayloadLimit_ZeroContentLength(t *testing.T) {
	router := setupTestRouter(100) // 100 bytes limit

	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = 0

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for zero content-length, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPayloadLimit_LargeWebhookPayload(t *testing.T) {
	// Test with realistic webhook limit (1MB)
	router := setupTestRouter(1 << 20) // 1MB limit

	// Create a payload just under the limit
	body := strings.Repeat("x", (1<<20)-100) // 1MB - 100 bytes
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestPayloadLimit_LargeWebhookPayloadExceeded(t *testing.T) {
	// Test with realistic webhook limit (1MB)
	router := setupTestRouter(1 << 20) // 1MB limit

	// Create a payload over the limit
	body := strings.Repeat("x", (1<<20)+1000) // 1MB + 1000 bytes
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(body))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPayloadLimit_AdminLimit(t *testing.T) {
	// Test with realistic admin limit (100KB)
	router := setupTestRouter(100 * 1024) // 100KB limit

	// Create a payload just under the limit
	body := strings.Repeat("x", 100*1024-100) // 100KB - 100 bytes
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestPayloadLimit_AdminLimitExceeded(t *testing.T) {
	// Test with realistic admin limit (100KB)
	router := setupTestRouter(100 * 1024) // 100KB limit

	// Create a payload over the limit
	body := strings.Repeat("x", 100*1024+1000) // 100KB + 1000 bytes
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(body))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPayloadLimit_ResponseFormat(t *testing.T) {
	router := setupTestRouter(100)

	body := strings.Repeat("x", 200)
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = 200

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify response is valid JSON with expected fields
	var resp PayloadLimitErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response should be valid JSON: %v", err)
	}

	// Verify all required fields are present
	if resp.Error == "" {
		t.Error("error field should not be empty")
	}
	if resp.Message == "" {
		t.Error("message field should not be empty")
	}
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("statusCode should be 413, got %d", resp.StatusCode)
	}
}

func TestPayloadLimit_JSONPayload(t *testing.T) {
	router := setupTestRouter(1024) // 1KB limit

	// Test with realistic JSON payload
	payload := map[string]interface{}{
		"alertname": "TestAlert",
		"severity":  "critical",
		"labels": map[string]string{
			"env":  "production",
			"team": "platform",
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}
