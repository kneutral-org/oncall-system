// Package carrier provides carrier management and BGP alert handling for the on-call system.
package carrier

import (
	"context"
	"strconv"
	"strings"

	"github.com/rs/zerolog"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// BGPAlertInfo contains extracted information from a BGP alert.
type BGPAlertInfo struct {
	// Carrier resolved from the alert, if any.
	Carrier *Carrier

	// ASN extracted from the alert labels.
	ASN int

	// Remote IP address of the BGP peer.
	RemoteIP string

	// BGP session state (e.g., "Established", "Idle", "Active").
	SessionState string

	// Number of affected prefixes, if available.
	AffectedPrefixes int

	// Impact level based on carrier type and priority.
	ImpactLevel ImpactLevel

	// Whether this is a transit carrier (higher impact).
	IsTransit bool

	// Whether carrier-specific escalation should be triggered.
	TriggerEscalation bool

	// Suggested escalation policy ID.
	EscalationPolicyID string
}

// ImpactLevel represents the impact level of a BGP alert.
type ImpactLevel string

const (
	ImpactLevelCritical ImpactLevel = "critical"
	ImpactLevelHigh     ImpactLevel = "high"
	ImpactLevelMedium   ImpactLevel = "medium"
	ImpactLevelLow      ImpactLevel = "low"
)

// BGPHandler provides special handling for BGP-related alerts.
type BGPHandler struct {
	resolver Resolver
	store    Store
	logger   zerolog.Logger
}

// NewBGPHandler creates a new BGPHandler.
func NewBGPHandler(store Store, logger zerolog.Logger) *BGPHandler {
	return &BGPHandler{
		resolver: NewResolver(store),
		store:    store,
		logger:   logger.With().Str("handler", "bgp").Logger(),
	}
}

// BGP-related alert name patterns.
var bgpAlertPatterns = []string{
	"bgp",
	"peer",
	"session",
	"neighbor",
	"routing",
	"prefix",
}

// IsBGPAlert determines if an alert is BGP-related.
func (h *BGPHandler) IsBGPAlert(alert *routingv1.Alert) bool {
	if alert == nil {
		return false
	}

	// Check alert name
	alertName := strings.ToLower(alert.Summary)
	for _, pattern := range bgpAlertPatterns {
		if strings.Contains(alertName, pattern) {
			return true
		}
	}

	// Check labels for BGP indicators
	if alert.Labels != nil {
		// Check for ASN labels
		for _, key := range asnLabelKeys {
			if _, ok := alert.Labels[key]; ok {
				return true
			}
		}

		// Check for alertname containing BGP
		if alertName, ok := alert.Labels["alertname"]; ok {
			if strings.Contains(strings.ToLower(alertName), "bgp") {
				return true
			}
		}

		// Check for bgp_* labels
		for key := range alert.Labels {
			if strings.HasPrefix(strings.ToLower(key), "bgp_") {
				return true
			}
		}
	}

	return false
}

// HandleBGPAlert processes a BGP alert and returns extracted information.
func (h *BGPHandler) HandleBGPAlert(ctx context.Context, alert *routingv1.Alert) (*BGPAlertInfo, error) {
	if alert == nil {
		return nil, ErrInvalidCarrier
	}

	info := &BGPAlertInfo{}
	labels := alert.Labels
	if labels == nil {
		labels = make(map[string]string)
	}

	// Extract ASN
	info.ASN = h.extractASN(labels)

	// Extract other BGP-specific information
	info.RemoteIP = h.extractRemoteIP(labels)
	info.SessionState = h.extractSessionState(labels)
	info.AffectedPrefixes = h.extractAffectedPrefixes(labels)

	// Try to resolve carrier
	carrier, err := h.resolver.ResolveFromAlert(ctx, alert)
	if err == nil {
		info.Carrier = carrier
		info.IsTransit = carrier.Type == CarrierTypeTransit
		info.EscalationPolicyID = carrier.EscalationPolicy

		// Determine impact level based on carrier
		info.ImpactLevel = h.determineImpactLevel(carrier, info.AffectedPrefixes)

		// Determine if escalation should be triggered
		info.TriggerEscalation = h.shouldTriggerEscalation(carrier, info)

		h.logger.Info().
			Str("alertId", alert.Id).
			Str("carrierId", carrier.ID).
			Str("carrierName", carrier.Name).
			Int("asn", info.ASN).
			Str("impactLevel", string(info.ImpactLevel)).
			Bool("triggerEscalation", info.TriggerEscalation).
			Msg("BGP alert processed with carrier context")
	} else {
		// No carrier found - still process as BGP alert
		info.ImpactLevel = h.determineImpactLevelWithoutCarrier(info.AffectedPrefixes)

		h.logger.Info().
			Str("alertId", alert.Id).
			Int("asn", info.ASN).
			Str("impactLevel", string(info.ImpactLevel)).
			Msg("BGP alert processed without carrier context")
	}

	return info, nil
}

// extractASN extracts the ASN from alert labels.
func (h *BGPHandler) extractASN(labels map[string]string) int {
	for _, key := range asnLabelKeys {
		if asnStr, ok := labels[key]; ok {
			asn := parseASN(asnStr)
			if asn > 0 {
				return asn
			}
		}
	}
	return 0
}

// extractRemoteIP extracts the remote IP from alert labels.
func (h *BGPHandler) extractRemoteIP(labels map[string]string) string {
	ipKeys := []string{"remote_ip", "peer_ip", "neighbor_ip", "bgp_peer_ip", "peer_address"}
	for _, key := range ipKeys {
		if ip, ok := labels[key]; ok {
			return ip
		}
	}
	return ""
}

// extractSessionState extracts the BGP session state from alert labels.
func (h *BGPHandler) extractSessionState(labels map[string]string) string {
	stateKeys := []string{"state", "session_state", "bgp_state", "peer_state"}
	for _, key := range stateKeys {
		if state, ok := labels[key]; ok {
			return state
		}
	}
	return ""
}

// extractAffectedPrefixes extracts the number of affected prefixes from alert labels.
func (h *BGPHandler) extractAffectedPrefixes(labels map[string]string) int {
	prefixKeys := []string{"prefixes", "affected_prefixes", "prefix_count", "route_count"}
	for _, key := range prefixKeys {
		if prefixStr, ok := labels[key]; ok {
			count, err := strconv.Atoi(prefixStr)
			if err == nil && count > 0 {
				return count
			}
		}
	}
	return 0
}

// determineImpactLevel determines the impact level based on carrier and affected prefixes.
func (h *BGPHandler) determineImpactLevel(carrier *Carrier, affectedPrefixes int) ImpactLevel {
	// Transit carriers are always high impact
	if carrier.Type == CarrierTypeTransit {
		if carrier.Priority <= 1 || affectedPrefixes > 1000 {
			return ImpactLevelCritical
		}
		return ImpactLevelHigh
	}

	// Provider (upstream) carriers
	if carrier.Type == CarrierTypeProvider {
		if carrier.Priority <= 2 || affectedPrefixes > 500 {
			return ImpactLevelHigh
		}
		return ImpactLevelMedium
	}

	// Peering carriers
	if carrier.Type == CarrierTypePeering {
		if affectedPrefixes > 5000 {
			return ImpactLevelHigh
		}
		if affectedPrefixes > 1000 {
			return ImpactLevelMedium
		}
		return ImpactLevelLow
	}

	// Customer BGP sessions
	if carrier.Type == CarrierTypeCustomer {
		if carrier.Priority <= 1 {
			return ImpactLevelHigh
		}
		return ImpactLevelMedium
	}

	// Default based on priority
	if carrier.Priority <= 1 {
		return ImpactLevelHigh
	}
	if carrier.Priority <= 3 {
		return ImpactLevelMedium
	}
	return ImpactLevelLow
}

// determineImpactLevelWithoutCarrier determines impact level when no carrier is found.
func (h *BGPHandler) determineImpactLevelWithoutCarrier(affectedPrefixes int) ImpactLevel {
	if affectedPrefixes > 5000 {
		return ImpactLevelCritical
	}
	if affectedPrefixes > 1000 {
		return ImpactLevelHigh
	}
	if affectedPrefixes > 100 {
		return ImpactLevelMedium
	}
	return ImpactLevelLow
}

// shouldTriggerEscalation determines if carrier-specific escalation should be triggered.
func (h *BGPHandler) shouldTriggerEscalation(carrier *Carrier, info *BGPAlertInfo) bool {
	// Always escalate for transit carriers
	if carrier.Type == CarrierTypeTransit {
		return true
	}

	// Escalate for high priority carriers
	if carrier.Priority <= 1 {
		return true
	}

	// Escalate for critical/high impact
	if info.ImpactLevel == ImpactLevelCritical || info.ImpactLevel == ImpactLevelHigh {
		return true
	}

	// Escalate if escalation policy is configured
	if carrier.EscalationPolicy != "" {
		return true
	}

	return false
}

// GetCarrierContacts returns the contacts for a carrier, with primary contacts first.
func (h *BGPHandler) GetCarrierContacts(carrier *Carrier) []Contact {
	if carrier == nil || len(carrier.Contacts) == 0 {
		return nil
	}

	// Sort contacts with primary first
	result := make([]Contact, 0, len(carrier.Contacts))
	for _, c := range carrier.Contacts {
		if c.Primary {
			result = append([]Contact{c}, result...)
		} else {
			result = append(result, c)
		}
	}

	return result
}
