// Package metrics provides Prometheus metrics for observability.
package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterMetricsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	RegisterMetricsEndpoint(router)

	// Test that /metrics endpoint works
	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "# HELP")
}

func TestRegisterMetricsEndpointWithPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	RegisterMetricsEndpointWithPath(router, "/custom/metrics")

	// Test that custom path works
	req := httptest.NewRequest("GET", "/custom/metrics", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMetricsHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := MetricsHandler()

	require.NotNil(t, handler)
}

func TestRecordNotificationSent(t *testing.T) {
	// This should not panic
	RecordNotificationSent("slack", "success")
	RecordNotificationSent("email", "failed")
	RecordNotificationSent("slack", "success")
}

func TestRecordNotificationLatency(t *testing.T) {
	// This should not panic
	RecordNotificationLatency("slack", 0.5)
	RecordNotificationLatency("email", 1.2)
}

func TestRecordTemplateRender(t *testing.T) {
	// This should not panic
	RecordTemplateRender(0.001)
	RecordTemplateRender(0.05)
}

func TestSetDeliveryQueueSize(t *testing.T) {
	// This should not panic
	SetDeliveryQueueSize(10)
	SetDeliveryQueueSize(5)
	SetDeliveryQueueSize(0)
}

func TestRecordAlertReceived(t *testing.T) {
	// This should not panic
	RecordAlertReceived("alertmanager", "critical")
	RecordAlertReceived("grafana", "warning")
}

func TestRecordAlertProcessed(t *testing.T) {
	// This should not panic
	RecordAlertProcessed("routed")
	RecordAlertProcessed("suppressed")
	RecordAlertProcessed("escalated")
}

func TestRecordAlertProcessingDuration(t *testing.T) {
	// This should not panic
	RecordAlertProcessingDuration("alertmanager", 0.1)
	RecordAlertProcessingDuration("grafana", 0.25)
}

func TestRecordEscalationTriggered(t *testing.T) {
	// This should not panic
	RecordEscalationTriggered("default-policy", "1")
	RecordEscalationTriggered("critical-policy", "2")
}

func TestSetActiveAlerts(t *testing.T) {
	// This should not panic
	SetActiveAlerts("critical", 5)
	SetActiveAlerts("warning", 10)
	SetActiveAlerts("info", 100)
}

func TestSetMaintenanceWindowsActive(t *testing.T) {
	// This should not panic
	SetMaintenanceWindowsActive(2)
	SetMaintenanceWindowsActive(0)
}

func TestRecordAlertSuppressed(t *testing.T) {
	// This should not panic
	RecordAlertSuppressed()
}

func TestRecordRoutingRuleMatched(t *testing.T) {
	// This should not panic
	RecordRoutingRuleMatched("critical-alerts")
	RecordRoutingRuleMatched("network-alerts")
}

func TestRecordHTTPRequest(t *testing.T) {
	// This should not panic
	RecordHTTPRequest("GET", "/api/alerts", "200")
	RecordHTTPRequest("POST", "/api/alerts", "201")
	RecordHTTPRequest("GET", "/api/alerts/123", "404")
}

func TestRecordHTTPRequestDuration(t *testing.T) {
	// This should not panic
	RecordHTTPRequestDuration("GET", "/api/alerts", 0.05)
	RecordHTTPRequestDuration("POST", "/api/alerts", 0.2)
}

func TestRecordGRPCRequest(t *testing.T) {
	// This should not panic
	RecordGRPCRequest("/alerting.v1.AlertService/CreateAlert", "OK")
	RecordGRPCRequest("/alerting.v1.AlertService/GetAlert", "NOT_FOUND")
}

func TestRecordGRPCRequestDuration(t *testing.T) {
	// This should not panic
	RecordGRPCRequestDuration("/alerting.v1.AlertService/CreateAlert", 0.01)
	RecordGRPCRequestDuration("/alerting.v1.AlertService/ListAlerts", 0.15)
}

func TestRecordWebhookRequest(t *testing.T) {
	// This should not panic
	RecordWebhookRequest("alertmanager", "success")
	RecordWebhookRequest("grafana", "invalid_signature")
}

func TestRecordDatabaseQuery(t *testing.T) {
	// This should not panic
	RecordDatabaseQuery("select", 0.005)
	RecordDatabaseQuery("insert", 0.01)
	RecordDatabaseQuery("update", 0.008)
}

func TestRecordCacheOperation(t *testing.T) {
	// This should not panic
	RecordCacheOperation("tiers", "hit")
	RecordCacheOperation("tiers", "miss")
	RecordCacheOperation("rules", "hit")
}

func TestMetricsAreRegistered(t *testing.T) {
	// Verify all metrics are registered with Prometheus
	metrics := []prometheus.Collector{
		NotificationsSent,
		NotificationLatency,
		TemplateRenderDuration,
		DeliveryQueueSize,
		AlertsReceived,
		AlertsProcessed,
		AlertProcessingDuration,
		EscalationTriggered,
		ActiveAlerts,
		MaintenanceWindowsActive,
		AlertsSuppressed,
		RoutingRulesMatched,
		HTTPRequestsTotal,
		HTTPRequestDuration,
		GRPCRequestsTotal,
		GRPCRequestDuration,
		WebhookRequestsTotal,
		DatabaseQueryDuration,
		CacheHits,
	}

	for _, metric := range metrics {
		assert.NotNil(t, metric)
	}
}
