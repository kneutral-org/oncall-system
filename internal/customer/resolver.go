// Package customer provides customer and tier management for alert routing.
package customer

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

var (
	// ErrNoCustomerResolved is returned when no customer can be resolved from alert labels.
	ErrNoCustomerResolved = errors.New("no customer could be resolved from alert labels")
)

// ResolutionMethod indicates how the customer was resolved.
type ResolutionMethod string

const (
	ResolutionMethodDirectLabel ResolutionMethod = "direct_label"
	ResolutionMethodAccountID   ResolutionMethod = "account_id"
	ResolutionMethodDomain      ResolutionMethod = "domain"
	ResolutionMethodIPRange     ResolutionMethod = "ip_range"
	ResolutionMethodNotResolved ResolutionMethod = "not_resolved"
)

// Resolver defines the interface for customer resolution.
type Resolver interface {
	// Resolve resolves a customer from alert labels.
	Resolve(ctx context.Context, labels map[string]string) (*Customer, error)

	// GetTierConfig gets the tier configuration for a customer.
	GetTierConfig(ctx context.Context, customerID string) (*TierConfig, error)

	// ResolveWithTier resolves customer and returns with tier config.
	ResolveWithTier(ctx context.Context, labels map[string]string) (*Customer, *TierConfig, error)
}

// ResolverConfig holds configuration for the customer resolver.
type ResolverConfig struct {
	// CacheTTL for caching resolved customers
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
	customerStore Store
	tierStore     TierStore
	config        ResolverConfig
	logger        zerolog.Logger

	// Simple cache for resolved customers
	cache    map[string]*cacheEntry
	cacheMu  sync.RWMutex
	stopCh   chan struct{}
	stopOnce sync.Once
}

type cacheEntry struct {
	customer  *Customer
	expiresAt time.Time
}

// NewResolver creates a new customer resolver.
func NewResolver(customerStore Store, tierStore TierStore, config ResolverConfig) *DefaultResolver {
	r := &DefaultResolver{
		customerStore: customerStore,
		tierStore:     tierStore,
		config:        config,
		logger:        config.Logger,
		cache:         make(map[string]*cacheEntry),
		stopCh:        make(chan struct{}),
	}

	// Start cache cleanup goroutine
	go r.cleanupCache()

	return r
}

// Resolve resolves a customer from alert labels.
// Priority order:
// 1. customer label - Direct customer ID
// 2. account_id label - Account identifier
// 3. domain label - Domain lookup
// 4. client_ip label - IP range match
func (r *DefaultResolver) Resolve(ctx context.Context, labels map[string]string) (*Customer, error) {
	if labels == nil {
		return nil, ErrNoCustomerResolved
	}

	// Resolution attempts in priority order
	resolutionAttempts := []struct {
		method    ResolutionMethod
		extractFn func() (*Customer, error)
	}{
		{
			method: ResolutionMethodDirectLabel,
			extractFn: func() (*Customer, error) {
				if customerID, ok := labels["customer"]; ok && customerID != "" {
					// Check cache first
					if customer := r.getCached(customerID); customer != nil {
						return customer, nil
					}
					customer, err := r.customerStore.GetByID(ctx, customerID)
					if err == nil {
						r.setCache(customerID, customer)
					}
					return customer, err
				}
				return nil, ErrNoCustomerResolved
			},
		},
		{
			method: ResolutionMethodAccountID,
			extractFn: func() (*Customer, error) {
				if accountID, ok := labels["account_id"]; ok && accountID != "" {
					cacheKey := "account:" + accountID
					if customer := r.getCached(cacheKey); customer != nil {
						return customer, nil
					}
					customer, err := r.customerStore.GetByAccountID(ctx, accountID)
					if err == nil {
						r.setCache(cacheKey, customer)
					}
					return customer, err
				}
				return nil, ErrNoCustomerResolved
			},
		},
		{
			method: ResolutionMethodDomain,
			extractFn: func() (*Customer, error) {
				if domain, ok := labels["domain"]; ok && domain != "" {
					cacheKey := "domain:" + domain
					if customer := r.getCached(cacheKey); customer != nil {
						return customer, nil
					}
					customer, err := r.customerStore.GetByDomain(ctx, domain)
					if err == nil {
						r.setCache(cacheKey, customer)
					}
					return customer, err
				}
				return nil, ErrNoCustomerResolved
			},
		},
		{
			method: ResolutionMethodIPRange,
			extractFn: func() (*Customer, error) {
				if clientIP, ok := labels["client_ip"]; ok && clientIP != "" {
					cacheKey := "ip:" + clientIP
					if customer := r.getCached(cacheKey); customer != nil {
						return customer, nil
					}
					customers, err := r.customerStore.GetByIPRange(ctx, clientIP)
					if err != nil {
						return nil, err
					}
					if len(customers) > 0 {
						// Return first match (could be enhanced to prioritize by tier)
						r.setCache(cacheKey, customers[0])
						return customers[0], nil
					}
				}
				return nil, ErrNoCustomerResolved
			},
		},
	}

	for _, attempt := range resolutionAttempts {
		customer, err := attempt.extractFn()
		if err == nil {
			r.logger.Debug().
				Str("customer_id", customer.ID).
				Str("method", string(attempt.method)).
				Msg("customer resolved")
			return customer, nil
		}
		if !errors.Is(err, ErrNoCustomerResolved) && !errors.Is(err, ErrCustomerNotFound) {
			// Unexpected error
			r.logger.Warn().
				Err(err).
				Str("method", string(attempt.method)).
				Msg("error during customer resolution")
		}
	}

	return nil, ErrNoCustomerResolved
}

// GetTierConfig gets the tier configuration for a customer.
func (r *DefaultResolver) GetTierConfig(ctx context.Context, customerID string) (*TierConfig, error) {
	customer, err := r.customerStore.GetByID(ctx, customerID)
	if err != nil {
		return nil, err
	}

	tier, err := r.tierStore.GetByID(ctx, customer.TierID)
	if err != nil {
		if errors.Is(err, ErrTierNotFound) {
			// Return default config if tier not found
			return &TierConfig{
				SeverityBoost:        0,
				EscalationMultiplier: 1.0,
			}, nil
		}
		return nil, err
	}

	return &TierConfig{
		Tier:                 tier,
		SeverityBoost:        tier.SeverityBoost,
		EscalationMultiplier: tier.EscalationMultiplier,
		DedicatedTeamID:      tier.DedicatedTeamID,
	}, nil
}

// ResolveWithTier resolves customer and returns with tier config.
func (r *DefaultResolver) ResolveWithTier(ctx context.Context, labels map[string]string) (*Customer, *TierConfig, error) {
	customer, err := r.Resolve(ctx, labels)
	if err != nil {
		return nil, nil, err
	}

	tierConfig, err := r.GetTierConfig(ctx, customer.ID)
	if err != nil {
		// Return customer even if tier lookup fails
		r.logger.Warn().
			Err(err).
			Str("customer_id", customer.ID).
			Msg("failed to get tier config, returning customer without tier")
		return customer, &TierConfig{
			SeverityBoost:        0,
			EscalationMultiplier: 1.0,
		}, nil
	}

	return customer, tierConfig, nil
}

// getCached retrieves a customer from cache.
func (r *DefaultResolver) getCached(key string) *Customer {
	r.cacheMu.RLock()
	defer r.cacheMu.RUnlock()

	entry, ok := r.cache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil
	}
	return entry.customer
}

// setCache stores a customer in cache.
func (r *DefaultResolver) setCache(key string, customer *Customer) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	r.cache[key] = &cacheEntry{
		customer:  customer,
		expiresAt: time.Now().Add(r.config.CacheTTL),
	}
}

// cleanupCache periodically removes expired entries.
func (r *DefaultResolver) cleanupCache() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.cacheMu.Lock()
			now := time.Now()
			for key, entry := range r.cache {
				if now.After(entry.expiresAt) {
					delete(r.cache, key)
				}
			}
			r.cacheMu.Unlock()
		}
	}
}

// InvalidateCache invalidates all cache entries for a customer.
func (r *DefaultResolver) InvalidateCache(customerID string) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	// Remove direct ID cache
	delete(r.cache, customerID)

	// Remove all entries pointing to this customer
	for key, entry := range r.cache {
		if entry.customer != nil && entry.customer.ID == customerID {
			delete(r.cache, key)
		}
	}
}

// Stop stops the resolver and cleans up resources.
func (r *DefaultResolver) Stop() {
	r.stopOnce.Do(func() {
		close(r.stopCh)
	})
}

// Ensure DefaultResolver implements Resolver
var _ Resolver = (*DefaultResolver)(nil)
