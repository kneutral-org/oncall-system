// Package main provides the entry point for the alerting-system server.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"github.com/kneutral-org/alerting-system/internal/store"
	"github.com/kneutral-org/alerting-system/internal/webhook"
	alertingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/v1"
)

func main() {
	// Setup logger
	logger := zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "alerting-system").
		Logger()

	// Get config from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize stores (in-memory for now, replace with real implementations)
	alertStore := NewInMemoryAlertStore()
	serviceStore := NewInMemoryServiceStore()

	// Create a default service for testing
	_, _ = serviceStore.Create(context.Background(), &store.Service{
		ID:             "default-service",
		Name:           "Default Service",
		IntegrationKey: "default-key",
	})

	// Setup Gin router
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(ginLogger(logger))

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// API v1 routes
	apiV1 := router.Group("/api/v1")

	// Register webhook handlers
	webhookHandler := webhook.NewHandler(alertStore, serviceStore, logger)
	webhookHandler.RegisterRoutes(apiV1)

	// Create server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Info().Str("port", port).Msg("starting HTTP server")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal().Err(err).Msg("failed to start server")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal().Err(err).Msg("server forced to shutdown")
	}

	logger.Info().Msg("server exited properly")
}

// ginLogger returns a Gin middleware that logs requests using zerolog.
func ginLogger(logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		event := logger.Info()
		if statusCode >= 400 {
			event = logger.Warn()
		}
		if statusCode >= 500 {
			event = logger.Error()
		}

		if raw != "" {
			path = path + "?" + raw
		}

		event.
			Str("method", c.Request.Method).
			Str("path", path).
			Int("status", statusCode).
			Dur("latency", latency).
			Str("clientIP", c.ClientIP()).
			Msg("request")
	}
}

// InMemoryAlertStore is a simple in-memory implementation of store.AlertStore.
// Replace with a real database implementation in production.
type InMemoryAlertStore struct {
	alerts     map[string]*alertingv1.Alert
	alertsByFP map[string]*alertingv1.Alert
	counter    int64
}

// NewInMemoryAlertStore creates a new in-memory alert store.
func NewInMemoryAlertStore() *InMemoryAlertStore {
	return &InMemoryAlertStore{
		alerts:     make(map[string]*alertingv1.Alert),
		alertsByFP: make(map[string]*alertingv1.Alert),
	}
}

func (s *InMemoryAlertStore) Create(ctx context.Context, alert *alertingv1.Alert) (*alertingv1.Alert, error) {
	s.counter++
	alert.Id = fmt.Sprintf("alert-%d", s.counter)
	s.alerts[alert.Id] = alert
	s.alertsByFP[alert.Fingerprint] = alert
	return alert, nil
}

func (s *InMemoryAlertStore) GetByID(ctx context.Context, id string) (*alertingv1.Alert, error) {
	alert, ok := s.alerts[id]
	if !ok {
		return nil, nil
	}
	return alert, nil
}

func (s *InMemoryAlertStore) GetByFingerprint(ctx context.Context, fingerprint string) (*alertingv1.Alert, error) {
	alert, ok := s.alertsByFP[fingerprint]
	if !ok {
		return nil, nil
	}
	return alert, nil
}

func (s *InMemoryAlertStore) Update(ctx context.Context, alert *alertingv1.Alert) (*alertingv1.Alert, error) {
	s.alerts[alert.Id] = alert
	s.alertsByFP[alert.Fingerprint] = alert
	return alert, nil
}

func (s *InMemoryAlertStore) CreateOrUpdate(ctx context.Context, alert *alertingv1.Alert) (*alertingv1.Alert, bool, error) {
	existing, ok := s.alertsByFP[alert.Fingerprint]
	if ok {
		alert.Id = existing.Id
		s.alerts[alert.Id] = alert
		s.alertsByFP[alert.Fingerprint] = alert
		return alert, false, nil
	}
	created, err := s.Create(ctx, alert)
	return created, true, err
}

func (s *InMemoryAlertStore) List(ctx context.Context, req *alertingv1.ListAlertsRequest) (*alertingv1.ListAlertsResponse, error) {
	var alerts []*alertingv1.Alert
	for _, a := range s.alerts {
		alerts = append(alerts, a)
	}
	return &alertingv1.ListAlertsResponse{Alerts: alerts, TotalCount: int32(len(alerts))}, nil
}

// InMemoryServiceStore is a simple in-memory implementation of store.ServiceStore.
type InMemoryServiceStore struct {
	services map[string]*store.Service
	counter  int64
}

// NewInMemoryServiceStore creates a new in-memory service store.
func NewInMemoryServiceStore() *InMemoryServiceStore {
	return &InMemoryServiceStore{
		services: make(map[string]*store.Service),
	}
}

func (s *InMemoryServiceStore) GetByIntegrationKey(ctx context.Context, integrationKey string) (*store.Service, error) {
	for _, svc := range s.services {
		if svc.IntegrationKey == integrationKey {
			return svc, nil
		}
	}
	return nil, fmt.Errorf("service not found for integration key: %s", integrationKey)
}

func (s *InMemoryServiceStore) Create(ctx context.Context, service *store.Service) (*store.Service, error) {
	if service.ID == "" {
		s.counter++
		service.ID = fmt.Sprintf("svc-%d", s.counter)
	}
	s.services[service.ID] = service
	return service, nil
}

func (s *InMemoryServiceStore) GetByID(ctx context.Context, id string) (*store.Service, error) {
	svc, ok := s.services[id]
	if !ok {
		return nil, nil
	}
	return svc, nil
}
