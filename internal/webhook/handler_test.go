package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/kneutral-org/alerting-system/internal/store"
	alertingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/v1"
)

// mockAlertStore implements store.AlertStore for testing.
type mockAlertStore struct {
	alerts          map[string]*alertingv1.Alert
	alertsByFP      map[string]*alertingv1.Alert
	createOrUpdateFn func(ctx context.Context, alert *alertingv1.Alert) (*alertingv1.Alert, bool, error)
}

func newMockAlertStore() *mockAlertStore {
	return &mockAlertStore{
		alerts:     make(map[string]*alertingv1.Alert),
		alertsByFP: make(map[string]*alertingv1.Alert),
	}
}

func (m *mockAlertStore) Create(ctx context.Context, alert *alertingv1.Alert) (*alertingv1.Alert, error) {
	alert.Id = "alert-" + alert.Fingerprint
	m.alerts[alert.Id] = alert
	m.alertsByFP[alert.Fingerprint] = alert
	return alert, nil
}

func (m *mockAlertStore) GetByID(ctx context.Context, id string) (*alertingv1.Alert, error) {
	alert, ok := m.alerts[id]
	if !ok {
		return nil, nil
	}
	return alert, nil
}

func (m *mockAlertStore) GetByFingerprint(ctx context.Context, fingerprint string) (*alertingv1.Alert, error) {
	alert, ok := m.alertsByFP[fingerprint]
	if !ok {
		return nil, nil
	}
	return alert, nil
}

func (m *mockAlertStore) Update(ctx context.Context, alert *alertingv1.Alert) (*alertingv1.Alert, error) {
	m.alerts[alert.Id] = alert
	m.alertsByFP[alert.Fingerprint] = alert
	return alert, nil
}

func (m *mockAlertStore) CreateOrUpdate(ctx context.Context, alert *alertingv1.Alert) (*alertingv1.Alert, bool, error) {
	if m.createOrUpdateFn != nil {
		return m.createOrUpdateFn(ctx, alert)
	}

	existing, ok := m.alertsByFP[alert.Fingerprint]
	if ok {
		alert.Id = existing.Id
		m.alerts[alert.Id] = alert
		m.alertsByFP[alert.Fingerprint] = alert
		return alert, false, nil
	}
	// Use fingerprint as ID suffix, truncating if longer than 8 chars
	fpSuffix := alert.Fingerprint
	if len(fpSuffix) > 8 {
		fpSuffix = fpSuffix[:8]
	}
	alert.Id = "alert-" + fpSuffix
	m.alerts[alert.Id] = alert
	m.alertsByFP[alert.Fingerprint] = alert
	return alert, true, nil
}

func (m *mockAlertStore) List(ctx context.Context, req *alertingv1.ListAlertsRequest) (*alertingv1.ListAlertsResponse, error) {
	var alerts []*alertingv1.Alert
	for _, a := range m.alerts {
		alerts = append(alerts, a)
	}
	return &alertingv1.ListAlertsResponse{Alerts: alerts}, nil
}

// mockServiceStore implements store.ServiceStore for testing.
type mockServiceStore struct {
	services map[string]*store.Service
}

func newMockServiceStore() *mockServiceStore {
	return &mockServiceStore{
		services: map[string]*store.Service{
			"valid-key": {
				ID:             "svc-123",
				Name:           "Test Service",
				IntegrationKey: "valid-key",
			},
		},
	}
}

func (m *mockServiceStore) GetByIntegrationKey(ctx context.Context, integrationKey string) (*store.Service, error) {
	svc, ok := m.services[integrationKey]
	if !ok {
		return nil, context.DeadlineExceeded // Simulate not found
	}
	return svc, nil
}

func (m *mockServiceStore) Create(ctx context.Context, service *store.Service) (*store.Service, error) {
	m.services[service.IntegrationKey] = service
	return service, nil
}

func (m *mockServiceStore) GetByID(ctx context.Context, id string) (*store.Service, error) {
	for _, svc := range m.services {
		if svc.ID == id {
			return svc, nil
		}
	}
	return nil, nil
}

func setupTestHandler() (*Handler, *gin.Engine, *mockAlertStore, *mockServiceStore) {
	gin.SetMode(gin.TestMode)

	alertStore := newMockAlertStore()
	serviceStore := newMockServiceStore()
	logger := zerolog.Nop()

	handler := NewHandler(alertStore, serviceStore, logger)

	router := gin.New()
	api := router.Group("/api/v1")
	handler.RegisterRoutes(api)

	return handler, router, alertStore, serviceStore
}

// TestAlertmanagerWebhook_Success tests successful Alertmanager webhook processing.
func TestAlertmanagerWebhook_Success(t *testing.T) {
	_, router, alertStore, _ := setupTestHandler()

	payload := AlertmanagerPayload{
		Version:  "4",
		GroupKey: "test-group",
		Status:   "firing",
		Receiver: "test-receiver",
		Alerts: []AlertmanagerAlert{
			{
				Status:      "firing",
				Labels:      map[string]string{"alertname": "TestAlert", "severity": "critical"},
				Annotations: map[string]string{"summary": "Test summary", "description": "Test description"},
				StartsAt:    time.Now(),
				Fingerprint: "abc123",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/alertmanager/valid-key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp WebhookResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp.AlertIds) != 1 {
		t.Errorf("expected 1 alert ID, got %d", len(resp.AlertIds))
	}

	if resp.Created != 1 {
		t.Errorf("expected 1 created, got %d", resp.Created)
	}

	// Verify alert was stored
	if len(alertStore.alerts) != 1 {
		t.Errorf("expected 1 alert in store, got %d", len(alertStore.alerts))
	}
}

// TestAlertmanagerWebhook_InvalidKey tests unauthorized access with invalid integration key.
func TestAlertmanagerWebhook_InvalidKey(t *testing.T) {
	_, router, _, _ := setupTestHandler()

	payload := AlertmanagerPayload{
		Alerts: []AlertmanagerAlert{{Fingerprint: "test"}},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/alertmanager/invalid-key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestAlertmanagerWebhook_InvalidPayload tests bad request with invalid payload.
func TestAlertmanagerWebhook_InvalidPayload(t *testing.T) {
	_, router, _, _ := setupTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/alertmanager/valid-key", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestAlertmanagerWebhook_EmptyAlerts tests bad request with no alerts.
func TestAlertmanagerWebhook_EmptyAlerts(t *testing.T) {
	_, router, _, _ := setupTestHandler()

	payload := AlertmanagerPayload{
		Alerts: []AlertmanagerAlert{},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/alertmanager/valid-key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestAlertmanagerWebhook_ResolvedAlert tests processing a resolved alert.
func TestAlertmanagerWebhook_ResolvedAlert(t *testing.T) {
	_, router, alertStore, _ := setupTestHandler()

	endTime := time.Now()
	payload := AlertmanagerPayload{
		Version: "4",
		Status:  "resolved",
		Alerts: []AlertmanagerAlert{
			{
				Status:      "resolved",
				Labels:      map[string]string{"alertname": "TestAlert"},
				Annotations: map[string]string{"summary": "Test resolved"},
				StartsAt:    time.Now().Add(-1 * time.Hour),
				EndsAt:      endTime,
				Fingerprint: "resolved123",
			},
		},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/alertmanager/valid-key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify alert status
	alert := alertStore.alertsByFP["resolved123"]
	if alert == nil {
		t.Fatal("alert not found in store")
	}
	if alert.Status != alertingv1.AlertStatus_ALERT_STATUS_RESOLVED {
		t.Errorf("expected resolved status, got %v", alert.Status)
	}
	if alert.ResolvedAt == nil {
		t.Error("expected resolved_at to be set")
	}
}

// TestGrafanaWebhook_Success tests successful Grafana webhook processing.
func TestGrafanaWebhook_Success(t *testing.T) {
	_, router, alertStore, _ := setupTestHandler()

	payload := GrafanaPayload{
		Title:    "High CPU Usage",
		RuleID:   123,
		RuleName: "CPU Alert",
		RuleURL:  "http://grafana/alerts/123",
		State:    "alerting",
		Message:  "CPU usage is above 90%",
		Tags:     map[string]string{"severity": "high", "env": "prod"},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/grafana/valid-key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp WebhookResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp.AlertIds) != 1 {
		t.Errorf("expected 1 alert ID, got %d", len(resp.AlertIds))
	}

	// Verify alert was stored with correct properties
	if len(alertStore.alerts) != 1 {
		t.Errorf("expected 1 alert in store, got %d", len(alertStore.alerts))
	}

	for _, alert := range alertStore.alerts {
		if alert.Summary != "High CPU Usage" {
			t.Errorf("expected summary 'High CPU Usage', got '%s'", alert.Summary)
		}
		if alert.Source != alertingv1.AlertSource_ALERT_SOURCE_GRAFANA {
			t.Errorf("expected Grafana source, got %v", alert.Source)
		}
		if alert.Severity != alertingv1.Severity_SEVERITY_HIGH {
			t.Errorf("expected high severity, got %v", alert.Severity)
		}
	}
}

// TestGrafanaWebhook_InvalidKey tests unauthorized access.
func TestGrafanaWebhook_InvalidKey(t *testing.T) {
	_, router, _, _ := setupTestHandler()

	payload := GrafanaPayload{Title: "Test"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/grafana/invalid-key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestGrafanaWebhook_MissingTitle tests bad request with missing title.
func TestGrafanaWebhook_MissingTitle(t *testing.T) {
	_, router, _, _ := setupTestHandler()

	payload := GrafanaPayload{} // No title or ruleName
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/grafana/valid-key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestGrafanaWebhook_OkState tests processing an OK/resolved state.
func TestGrafanaWebhook_OkState(t *testing.T) {
	_, router, alertStore, _ := setupTestHandler()

	payload := GrafanaPayload{
		Title:   "CPU Alert Resolved",
		RuleID:  456,
		State:   "ok",
		Message: "CPU back to normal",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/grafana/valid-key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify alert status is resolved
	for _, alert := range alertStore.alerts {
		if alert.Status != alertingv1.AlertStatus_ALERT_STATUS_RESOLVED {
			t.Errorf("expected resolved status, got %v", alert.Status)
		}
	}
}

// TestGenericWebhook_Success tests successful generic webhook processing.
func TestGenericWebhook_Success(t *testing.T) {
	_, router, alertStore, _ := setupTestHandler()

	payload := GenericPayload{
		Summary:  "Custom alert from monitoring system",
		Details:  "Detailed description of the issue",
		Severity: "critical",
		Status:   "triggered",
		Labels:   map[string]string{"env": "production", "team": "platform"},
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/generic/valid-key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp WebhookResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp.AlertIds) != 1 {
		t.Errorf("expected 1 alert ID, got %d", len(resp.AlertIds))
	}

	// Verify alert properties
	for _, alert := range alertStore.alerts {
		if alert.Summary != "Custom alert from monitoring system" {
			t.Errorf("expected correct summary, got '%s'", alert.Summary)
		}
		if alert.Severity != alertingv1.Severity_SEVERITY_CRITICAL {
			t.Errorf("expected critical severity, got %v", alert.Severity)
		}
		if alert.Source != alertingv1.AlertSource_ALERT_SOURCE_GENERIC {
			t.Errorf("expected generic source, got %v", alert.Source)
		}
	}
}

// TestGenericWebhook_MissingSummary tests bad request with missing summary.
func TestGenericWebhook_MissingSummary(t *testing.T) {
	_, router, _, _ := setupTestHandler()

	payload := map[string]string{
		"details": "Some details without summary",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/generic/valid-key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestGenericWebhook_CustomFingerprint tests using a custom fingerprint.
func TestGenericWebhook_CustomFingerprint(t *testing.T) {
	_, router, alertStore, _ := setupTestHandler()

	payload := GenericPayload{
		Summary:     "Alert with custom fingerprint",
		Fingerprint: "custom-fp-12345",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/generic/valid-key", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify fingerprint was used
	alert := alertStore.alertsByFP["custom-fp-12345"]
	if alert == nil {
		t.Error("alert with custom fingerprint not found")
	}
}

// TestGenericWebhook_Deduplication tests that same fingerprint updates existing alert.
func TestGenericWebhook_Deduplication(t *testing.T) {
	_, router, alertStore, _ := setupTestHandler()

	// First alert
	payload1 := GenericPayload{
		Summary:     "Dedup test alert",
		Fingerprint: "dedup-fp",
		Status:      "triggered",
	}

	body1, _ := json.Marshal(payload1)
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/generic/valid-key", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")

	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("first request: expected status 200, got %d", w1.Code)
	}

	var resp1 WebhookResponse
	json.Unmarshal(w1.Body.Bytes(), &resp1)
	if resp1.Created != 1 {
		t.Errorf("first request: expected created=1, got %d", resp1.Created)
	}

	// Second alert with same fingerprint
	payload2 := GenericPayload{
		Summary:     "Dedup test alert - updated",
		Fingerprint: "dedup-fp",
		Status:      "resolved",
	}

	body2, _ := json.Marshal(payload2)
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/webhook/generic/valid-key", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")

	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("second request: expected status 200, got %d", w2.Code)
	}

	var resp2 WebhookResponse
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	if resp2.Updated != 1 {
		t.Errorf("second request: expected updated=1, got %d", resp2.Updated)
	}

	// Should still only have 1 alert
	if len(alertStore.alertsByFP) != 1 {
		t.Errorf("expected 1 alert after dedup, got %d", len(alertStore.alertsByFP))
	}

	// Alert should be updated
	alert := alertStore.alertsByFP["dedup-fp"]
	if alert.Status != alertingv1.AlertStatus_ALERT_STATUS_RESOLVED {
		t.Errorf("expected resolved status after update, got %v", alert.Status)
	}
}

// TestMapAlertmanagerStatus tests status mapping.
func TestMapAlertmanagerStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected alertingv1.AlertStatus
	}{
		{"firing", alertingv1.AlertStatus_ALERT_STATUS_TRIGGERED},
		{"resolved", alertingv1.AlertStatus_ALERT_STATUS_RESOLVED},
		{"unknown", alertingv1.AlertStatus_ALERT_STATUS_TRIGGERED},
		{"", alertingv1.AlertStatus_ALERT_STATUS_TRIGGERED},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapAlertmanagerStatus(tt.input)
			if result != tt.expected {
				t.Errorf("mapAlertmanagerStatus(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestExtractSeverity tests severity extraction from labels.
func TestExtractSeverity(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected alertingv1.Severity
	}{
		{"critical", map[string]string{"severity": "critical"}, alertingv1.Severity_SEVERITY_CRITICAL},
		{"high", map[string]string{"severity": "high"}, alertingv1.Severity_SEVERITY_HIGH},
		{"warning", map[string]string{"severity": "warning"}, alertingv1.Severity_SEVERITY_HIGH},
		{"medium", map[string]string{"severity": "medium"}, alertingv1.Severity_SEVERITY_MEDIUM},
		{"low", map[string]string{"severity": "low"}, alertingv1.Severity_SEVERITY_LOW},
		{"info", map[string]string{"severity": "info"}, alertingv1.Severity_SEVERITY_INFO},
		{"no severity", map[string]string{"alertname": "test"}, alertingv1.Severity_SEVERITY_MEDIUM},
		{"empty labels", nil, alertingv1.Severity_SEVERITY_MEDIUM},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSeverity(tt.labels)
			if result != tt.expected {
				t.Errorf("extractSeverity() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestParseGenericStatus tests generic status parsing.
func TestParseGenericStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected alertingv1.AlertStatus
	}{
		{"triggered", alertingv1.AlertStatus_ALERT_STATUS_TRIGGERED},
		{"firing", alertingv1.AlertStatus_ALERT_STATUS_TRIGGERED},
		{"alerting", alertingv1.AlertStatus_ALERT_STATUS_TRIGGERED},
		{"resolved", alertingv1.AlertStatus_ALERT_STATUS_RESOLVED},
		{"ok", alertingv1.AlertStatus_ALERT_STATUS_RESOLVED},
		{"acknowledged", alertingv1.AlertStatus_ALERT_STATUS_ACKNOWLEDGED},
		{"acked", alertingv1.AlertStatus_ALERT_STATUS_ACKNOWLEDGED},
		{"suppressed", alertingv1.AlertStatus_ALERT_STATUS_SUPPRESSED},
		{"silenced", alertingv1.AlertStatus_ALERT_STATUS_SUPPRESSED},
		{"unknown", alertingv1.AlertStatus_ALERT_STATUS_TRIGGERED},
		{"", alertingv1.AlertStatus_ALERT_STATUS_TRIGGERED},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseGenericStatus(tt.input)
			if result != tt.expected {
				t.Errorf("parseGenericStatus(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestParseGenericSeverity tests generic severity parsing.
func TestParseGenericSeverity(t *testing.T) {
	tests := []struct {
		input    string
		expected alertingv1.Severity
	}{
		{"critical", alertingv1.Severity_SEVERITY_CRITICAL},
		{"crit", alertingv1.Severity_SEVERITY_CRITICAL},
		{"p1", alertingv1.Severity_SEVERITY_CRITICAL},
		{"high", alertingv1.Severity_SEVERITY_HIGH},
		{"warning", alertingv1.Severity_SEVERITY_HIGH},
		{"p2", alertingv1.Severity_SEVERITY_HIGH},
		{"medium", alertingv1.Severity_SEVERITY_MEDIUM},
		{"p3", alertingv1.Severity_SEVERITY_MEDIUM},
		{"low", alertingv1.Severity_SEVERITY_LOW},
		{"p4", alertingv1.Severity_SEVERITY_LOW},
		{"info", alertingv1.Severity_SEVERITY_INFO},
		{"informational", alertingv1.Severity_SEVERITY_INFO},
		{"p5", alertingv1.Severity_SEVERITY_INFO},
		{"unknown", alertingv1.Severity_SEVERITY_MEDIUM},
		{"", alertingv1.Severity_SEVERITY_MEDIUM},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseGenericSeverity(tt.input)
			if result != tt.expected {
				t.Errorf("parseGenericSeverity(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestGenerateGrafanaFingerprint tests fingerprint generation consistency.
func TestGenerateGrafanaFingerprint(t *testing.T) {
	payload1 := &GrafanaPayload{
		RuleID: 123,
		Tags:   map[string]string{"env": "prod", "team": "platform"},
	}
	payload2 := &GrafanaPayload{
		RuleID: 123,
		Tags:   map[string]string{"team": "platform", "env": "prod"}, // Same tags, different order
	}
	payload3 := &GrafanaPayload{
		RuleID: 456, // Different rule ID
		Tags:   map[string]string{"env": "prod", "team": "platform"},
	}

	fp1 := generateGrafanaFingerprint(payload1)
	fp2 := generateGrafanaFingerprint(payload2)
	fp3 := generateGrafanaFingerprint(payload3)

	// Same rule and tags should produce same fingerprint
	if fp1 != fp2 {
		t.Errorf("fingerprints should match for same rule+tags: %s vs %s", fp1, fp2)
	}

	// Different rule should produce different fingerprint
	if fp1 == fp3 {
		t.Errorf("fingerprints should differ for different rule IDs")
	}
}
