package webhook

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	alertingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/v1"
)

// GrafanaPayload represents the webhook payload from Grafana alerting.
type GrafanaPayload struct {
	Title       string            `json:"title"`
	RuleID      int64             `json:"ruleId"`
	RuleName    string            `json:"ruleName"`
	RuleURL     string            `json:"ruleUrl"`
	State       string            `json:"state"`
	Message     string            `json:"message"`
	EvalMatches []GrafanaMatch    `json:"evalMatches,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	OrgID       int64             `json:"orgId,omitempty"`
	DashboardID int64             `json:"dashboardId,omitempty"`
	PanelID     int64             `json:"panelId,omitempty"`
	ImageURL    string            `json:"imageUrl,omitempty"`
}

// GrafanaMatch represents a metric match from Grafana evaluation.
type GrafanaMatch struct {
	Metric string      `json:"metric"`
	Value  interface{} `json:"value"`
	Tags   map[string]string `json:"tags,omitempty"`
}

// GrafanaWebhook handles POST /api/v1/webhook/grafana/:integration_key
func (h *Handler) GrafanaWebhook(c *gin.Context) {
	// Validate integration key
	service := h.validateIntegrationKey(c)
	if service == nil {
		return
	}

	// Parse payload
	var payload GrafanaPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		h.logger.Error().Err(err).Msg("failed to parse grafana payload")
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "badRequest",
			Message: "invalid grafana payload: " + err.Error(),
		})
		return
	}

	// Validate payload
	if payload.RuleName == "" && payload.Title == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "badRequest",
			Message: "rule name or title is required",
		})
		return
	}

	h.logger.Info().
		Str("serviceId", service.ID).
		Str("ruleName", payload.RuleName).
		Str("state", payload.State).
		Msg("processing grafana webhook")

	alert, wasCreated, err := h.processGrafanaAlert(c, service.ID, &payload)
	if err != nil {
		h.logger.Error().
			Err(err).
			Str("ruleName", payload.RuleName).
			Msg("failed to process grafana alert")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "internalError",
			Message: "failed to process alert: " + err.Error(),
		})
		return
	}

	created := 0
	updated := 0
	if wasCreated {
		created = 1
	} else {
		updated = 1
	}

	c.JSON(http.StatusOK, WebhookResponse{
		Message:  "alert processed successfully",
		AlertIds: []string{alert.Id},
		Created:  created,
		Updated:  updated,
	})
}

func (h *Handler) processGrafanaAlert(c *gin.Context, serviceID string, payload *GrafanaPayload) (*alertingv1.Alert, bool, error) {
	// Map Grafana state to internal status
	status := mapGrafanaState(payload.State)

	// Extract severity from tags
	severity := extractGrafanaSeverity(payload.Tags)

	// Generate fingerprint from ruleId + tags
	fingerprint := generateGrafanaFingerprint(payload)

	// Build summary
	summary := payload.Title
	if summary == "" {
		summary = payload.RuleName
	}

	// Build labels from tags
	labels := make(map[string]string)
	for k, v := range payload.Tags {
		labels[k] = v
	}
	labels["ruleId"] = fmt.Sprintf("%d", payload.RuleID)
	labels["ruleName"] = payload.RuleName

	// Build annotations
	annotations := map[string]string{
		"ruleUrl": payload.RuleURL,
	}
	if payload.ImageURL != "" {
		annotations["imageUrl"] = payload.ImageURL
	}

	// Create raw payload for storage
	rawPayloadMap := map[string]interface{}{
		"title":       payload.Title,
		"ruleId":      payload.RuleID,
		"ruleName":    payload.RuleName,
		"ruleUrl":     payload.RuleURL,
		"state":       payload.State,
		"message":     payload.Message,
		"orgId":       payload.OrgID,
		"dashboardId": payload.DashboardID,
		"panelId":     payload.PanelID,
	}
	if len(payload.EvalMatches) > 0 {
		matches := make([]interface{}, len(payload.EvalMatches))
		for i, m := range payload.EvalMatches {
			matches[i] = map[string]interface{}{
				"metric": m.Metric,
				"value":  m.Value,
			}
		}
		rawPayloadMap["evalMatches"] = matches
	}
	rawPayload, _ := structpb.NewStruct(rawPayloadMap)

	alert := &alertingv1.Alert{
		Fingerprint:  fingerprint,
		Summary:      summary,
		Details:      payload.Message,
		Severity:     severity,
		Source:       alertingv1.AlertSource_ALERT_SOURCE_GRAFANA,
		ServiceId:    serviceID,
		Labels:       labels,
		Annotations:  annotations,
		Status:       status,
		TriggeredAt:  timestamppb.New(time.Now()),
		RawPayload:   rawPayload,
	}

	// Set resolved_at if the alert is resolved
	if status == alertingv1.AlertStatus_ALERT_STATUS_RESOLVED {
		alert.ResolvedAt = timestamppb.Now()
	}

	return h.alertStore.CreateOrUpdate(c.Request.Context(), alert)
}

func mapGrafanaState(state string) alertingv1.AlertStatus {
	switch state {
	case "alerting":
		return alertingv1.AlertStatus_ALERT_STATUS_TRIGGERED
	case "ok":
		return alertingv1.AlertStatus_ALERT_STATUS_RESOLVED
	case "no_data":
		return alertingv1.AlertStatus_ALERT_STATUS_TRIGGERED
	case "paused":
		return alertingv1.AlertStatus_ALERT_STATUS_SUPPRESSED
	default:
		return alertingv1.AlertStatus_ALERT_STATUS_TRIGGERED
	}
}

func extractGrafanaSeverity(tags map[string]string) alertingv1.Severity {
	if tags == nil {
		return alertingv1.Severity_SEVERITY_MEDIUM
	}

	severityStr, ok := tags["severity"]
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

func generateGrafanaFingerprint(payload *GrafanaPayload) string {
	// Create a deterministic string from ruleId and sorted tags
	data := fmt.Sprintf("grafana:%d:", payload.RuleID)

	if payload.Tags != nil && len(payload.Tags) > 0 {
		keys := make([]string, 0, len(payload.Tags))
		for k := range payload.Tags {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			data += fmt.Sprintf("%s=%s,", k, payload.Tags[k])
		}
	}

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes (32 hex chars)
}
