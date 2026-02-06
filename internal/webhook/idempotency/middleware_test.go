package idempotency

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func setupTestRouter(cfg Config) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.POST("/webhook/:integration_key", Middleware(cfg), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.POST("/webhook-error/:integration_key", Middleware(cfg), func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	})

	return router
}

func TestMiddleware_FirstRequest(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	cfg := NewConfig(store).WithLogger(zerolog.Nop())
	router := setupTestRouter(cfg)

	body := []byte(`{"test": "data"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook/test-key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestMiddleware_DuplicateRequest(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	cfg := NewConfig(store).WithLogger(zerolog.Nop())
	router := setupTestRouter(cfg)

	body := []byte(`{"test": "data"}`)

	// First request
	req1 := httptest.NewRequest(http.MethodPost, "/webhook/test-key", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("first request: expected status 200, got %d", w1.Code)
	}

	// Duplicate request with same body
	req2 := httptest.NewRequest(http.MethodPost, "/webhook/test-key", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Errorf("duplicate request: expected status 409, got %d", w2.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["error"] != "conflict" {
		t.Errorf("expected error 'conflict', got '%v'", resp["error"])
	}
}

func TestMiddleware_DifferentBodies(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	cfg := NewConfig(store).WithLogger(zerolog.Nop())
	router := setupTestRouter(cfg)

	// First request
	body1 := []byte(`{"test": "data1"}`)
	req1 := httptest.NewRequest(http.MethodPost, "/webhook/test-key", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("first request: expected status 200, got %d", w1.Code)
	}

	// Second request with different body
	body2 := []byte(`{"test": "data2"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/webhook/test-key", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("second request with different body: expected status 200, got %d", w2.Code)
	}
}

func TestMiddleware_DifferentIntegrationKeys(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	cfg := NewConfig(store).WithLogger(zerolog.Nop())
	router := setupTestRouter(cfg)

	body := []byte(`{"test": "data"}`)

	// First request
	req1 := httptest.NewRequest(http.MethodPost, "/webhook/key-1", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("first request: expected status 200, got %d", w1.Code)
	}

	// Second request with same body but different integration key
	req2 := httptest.NewRequest(http.MethodPost, "/webhook/key-2", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("second request with different integration key: expected status 200, got %d", w2.Code)
	}
}

func TestMiddleware_IdempotencyKeyHeader(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	cfg := NewConfig(store).WithLogger(zerolog.Nop())
	router := setupTestRouter(cfg)

	body := []byte(`{"test": "data"}`)

	// First request with idempotency key header
	req1 := httptest.NewRequest(http.MethodPost, "/webhook/test-key", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set(IdempotencyKeyHeader, "my-idempotency-key")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("first request: expected status 200, got %d", w1.Code)
	}

	// Duplicate request with same idempotency key but different body
	body2 := []byte(`{"test": "different-data"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/webhook/test-key", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set(IdempotencyKeyHeader, "my-idempotency-key")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	// Should be duplicate because same idempotency key
	if w2.Code != http.StatusConflict {
		t.Errorf("duplicate request with same idempotency key: expected status 409, got %d", w2.Code)
	}

	// Different idempotency key should succeed
	req3 := httptest.NewRequest(http.MethodPost, "/webhook/test-key", bytes.NewReader(body2))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set(IdempotencyKeyHeader, "different-idempotency-key")
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("request with different idempotency key: expected status 200, got %d", w3.Code)
	}
}

func TestMiddleware_TTLExpiration(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	shortTTL := 50 * time.Millisecond
	cfg := NewConfig(store).WithTTL(shortTTL).WithLogger(zerolog.Nop())
	router := setupTestRouter(cfg)

	body := []byte(`{"test": "data"}`)

	// First request
	req1 := httptest.NewRequest(http.MethodPost, "/webhook/test-key", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("first request: expected status 200, got %d", w1.Code)
	}

	// Wait for TTL to expire
	time.Sleep(60 * time.Millisecond)

	// Same request should now succeed again
	req2 := httptest.NewRequest(http.MethodPost, "/webhook/test-key", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("request after TTL expiration: expected status 200, got %d", w2.Code)
	}
}

func TestMiddleware_DeleteKeyOnError(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	cfg := NewConfig(store).WithDeleteKeyOnError(true).WithLogger(zerolog.Nop())

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/webhook-error/:integration_key", Middleware(cfg), func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	})

	body := []byte(`{"test": "data"}`)

	// First request that fails
	req1 := httptest.NewRequest(http.MethodPost, "/webhook-error/test-key", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusInternalServerError {
		t.Errorf("first request: expected status 500, got %d", w1.Code)
	}

	// Retry should be allowed because key was deleted on error
	req2 := httptest.NewRequest(http.MethodPost, "/webhook-error/test-key", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	// Should not be blocked as a duplicate
	if w2.Code != http.StatusInternalServerError {
		t.Errorf("retry after error: expected status 500, got %d", w2.Code)
	}
}

func TestMiddleware_CustomKeyExtractor(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	customExtractor := func(c *gin.Context) string {
		return c.GetHeader("X-Custom-Key")
	}

	cfg := NewConfig(store).
		WithKeyExtractor(customExtractor).
		WithLogger(zerolog.Nop())

	router := setupTestRouter(cfg)

	body := []byte(`{"test": "data"}`)

	// Request with custom key
	req1 := httptest.NewRequest(http.MethodPost, "/webhook/test-key", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Custom-Key", "custom-key-123")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("first request: expected status 200, got %d", w1.Code)
	}

	// Different body but same custom key should be duplicate
	body2 := []byte(`{"test": "different"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/webhook/test-key", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Custom-Key", "custom-key-123")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Errorf("duplicate with custom key: expected status 409, got %d", w2.Code)
	}
}

func TestMiddleware_NoKeyExtracted(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	// Extractor that returns empty string
	emptyExtractor := func(c *gin.Context) string {
		return ""
	}

	cfg := NewConfig(store).
		WithKeyExtractor(emptyExtractor).
		WithLogger(zerolog.Nop())

	router := setupTestRouter(cfg)

	body := []byte(`{"test": "data"}`)

	// Request should proceed without idempotency check
	req1 := httptest.NewRequest(http.MethodPost, "/webhook/test-key", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("first request: expected status 200, got %d", w1.Code)
	}

	// Same request should also proceed (no idempotency check)
	req2 := httptest.NewRequest(http.MethodPost, "/webhook/test-key", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("second request without key: expected status 200, got %d", w2.Code)
	}
}

func TestMiddleware_PanicOnNilStore(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when store is nil")
		}
	}()

	cfg := Config{Store: nil}
	Middleware(cfg)
}

func TestDefaultKeyExtractor_WithHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a test context with idempotency header
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/webhook/int-key", nil)
	c.Request.Header.Set(IdempotencyKeyHeader, "header-key")
	c.Params = gin.Params{{Key: "integration_key", Value: "int-key"}}

	key := DefaultKeyExtractor(c)

	expected := "int-key:header-key"
	if key != expected {
		t.Errorf("expected key %q, got %q", expected, key)
	}
}

func TestDefaultKeyExtractor_WithBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"test": "data"}`)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/webhook/int-key", bytes.NewReader(body))
	c.Params = gin.Params{{Key: "integration_key", Value: "int-key"}}

	key := DefaultKeyExtractor(c)

	// Key should be a SHA256 hash
	if len(key) != 64 { // SHA256 hex string is 64 chars
		t.Errorf("expected 64-char hash, got %d chars: %s", len(key), key)
	}
}

func BenchmarkMiddleware(b *testing.B) {
	store := NewMemoryStore()
	defer store.Close()

	cfg := NewConfig(store).WithLogger(zerolog.Nop())
	router := setupTestRouter(cfg)

	body := []byte(`{"test": "data"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/webhook/test-key", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(IdempotencyKeyHeader, "unique-key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Reset for next iteration
		store.Delete(req.Context(), "test-key:unique-key")
	}
}
