// Package site provides site resolution and enrichment for alerts.
package site

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

var (
	// ErrNoSiteResolved is returned when no site can be resolved from alert labels.
	ErrNoSiteResolved = errors.New("no site could be resolved from alert labels")
)

// ResolutionMethod indicates how the site was resolved.
type ResolutionMethod string

const (
	ResolutionMethodDirectLabel   ResolutionMethod = "direct_label"
	ResolutionMethodDatacenter    ResolutionMethod = "datacenter_label"
	ResolutionMethodPOP           ResolutionMethod = "pop_label"
	ResolutionMethodLocation      ResolutionMethod = "location_label"
	ResolutionMethodHostname      ResolutionMethod = "hostname_extraction"
	ResolutionMethodNotResolved   ResolutionMethod = "not_resolved"
)

// Resolver defines the interface for site resolution and enrichment.
type Resolver interface {
	// Resolve resolves a site from alert labels.
	Resolve(ctx context.Context, labels map[string]string) (*Site, error)

	// Enrich enriches an alert with site metadata.
	Enrich(ctx context.Context, alert *routingv1.Alert) (*EnrichedAlert, error)

	// IsBusinessHours checks if the current time is within site business hours.
	IsBusinessHours(ctx context.Context, siteID string, at time.Time) (bool, error)
}

// ResolverConfig holds configuration for the site resolver.
type ResolverConfig struct {
	// CacheConfig for the internal site cache
	CacheConfig CacheConfig
	// Logger for the resolver
	Logger zerolog.Logger
	// DefaultTimezone used when site has no timezone configured
	DefaultTimezone string
}

// DefaultResolverConfig returns the default resolver configuration.
func DefaultResolverConfig() ResolverConfig {
	return ResolverConfig{
		CacheConfig:     DefaultCacheConfig(),
		Logger:          zerolog.Nop(),
		DefaultTimezone: "UTC",
	}
}

// DefaultResolver is the standard implementation of Resolver.
type DefaultResolver struct {
	store  Store
	cache  *Cache
	config ResolverConfig
	logger zerolog.Logger
}

// NewResolver creates a new site resolver.
func NewResolver(store Store, config ResolverConfig) *DefaultResolver {
	return &DefaultResolver{
		store:  store,
		cache:  NewCache(config.CacheConfig),
		config: config,
		logger: config.Logger,
	}
}

// Resolve resolves a site from alert labels.
// Priority order:
// 1. site label - Direct site code
// 2. datacenter or dc label
// 3. pop label
// 4. location label
// 5. instance label - Extract from hostname patterns
func (r *DefaultResolver) Resolve(ctx context.Context, labels map[string]string) (*Site, error) {
	if labels == nil {
		return nil, ErrNoSiteResolved
	}

	// Try resolution in priority order
	resolutionAttempts := []struct {
		method     ResolutionMethod
		extractFn  func() (string, bool)
	}{
		{
			method: ResolutionMethodDirectLabel,
			extractFn: func() (string, bool) {
				if code, ok := labels["site"]; ok && code != "" {
					return NormalizeSiteCode(code), true
				}
				return "", false
			},
		},
		{
			method: ResolutionMethodDatacenter,
			extractFn: func() (string, bool) {
				if code, ok := labels["datacenter"]; ok && code != "" {
					return NormalizeSiteCode(code), true
				}
				if code, ok := labels["dc"]; ok && code != "" {
					return NormalizeSiteCode(code), true
				}
				return "", false
			},
		},
		{
			method: ResolutionMethodPOP,
			extractFn: func() (string, bool) {
				if code, ok := labels["pop"]; ok && code != "" {
					return NormalizeSiteCode(code), true
				}
				return "", false
			},
		},
		{
			method: ResolutionMethodLocation,
			extractFn: func() (string, bool) {
				if code, ok := labels["location"]; ok && code != "" {
					return NormalizeSiteCode(code), true
				}
				return "", false
			},
		},
		{
			method: ResolutionMethodHostname,
			extractFn: func() (string, bool) {
				if instance, ok := labels["instance"]; ok && instance != "" {
					return ExtractSiteCodeFromInstance(instance)
				}
				return "", false
			},
		},
	}

	for _, attempt := range resolutionAttempts {
		code, found := attempt.extractFn()
		if !found {
			continue
		}

		// Try to get from cache first
		if site := r.cache.Get(code); site != nil {
			r.logger.Debug().
				Str("code", code).
				Str("method", string(attempt.method)).
				Msg("site resolved from cache")
			return site, nil
		}

		// Query the store
		site, err := r.store.GetByCode(ctx, code)
		if err != nil {
			if errors.Is(err, ErrSiteNotFound) {
				r.logger.Debug().
					Str("code", code).
					Str("method", string(attempt.method)).
					Msg("site code not found in store, trying next method")
				continue
			}
			return nil, err
		}

		// Cache the result
		r.cache.Set(site)

		r.logger.Debug().
			Str("code", code).
			Str("method", string(attempt.method)).
			Str("site_id", site.ID).
			Msg("site resolved from store")

		return site, nil
	}

	return nil, ErrNoSiteResolved
}

// Enrich enriches an alert with site metadata.
func (r *DefaultResolver) Enrich(ctx context.Context, alert *routingv1.Alert) (*EnrichedAlert, error) {
	if alert == nil {
		return nil, errors.New("alert is nil")
	}

	enriched := &EnrichedAlert{
		Original:         alert,
		IsBusinessHours:  true, // Default to business hours if unknown
		ResolutionMethod: string(ResolutionMethodNotResolved),
	}

	// Extract customer tier from labels
	if tier, ok := alert.Labels["customer_tier"]; ok {
		enriched.CustomerTier = tier
	} else if tier, ok := alert.Labels["tier"]; ok {
		enriched.CustomerTier = tier
	}

	// Resolve site
	site, err := r.Resolve(ctx, alert.Labels)
	if err != nil {
		if errors.Is(err, ErrNoSiteResolved) {
			r.logger.Debug().
				Str("alert_id", alert.Id).
				Msg("no site resolved for alert")
			return enriched, nil
		}
		return nil, err
	}

	enriched.Site = site
	enriched.ResolvedSiteCode = site.Code
	enriched.ResolutionMethod = r.getResolutionMethod(alert.Labels)

	// Load primary team if configured
	if site.PrimaryTeamID != nil && *site.PrimaryTeamID != "" {
		team, err := r.store.GetTeamByID(ctx, *site.PrimaryTeamID)
		if err != nil && !errors.Is(err, ErrTeamNotFound) {
			r.logger.Warn().
				Err(err).
				Str("team_id", *site.PrimaryTeamID).
				Msg("failed to load primary team")
		} else if err == nil {
			enriched.PrimaryTeam = team
		}
	}

	// Load secondary team if configured
	if site.SecondaryTeamID != nil && *site.SecondaryTeamID != "" {
		team, err := r.store.GetTeamByID(ctx, *site.SecondaryTeamID)
		if err != nil && !errors.Is(err, ErrTeamNotFound) {
			r.logger.Warn().
				Err(err).
				Str("team_id", *site.SecondaryTeamID).
				Msg("failed to load secondary team")
		} else if err == nil {
			enriched.SecondaryTeam = team
		}
	}

	// Load escalation policy if configured
	if site.DefaultEscalationPolicyID != nil && *site.DefaultEscalationPolicyID != "" {
		// Note: EscalationPolicy store would need to be added
		enriched.EscalationPolicy = &EscalationPolicy{
			ID: *site.DefaultEscalationPolicyID,
		}
	}

	// Check business hours
	now := time.Now()
	isBusinessHours, err := r.IsBusinessHours(ctx, site.ID, now)
	if err != nil {
		r.logger.Warn().
			Err(err).
			Str("site_id", site.ID).
			Msg("failed to check business hours")
		// Default to true on error
		enriched.IsBusinessHours = true
	} else {
		enriched.IsBusinessHours = isBusinessHours
	}

	r.logger.Debug().
		Str("alert_id", alert.Id).
		Str("site_code", site.Code).
		Bool("is_business_hours", enriched.IsBusinessHours).
		Str("customer_tier", enriched.CustomerTier).
		Msg("alert enriched successfully")

	return enriched, nil
}

// IsBusinessHours checks if the given time is within the site's business hours.
func (r *DefaultResolver) IsBusinessHours(ctx context.Context, siteID string, at time.Time) (bool, error) {
	// Try cache first
	site := r.cache.GetByID(siteID)
	if site == nil {
		// Load from store
		var err error
		site, err = r.store.GetByID(ctx, siteID)
		if err != nil {
			return false, err
		}
		r.cache.Set(site)
	}

	timezone := site.Timezone
	if timezone == "" {
		timezone = r.config.DefaultTimezone
	}

	return IsWithinBusinessHours(site.BusinessHours, at, timezone)
}

// getResolutionMethod determines which method was used to resolve the site.
func (r *DefaultResolver) getResolutionMethod(labels map[string]string) string {
	if _, ok := labels["site"]; ok {
		return string(ResolutionMethodDirectLabel)
	}
	if _, ok := labels["datacenter"]; ok {
		return string(ResolutionMethodDatacenter)
	}
	if _, ok := labels["dc"]; ok {
		return string(ResolutionMethodDatacenter)
	}
	if _, ok := labels["pop"]; ok {
		return string(ResolutionMethodPOP)
	}
	if _, ok := labels["location"]; ok {
		return string(ResolutionMethodLocation)
	}
	if _, ok := labels["instance"]; ok {
		return string(ResolutionMethodHostname)
	}
	return string(ResolutionMethodNotResolved)
}

// InvalidateCache invalidates a site from the cache.
func (r *DefaultResolver) InvalidateCache(code string) {
	r.cache.Invalidate(code)
}

// InvalidateCacheByID invalidates a site from the cache by ID.
func (r *DefaultResolver) InvalidateCacheByID(id string) {
	r.cache.InvalidateByID(id)
}

// Stop stops the resolver and cleans up resources.
func (r *DefaultResolver) Stop() {
	r.cache.Stop()
}

// Ensure DefaultResolver implements Resolver
var _ Resolver = (*DefaultResolver)(nil)
