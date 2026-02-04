// Package metrics provides Prometheus metrics for observability.
package metrics

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// NotificationsSent tracks total notifications sent by channel and status.
	NotificationsSent = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "notifications_sent_total",
			Help: "Total notifications sent by channel and status",
		},
		[]string{"channel", "status"},
	)

	// NotificationLatency tracks notification delivery latency.
	NotificationLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "notification_latency_seconds",
			Help:    "Notification delivery latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"channel"},
	)

	// TemplateRenderDuration tracks template rendering duration.
	TemplateRenderDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "template_render_duration_seconds",
			Help:    "Template rendering duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
	)

	// DeliveryQueueSize tracks current size of the delivery queue.
	DeliveryQueueSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "delivery_queue_size",
			Help: "Current size of delivery queue",
		},
	)

	// AlertsReceived tracks total alerts received by source and severity.
	AlertsReceived = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alerts_received_total",
			Help: "Total alerts received by source and severity",
		},
		[]string{"source", "severity"},
	)

	// AlertsProcessed tracks total alerts processed by status.
	AlertsProcessed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "alerts_processed_total",
			Help: "Total alerts processed by status",
		},
		[]string{"status"},
	)

	// AlertProcessingDuration tracks alert processing duration.
	AlertProcessingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "alert_processing_duration_seconds",
			Help:    "Alert processing duration in seconds",
			Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"source"},
	)

	// EscalationTriggered tracks escalation events.
	EscalationTriggered = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "escalation_triggered_total",
			Help: "Total escalation events triggered",
		},
		[]string{"policy", "step"},
	)

	// ActiveAlerts tracks currently active alerts by severity.
	ActiveAlerts = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "active_alerts",
			Help: "Current number of active alerts by severity",
		},
		[]string{"severity"},
	)

	// MaintenanceWindowsActive tracks currently active maintenance windows.
	MaintenanceWindowsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "maintenance_windows_active",
			Help: "Current number of active maintenance windows",
		},
	)

	// AlertsSuppressed tracks alerts suppressed by maintenance windows.
	AlertsSuppressed = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "alerts_suppressed_total",
			Help: "Total alerts suppressed by maintenance windows",
		},
	)

	// RoutingRulesMatched tracks routing rules matched.
	RoutingRulesMatched = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "routing_rules_matched_total",
			Help: "Total routing rules matched by rule name",
		},
		[]string{"rule"},
	)

	// HTTPRequestsTotal tracks total HTTP requests.
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests by method, path, and status",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestDuration tracks HTTP request duration.
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// GRPCRequestsTotal tracks total gRPC requests.
	GRPCRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Total gRPC requests by method and status",
		},
		[]string{"method", "status"},
	)

	// GRPCRequestDuration tracks gRPC request duration.
	GRPCRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "grpc_request_duration_seconds",
			Help:    "gRPC request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method"},
	)

	// WebhookRequestsTotal tracks total webhook requests received.
	WebhookRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webhook_requests_total",
			Help: "Total webhook requests received by source and status",
		},
		[]string{"source", "status"},
	)

	// DatabaseQueryDuration tracks database query duration.
	DatabaseQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "database_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"operation"},
	)

	// CacheHits tracks cache hit/miss ratio.
	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cache_operations_total",
			Help: "Total cache operations by type (hit/miss)",
		},
		[]string{"cache", "result"},
	)
)

// RegisterMetricsEndpoint registers the /metrics endpoint on a Gin router.
func RegisterMetricsEndpoint(router *gin.Engine) {
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// RegisterMetricsEndpointWithPath registers the metrics endpoint at a custom path.
func RegisterMetricsEndpointWithPath(router *gin.Engine, path string) {
	router.GET(path, gin.WrapH(promhttp.Handler()))
}

// MetricsHandler returns the Prometheus HTTP handler.
func MetricsHandler() gin.HandlerFunc {
	return gin.WrapH(promhttp.Handler())
}

// RecordNotificationSent records a notification sent event.
func RecordNotificationSent(channel, status string) {
	NotificationsSent.WithLabelValues(channel, status).Inc()
}

// RecordNotificationLatency records notification delivery latency.
func RecordNotificationLatency(channel string, seconds float64) {
	NotificationLatency.WithLabelValues(channel).Observe(seconds)
}

// RecordTemplateRender records template rendering duration.
func RecordTemplateRender(seconds float64) {
	TemplateRenderDuration.Observe(seconds)
}

// SetDeliveryQueueSize sets the current delivery queue size.
func SetDeliveryQueueSize(size float64) {
	DeliveryQueueSize.Set(size)
}

// RecordAlertReceived records an alert received event.
func RecordAlertReceived(source, severity string) {
	AlertsReceived.WithLabelValues(source, severity).Inc()
}

// RecordAlertProcessed records an alert processed event.
func RecordAlertProcessed(status string) {
	AlertsProcessed.WithLabelValues(status).Inc()
}

// RecordAlertProcessingDuration records alert processing duration.
func RecordAlertProcessingDuration(source string, seconds float64) {
	AlertProcessingDuration.WithLabelValues(source).Observe(seconds)
}

// RecordEscalationTriggered records an escalation triggered event.
func RecordEscalationTriggered(policy, step string) {
	EscalationTriggered.WithLabelValues(policy, step).Inc()
}

// SetActiveAlerts sets the number of active alerts by severity.
func SetActiveAlerts(severity string, count float64) {
	ActiveAlerts.WithLabelValues(severity).Set(count)
}

// SetMaintenanceWindowsActive sets the number of active maintenance windows.
func SetMaintenanceWindowsActive(count float64) {
	MaintenanceWindowsActive.Set(count)
}

// RecordAlertSuppressed records an alert suppressed event.
func RecordAlertSuppressed() {
	AlertsSuppressed.Inc()
}

// RecordRoutingRuleMatched records a routing rule matched event.
func RecordRoutingRuleMatched(rule string) {
	RoutingRulesMatched.WithLabelValues(rule).Inc()
}

// RecordHTTPRequest records an HTTP request.
func RecordHTTPRequest(method, path, status string) {
	HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
}

// RecordHTTPRequestDuration records HTTP request duration.
func RecordHTTPRequestDuration(method, path string, seconds float64) {
	HTTPRequestDuration.WithLabelValues(method, path).Observe(seconds)
}

// RecordGRPCRequest records a gRPC request.
func RecordGRPCRequest(method, status string) {
	GRPCRequestsTotal.WithLabelValues(method, status).Inc()
}

// RecordGRPCRequestDuration records gRPC request duration.
func RecordGRPCRequestDuration(method string, seconds float64) {
	GRPCRequestDuration.WithLabelValues(method).Observe(seconds)
}

// RecordWebhookRequest records a webhook request.
func RecordWebhookRequest(source, status string) {
	WebhookRequestsTotal.WithLabelValues(source, status).Inc()
}

// RecordDatabaseQuery records a database query duration.
func RecordDatabaseQuery(operation string, seconds float64) {
	DatabaseQueryDuration.WithLabelValues(operation).Observe(seconds)
}

// RecordCacheOperation records a cache operation.
func RecordCacheOperation(cache, result string) {
	CacheHits.WithLabelValues(cache, result).Inc()
}
