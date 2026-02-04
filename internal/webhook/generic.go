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

// GenericPayload represents a flexible generic webhook payload.
type GenericPayload struct {
	Summary     string            `json:"summary" binding:"required"`
	Details     string            `json:"details,omitempty"`
	Severity    string            `json:"severity,omitempty"`
	Status      string            `json:"status,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Fingerprint string            `json:"fingerprint,omitempty"`
	Source      string            `json:"source,omitempty"`
	Timestamp   *time.Time        `json:"timestamp,omitempty"`
}

// GenericWebhook handles POST /api/v1/webhook/generic/:integration_key
func (h *Handler) GenericWebhook(c *gin.Context) {
	// Validate integration key
	service := h.validateIntegrationKey(c)
	if service == nil {
		return
	}

	// Parse payload
	var payload GenericPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		h.logger.Error().Err(err).Msg("failed to parse generic payload")
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "badRequest",
			Message: "invalid generic payload: " + err.Error(),
		})
		return
	}

	// Summary is required (enforced by binding)
	if payload.Summary == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "badRequest",
			Message: "summary is required",
		})
		return
	}

	h.logger.Info().
		Str("serviceId", service.ID).
		Str("summary", payload.Summary).
		Msg("processing generic webhook")

	alert, wasCreated, err := h.processGenericAlert(c, service.ID, &payload)
	if err != nil {
		h.logger.Error().
			Err(err).
			Str("summary", payload.Summary).
			Msg("failed to process generic alert")
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

func (h *Handler) processGenericAlert(c *gin.Context, serviceID string, payload *GenericPayload) (*alertingv1.Alert, bool, error) {
	// Parse or default status
	status := parseGenericStatus(payload.Status)

	// Parse or default severity
	severity := parseGenericSeverity(payload.Severity)

	// Use provided fingerprint or generate one
	fingerprint := payload.Fingerprint
	if fingerprint == "" {
		fingerprint = generateGenericFingerprint(serviceID, payload)
	}

	// Set timestamp
	triggeredAt := time.Now()
	if payload.Timestamp != nil {
		triggeredAt = *payload.Timestamp
	}

	// Ensure labels map exists
	labels := payload.Labels
	if labels == nil {
		labels = make(map[string]string)
	}

	// Ensure annotations map exists
	annotations := payload.Annotations
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Create raw payload for storage
	rawPayloadMap := map[string]interface{}{
		"summary": payload.Summary,
	}
	if payload.Details != "" {
		rawPayloadMap["details"] = payload.Details
	}
	if payload.Severity != "" {
		rawPayloadMap["severity"] = payload.Severity
	}
	if payload.Status != "" {
		rawPayloadMap["status"] = payload.Status
	}
	if payload.Source != "" {
		rawPayloadMap["source"] = payload.Source
	}
	rawPayload, _ := structpb.NewStruct(rawPayloadMap)

	alert := &alertingv1.Alert{
		Fingerprint:  fingerprint,
		Summary:      payload.Summary,
		Details:      payload.Details,
		Severity:     severity,
		Source:       alertingv1.AlertSource_ALERT_SOURCE_GENERIC,
		ServiceId:    serviceID,
		Labels:       labels,
		Annotations:  annotations,
		Status:       status,
		TriggeredAt:  timestamppb.New(triggeredAt),
		RawPayload:   rawPayload,
	}

	// Set resolved_at if the alert is resolved
	if status == alertingv1.AlertStatus_ALERT_STATUS_RESOLVED {
		alert.ResolvedAt = timestamppb.Now()
	}

	return h.alertStore.CreateOrUpdate(c.Request.Context(), alert)
}

func parseGenericStatus(status string) alertingv1.AlertStatus {
	switch status {
	case "triggered", "firing", "alerting":
		return alertingv1.AlertStatus_ALERT_STATUS_TRIGGERED
	case "resolved", "ok":
		return alertingv1.AlertStatus_ALERT_STATUS_RESOLVED
	case "acknowledged", "acked":
		return alertingv1.AlertStatus_ALERT_STATUS_ACKNOWLEDGED
	case "suppressed", "silenced":
		return alertingv1.AlertStatus_ALERT_STATUS_SUPPRESSED
	default:
		// Default to triggered for new alerts
		return alertingv1.AlertStatus_ALERT_STATUS_TRIGGERED
	}
}

func parseGenericSeverity(severity string) alertingv1.Severity {
	switch severity {
	case "critical", "crit", "p1":
		return alertingv1.Severity_SEVERITY_CRITICAL
	case "high", "warning", "warn", "p2":
		return alertingv1.Severity_SEVERITY_HIGH
	case "medium", "p3":
		return alertingv1.Severity_SEVERITY_MEDIUM
	case "low", "p4":
		return alertingv1.Severity_SEVERITY_LOW
	case "info", "informational", "p5":
		return alertingv1.Severity_SEVERITY_INFO
	default:
		return alertingv1.Severity_SEVERITY_MEDIUM
	}
}

func generateGenericFingerprint(serviceID string, payload *GenericPayload) string {
	// Create a deterministic string from service, summary, and sorted labels
	data := fmt.Sprintf("generic:%s:%s:", serviceID, payload.Summary)

	if payload.Labels != nil && len(payload.Labels) > 0 {
		keys := make([]string, 0, len(payload.Labels))
		for k := range payload.Labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			data += fmt.Sprintf("%s=%s,", k, payload.Labels[k])
		}
	}

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes (32 hex chars)
}
