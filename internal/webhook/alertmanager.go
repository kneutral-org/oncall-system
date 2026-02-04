package webhook

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	alertingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/v1"
)

// AlertmanagerPayload represents the webhook payload from Alertmanager.
type AlertmanagerPayload struct {
	Version           string                 `json:"version"`
	GroupKey          string                 `json:"groupKey"`
	TruncatedAlerts   int                    `json:"truncatedAlerts,omitempty"`
	Status            string                 `json:"status"`
	Receiver          string                 `json:"receiver"`
	GroupLabels       map[string]string      `json:"groupLabels"`
	CommonLabels      map[string]string      `json:"commonLabels"`
	CommonAnnotations map[string]string      `json:"commonAnnotations"`
	ExternalURL       string                 `json:"externalURL"`
	Alerts            []AlertmanagerAlert    `json:"alerts"`
}

// AlertmanagerAlert represents a single alert in the Alertmanager payload.
type AlertmanagerAlert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL,omitempty"`
	Fingerprint  string            `json:"fingerprint"`
}

// AlertmanagerWebhook handles POST /api/v1/webhook/alertmanager/:integration_key
func (h *Handler) AlertmanagerWebhook(c *gin.Context) {
	// Validate integration key
	service := h.validateIntegrationKey(c)
	if service == nil {
		return
	}

	// Parse payload
	var payload AlertmanagerPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		h.logger.Error().Err(err).Msg("failed to parse alertmanager payload")
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "badRequest",
			Message: "invalid alertmanager payload: " + err.Error(),
		})
		return
	}

	// Validate payload
	if len(payload.Alerts) == 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "badRequest",
			Message: "no alerts in payload",
		})
		return
	}

	h.logger.Info().
		Str("serviceId", service.ID).
		Str("groupKey", payload.GroupKey).
		Int("alertCount", len(payload.Alerts)).
		Msg("processing alertmanager webhook")

	var alertIds []string
	var created, updated int

	// Process each alert
	for _, amAlert := range payload.Alerts {
		alert, wasCreated, err := h.processAlertmanagerAlert(c, service.ID, &amAlert, &payload)
		if err != nil {
			h.logger.Error().
				Err(err).
				Str("fingerprint", amAlert.Fingerprint).
				Msg("failed to process alertmanager alert")
			continue
		}
		alertIds = append(alertIds, alert.Id)
		if wasCreated {
			created++
		} else {
			updated++
		}
	}

	c.JSON(http.StatusOK, WebhookResponse{
		Message:  "alerts processed successfully",
		AlertIds: alertIds,
		Created:  created,
		Updated:  updated,
	})
}

func (h *Handler) processAlertmanagerAlert(c *gin.Context, serviceID string, amAlert *AlertmanagerAlert, payload *AlertmanagerPayload) (*alertingv1.Alert, bool, error) {
	// Map Alertmanager status to internal status
	status := mapAlertmanagerStatus(amAlert.Status)

	// Extract severity from labels
	severity := extractSeverity(amAlert.Labels)

	// Build summary from alertname and annotations
	summary := buildAlertmanagerSummary(amAlert)

	// Build details from annotations
	details := ""
	if desc, ok := amAlert.Annotations["description"]; ok {
		details = desc
	}

	// Create raw payload for storage
	rawPayload, _ := structpb.NewStruct(map[string]interface{}{
		"version":     payload.Version,
		"groupKey":    payload.GroupKey,
		"receiver":    payload.Receiver,
		"externalURL": payload.ExternalURL,
		"alert": map[string]interface{}{
			"status":       amAlert.Status,
			"fingerprint":  amAlert.Fingerprint,
			"generatorURL": amAlert.GeneratorURL,
		},
	})

	alert := &alertingv1.Alert{
		Fingerprint:  amAlert.Fingerprint,
		Summary:      summary,
		Details:      details,
		Severity:     severity,
		Source:       alertingv1.AlertSource_ALERT_SOURCE_ALERTMANAGER,
		ServiceId:    serviceID,
		Labels:       amAlert.Labels,
		Annotations:  amAlert.Annotations,
		Status:       status,
		TriggeredAt:  timestamppb.New(amAlert.StartsAt),
		RawPayload:   rawPayload,
	}

	// Set resolved_at if the alert is resolved
	if status == alertingv1.AlertStatus_ALERT_STATUS_RESOLVED && !amAlert.EndsAt.IsZero() {
		alert.ResolvedAt = timestamppb.New(amAlert.EndsAt)
	}

	return h.alertStore.CreateOrUpdate(c.Request.Context(), alert)
}

func mapAlertmanagerStatus(status string) alertingv1.AlertStatus {
	switch status {
	case "firing":
		return alertingv1.AlertStatus_ALERT_STATUS_TRIGGERED
	case "resolved":
		return alertingv1.AlertStatus_ALERT_STATUS_RESOLVED
	default:
		return alertingv1.AlertStatus_ALERT_STATUS_TRIGGERED
	}
}

func extractSeverity(labels map[string]string) alertingv1.Severity {
	severityStr, ok := labels["severity"]
	if !ok {
		return alertingv1.Severity_SEVERITY_MEDIUM
	}

	switch severityStr {
	case "critical":
		return alertingv1.Severity_SEVERITY_CRITICAL
	case "high", "warning":
		return alertingv1.Severity_SEVERITY_HIGH
	case "medium":
		return alertingv1.Severity_SEVERITY_MEDIUM
	case "low":
		return alertingv1.Severity_SEVERITY_LOW
	case "info", "informational":
		return alertingv1.Severity_SEVERITY_INFO
	default:
		return alertingv1.Severity_SEVERITY_MEDIUM
	}
}

func buildAlertmanagerSummary(alert *AlertmanagerAlert) string {
	// Try summary annotation first
	if summary, ok := alert.Annotations["summary"]; ok && summary != "" {
		return summary
	}

	// Fall back to alertname label
	if alertname, ok := alert.Labels["alertname"]; ok && alertname != "" {
		return alertname
	}

	return "Alert from Alertmanager"
}
