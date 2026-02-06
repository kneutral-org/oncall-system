// Package carrier provides carrier management and BGP alert handling for the on-call system.
package carrier

import (
	"context"
	"strconv"
	"strings"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// Resolver provides carrier resolution from various identifiers.
type Resolver interface {
	// ResolveFromAlert resolves a carrier from alert labels.
	ResolveFromAlert(ctx context.Context, alert *routingv1.Alert) (*Carrier, error)

	// Resolve resolves a carrier by ASN or name.
	Resolve(ctx context.Context, asn int, name string) (*Carrier, error)
}

// DefaultResolver implements Resolver using a carrier store.
type DefaultResolver struct {
	store Store
}

// NewResolver creates a new DefaultResolver.
func NewResolver(store Store) *DefaultResolver {
	return &DefaultResolver{store: store}
}

// ASN label keys to check in order of preference.
var asnLabelKeys = []string{
	"asn",
	"peer_asn",
	"remote_asn",
	"neighbor_asn",
	"bgp_asn",
	"carrier_asn",
}

// Carrier name label keys to check in order of preference.
var carrierLabelKeys = []string{
	"carrier",
	"carrier_name",
	"peer",
	"peer_name",
	"provider",
	"provider_name",
	"isp",
	"isp_name",
}

// ResolveFromAlert resolves a carrier from alert labels.
// It first tries to find the carrier by ASN, then by name.
func (r *DefaultResolver) ResolveFromAlert(ctx context.Context, alert *routingv1.Alert) (*Carrier, error) {
	if alert == nil {
		return nil, ErrNotFound
	}

	labels := alert.Labels
	if labels == nil {
		return nil, ErrNotFound
	}

	// Try to resolve by ASN first
	for _, key := range asnLabelKeys {
		if asnStr, ok := labels[key]; ok {
			asn := parseASN(asnStr)
			if asn > 0 {
				carrier, err := r.store.GetByASN(ctx, asn)
				if err == nil {
					return carrier, nil
				}
				// Continue to try other keys if not found
			}
		}
	}

	// Try to resolve by carrier name
	for _, key := range carrierLabelKeys {
		if name, ok := labels[key]; ok {
			if name != "" {
				carrier, err := r.store.GetByName(ctx, name)
				if err == nil {
					return carrier, nil
				}
				// Continue to try other keys if not found
			}
		}
	}

	return nil, ErrNotFound
}

// Resolve resolves a carrier by ASN or name.
// If ASN is provided (> 0), it takes precedence over name.
func (r *DefaultResolver) Resolve(ctx context.Context, asn int, name string) (*Carrier, error) {
	if asn > 0 {
		carrier, err := r.store.GetByASN(ctx, asn)
		if err == nil {
			return carrier, nil
		}
		// If ASN lookup fails, fall through to name lookup
	}

	if name != "" {
		return r.store.GetByName(ctx, name)
	}

	return nil, ErrNotFound
}

// parseASN parses an ASN from a string.
// It handles various formats like "AS12345", "12345", etc.
func parseASN(s string) int {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)

	// Remove "ASN" or "AS" prefix if present (check longer prefix first)
	s = strings.TrimPrefix(s, "ASN")
	s = strings.TrimPrefix(s, "AS")
	s = strings.TrimSpace(s)

	asn, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}

	// Validate ASN range (1-4294967295 for 32-bit ASN)
	if asn < 1 || asn > 4294967295 {
		return 0
	}

	return asn
}

// Ensure DefaultResolver implements Resolver
var _ Resolver = (*DefaultResolver)(nil)
