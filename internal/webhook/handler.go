// Package webhook provides HTTP handlers for ingesting alerts from various sources.
package webhook

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/kneutral-org/alerting-system/internal/store"
)

// Handler handles webhook requests for alert ingestion.
type Handler struct {
	alertStore   store.AlertStore
	serviceStore store.ServiceStore
	logger       zerolog.Logger
}

// NewHandler creates a new webhook handler with the provided dependencies.
func NewHandler(alertStore store.AlertStore, serviceStore store.ServiceStore, logger zerolog.Logger) *Handler {
	return &Handler{
		alertStore:   alertStore,
		serviceStore: serviceStore,
		logger:       logger.With().Str("component", "webhook").Logger(),
	}
}

// RegisterRoutes registers all webhook routes on the provided router group.
// The routes will be created under /webhook/* path.
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	webhooks := router.Group("/webhook")
	webhooks.POST("/alertmanager/:integration_key", h.AlertmanagerWebhook)
	webhooks.POST("/grafana/:integration_key", h.GrafanaWebhook)
	webhooks.POST("/generic/:integration_key", h.GenericWebhook)
}

// RegisterWebhookRoutes registers webhook handlers directly on the provided group.
// Use this when the router group already has the webhook prefix and middleware applied.
func (h *Handler) RegisterWebhookRoutes(webhooks *gin.RouterGroup) {
	webhooks.POST("/alertmanager/:integration_key", h.AlertmanagerWebhook)
	webhooks.POST("/grafana/:integration_key", h.GrafanaWebhook)
	webhooks.POST("/generic/:integration_key", h.GenericWebhook)
}

// validateIntegrationKey validates the integration key and returns the associated service.
// Returns the service if valid, or sends an error response and returns nil if invalid.
func (h *Handler) validateIntegrationKey(c *gin.Context) *store.Service {
	integrationKey := c.Param("integration_key")
	if integrationKey == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "integration key is required",
		})
		return nil
	}

	service, err := h.serviceStore.GetByIntegrationKey(c.Request.Context(), integrationKey)
	if err != nil {
		h.logger.Warn().
			Str("integrationKey", integrationKey).
			Err(err).
			Msg("invalid integration key")
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "invalid integration key",
		})
		return nil
	}

	return service
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// WebhookResponse represents a successful webhook response.
type WebhookResponse struct {
	Message   string   `json:"message"`
	AlertIds  []string `json:"alertIds"`
	Created   int      `json:"created"`
	Updated   int      `json:"updated"`
}
