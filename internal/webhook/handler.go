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
	config       WebhookConfig
}

// NewHandler creates a new webhook handler with the provided dependencies.
func NewHandler(alertStore store.AlertStore, serviceStore store.ServiceStore, logger zerolog.Logger) *Handler {
	return &Handler{
		alertStore:   alertStore,
		serviceStore: serviceStore,
		logger:       logger.With().Str("component", "webhook").Logger(),
		config:       WebhookConfig{},
	}
}

// NewHandlerWithConfig creates a new webhook handler with the provided dependencies and HMAC configuration.
func NewHandlerWithConfig(alertStore store.AlertStore, serviceStore store.ServiceStore, logger zerolog.Logger, config WebhookConfig) *Handler {
	return &Handler{
		alertStore:   alertStore,
		serviceStore: serviceStore,
		logger:       logger.With().Str("component", "webhook").Logger(),
		config:       config,
	}
}

// RegisterRoutes registers all webhook routes on the provided router group.
// HMAC signature verification is applied per source if secrets are configured.
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	webhooks := router.Group("/webhook")

	// Register Alertmanager webhook with optional HMAC middleware
	if h.config.AlertmanagerSecret != "" {
		alertmanager := webhooks.Group("/alertmanager")
		alertmanager.Use(HMACMiddleware(DefaultAlertmanagerConfig(h.config.AlertmanagerSecret)))
		alertmanager.POST("/:integration_key", h.AlertmanagerWebhook)
		h.logger.Info().Msg("Alertmanager webhook HMAC verification enabled")
	} else {
		webhooks.POST("/alertmanager/:integration_key", h.AlertmanagerWebhook)
	}

	// Register Grafana webhook with optional HMAC middleware
	if h.config.GrafanaSecret != "" {
		grafana := webhooks.Group("/grafana")
		grafana.Use(HMACMiddleware(DefaultGrafanaConfig(h.config.GrafanaSecret)))
		grafana.POST("/:integration_key", h.GrafanaWebhook)
		h.logger.Info().Msg("Grafana webhook HMAC verification enabled")
	} else {
		webhooks.POST("/grafana/:integration_key", h.GrafanaWebhook)
	}

	// Register Generic webhook with optional HMAC middleware
	if h.config.GenericSecret != "" {
		generic := webhooks.Group("/generic")
		generic.Use(HMACMiddleware(DefaultGenericConfig(h.config.GenericSecret)))
		generic.POST("/:integration_key", h.GenericWebhook)
		h.logger.Info().Msg("Generic webhook HMAC verification enabled")
	} else {
		webhooks.POST("/generic/:integration_key", h.GenericWebhook)
	}
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
