// Package equipment provides equipment type management and resolution for alerts.
package equipment

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

var (
	// ErrNoEquipmentResolved is returned when no equipment type can be resolved from alert labels.
	ErrNoEquipmentResolved = errors.New("no equipment type could be resolved from alert labels")
)

// Resolver defines the interface for equipment type resolution.
type Resolver interface {
	// Resolve resolves an equipment type from alert labels.
	Resolve(ctx context.Context, labels map[string]string) (*ResolvedEquipment, error)

	// InvalidateCache invalidates the cache for a specific equipment type name.
	InvalidateCache(name string)

	// Stop stops the resolver and cleans up resources.
	Stop()
}

// ResolverConfig holds configuration for the equipment resolver.
type ResolverConfig struct {
	// CacheTTL is the time-to-live for cached equipment types.
	CacheTTL time.Duration
	// Logger for the resolver
	Logger zerolog.Logger
}

// DefaultResolverConfig returns the default resolver configuration.
func DefaultResolverConfig() ResolverConfig {
	return ResolverConfig{
		CacheTTL: 5 * time.Minute,
		Logger:   zerolog.Nop(),
	}
}

// DefaultResolver is the standard implementation of Resolver.
type DefaultResolver struct {
	store  Store
	config ResolverConfig
	logger zerolog.Logger
	cache  map[string]*cacheEntry
	mu     sync.RWMutex
	done   chan struct{}
}

type cacheEntry struct {
	eq        *EquipmentType
	expiresAt time.Time
}

// Hostname prefix patterns for equipment type detection.
// Common patterns: rtr-xxx (router), sw-xxx (switch), fw-xxx (firewall), srv-xxx (server)
var hostnamePatterns = []struct {
	pattern *regexp.Regexp
	eqType  string
}{
	{regexp.MustCompile(`^rtr[-_]`), "router"},
	{regexp.MustCompile(`^rt[-_]`), "router"},
	{regexp.MustCompile(`^router[-_]`), "router"},
	{regexp.MustCompile(`^sw[-_]`), "switch"},
	{regexp.MustCompile(`^switch[-_]`), "switch"},
	{regexp.MustCompile(`^fw[-_]`), "firewall"},
	{regexp.MustCompile(`^firewall[-_]`), "firewall"},
	{regexp.MustCompile(`^srv[-_]`), "server"},
	{regexp.MustCompile(`^server[-_]`), "server"},
	{regexp.MustCompile(`^lb[-_]`), "load_balancer"},
	{regexp.MustCompile(`^loadbalancer[-_]`), "load_balancer"},
	{regexp.MustCompile(`^ap[-_]`), "access_point"},
	{regexp.MustCompile(`^wap[-_]`), "access_point"},
	{regexp.MustCompile(`^nas[-_]`), "storage"},
	{regexp.MustCompile(`^san[-_]`), "storage"},
	{regexp.MustCompile(`^stor[-_]`), "storage"},
	{regexp.MustCompile(`^pdu[-_]`), "pdu"},
	{regexp.MustCompile(`^ups[-_]`), "ups"},
}

// Job name patterns for equipment type extraction
var jobPatterns = []struct {
	pattern *regexp.Regexp
	eqType  string
}{
	{regexp.MustCompile(`(?i)router`), "router"},
	{regexp.MustCompile(`(?i)switch`), "switch"},
	{regexp.MustCompile(`(?i)firewall`), "firewall"},
	{regexp.MustCompile(`(?i)server`), "server"},
	{regexp.MustCompile(`(?i)load.?balancer`), "load_balancer"},
	{regexp.MustCompile(`(?i)storage`), "storage"},
	{regexp.MustCompile(`(?i)pdu`), "pdu"},
	{regexp.MustCompile(`(?i)ups`), "ups"},
}

// NewResolver creates a new equipment type resolver.
func NewResolver(store Store, config ResolverConfig) *DefaultResolver {
	r := &DefaultResolver{
		store:  store,
		config: config,
		logger: config.Logger,
		cache:  make(map[string]*cacheEntry),
		done:   make(chan struct{}),
	}

	// Start cache cleanup goroutine
	go r.cleanupCache()

	return r
}

// Resolve resolves an equipment type from alert labels.
// Priority order:
// 1. equipment_type label - Direct equipment type name
// 2. device_type label - Device type label
// 3. job label - Extract from job name patterns
// 4. instance label - Extract from hostname patterns (e.g., rtr-xxx, sw-xxx, fw-xxx)
func (r *DefaultResolver) Resolve(ctx context.Context, labels map[string]string) (*ResolvedEquipment, error) {
	if labels == nil {
		return nil, ErrNoEquipmentResolved
	}

	// Try resolution in priority order
	resolutionAttempts := []struct {
		method    ResolutionMethod
		extractFn func() (string, bool)
	}{
		{
			method: ResolutionMethodDirectLabel,
			extractFn: func() (string, bool) {
				if eqType, ok := labels["equipment_type"]; ok && eqType != "" {
					return normalizeEquipmentName(eqType), true
				}
				return "", false
			},
		},
		{
			method: ResolutionMethodDeviceType,
			extractFn: func() (string, bool) {
				if deviceType, ok := labels["device_type"]; ok && deviceType != "" {
					return normalizeEquipmentName(deviceType), true
				}
				return "", false
			},
		},
		{
			method: ResolutionMethodJobPattern,
			extractFn: func() (string, bool) {
				if job, ok := labels["job"]; ok && job != "" {
					return extractFromJobName(job)
				}
				return "", false
			},
		},
		{
			method: ResolutionMethodHostnamePrefix,
			extractFn: func() (string, bool) {
				if instance, ok := labels["instance"]; ok && instance != "" {
					return extractFromHostname(instance)
				}
				return "", false
			},
		},
	}

	for _, attempt := range resolutionAttempts {
		eqTypeName, found := attempt.extractFn()
		if !found {
			continue
		}

		// Try to get from cache first
		r.mu.RLock()
		entry, cached := r.cache[strings.ToLower(eqTypeName)]
		r.mu.RUnlock()

		if cached && entry.expiresAt.After(time.Now()) {
			r.logger.Debug().
				Str("name", eqTypeName).
				Str("method", string(attempt.method)).
				Msg("equipment type resolved from cache")
			return &ResolvedEquipment{
				EquipmentType:    entry.eq,
				ResolutionMethod: attempt.method,
				MatchedValue:     eqTypeName,
			}, nil
		}

		// Query the store
		eq, err := r.store.GetByName(ctx, eqTypeName)
		if err != nil {
			if errors.Is(err, ErrEquipmentTypeNotFound) {
				r.logger.Debug().
					Str("name", eqTypeName).
					Str("method", string(attempt.method)).
					Msg("equipment type not found in store, trying next method")
				continue
			}
			return nil, err
		}

		// Cache the result
		r.mu.Lock()
		r.cache[strings.ToLower(eqTypeName)] = &cacheEntry{
			eq:        eq,
			expiresAt: time.Now().Add(r.config.CacheTTL),
		}
		r.mu.Unlock()

		r.logger.Debug().
			Str("name", eqTypeName).
			Str("method", string(attempt.method)).
			Str("equipment_id", eq.ID).
			Msg("equipment type resolved from store")

		return &ResolvedEquipment{
			EquipmentType:    eq,
			ResolutionMethod: attempt.method,
			MatchedValue:     eqTypeName,
		}, nil
	}

	return nil, ErrNoEquipmentResolved
}

// InvalidateCache invalidates the cache for a specific equipment type name.
func (r *DefaultResolver) InvalidateCache(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cache, strings.ToLower(name))
}

// Stop stops the resolver and cleans up resources.
func (r *DefaultResolver) Stop() {
	close(r.done)
}

// cleanupCache periodically removes expired cache entries.
func (r *DefaultResolver) cleanupCache() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-r.done:
			return
		case <-ticker.C:
			r.mu.Lock()
			now := time.Now()
			for name, entry := range r.cache {
				if entry.expiresAt.Before(now) {
					delete(r.cache, name)
				}
			}
			r.mu.Unlock()
		}
	}
}

// normalizeEquipmentName normalizes an equipment type name.
func normalizeEquipmentName(name string) string {
	// Convert to lowercase and replace spaces with underscores
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")
	return normalized
}

// extractFromHostname extracts equipment type from hostname patterns.
func extractFromHostname(hostname string) (string, bool) {
	hostname = strings.ToLower(hostname)

	// Remove port if present (e.g., "rtr-nyc-01:9100" -> "rtr-nyc-01")
	if idx := strings.Index(hostname, ":"); idx != -1 {
		hostname = hostname[:idx]
	}

	for _, p := range hostnamePatterns {
		if p.pattern.MatchString(hostname) {
			return p.eqType, true
		}
	}

	return "", false
}

// extractFromJobName extracts equipment type from job name patterns.
func extractFromJobName(job string) (string, bool) {
	job = strings.ToLower(job)

	for _, p := range jobPatterns {
		if p.pattern.MatchString(job) {
			return p.eqType, true
		}
	}

	return "", false
}

// Ensure DefaultResolver implements Resolver
var _ Resolver = (*DefaultResolver)(nil)
