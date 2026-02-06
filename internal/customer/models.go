// Package customer provides customer and tier management for alert routing.
package customer

import (
	"net"
	"time"
)

// CustomerTier represents a customer tier with SLA configuration.
type CustomerTier struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	Level                int               `json:"level"` // 1 = highest priority
	Description          string            `json:"description,omitempty"`
	CriticalResponseTime time.Duration     `json:"criticalResponseTime"`
	HighResponseTime     time.Duration     `json:"highResponseTime"`
	MediumResponseTime   time.Duration     `json:"mediumResponseTime"`
	LowResponseTime      time.Duration     `json:"lowResponseTime"`
	EscalationMultiplier float64           `json:"escalationMultiplier"` // 1.0 = normal, 0.5 = 2x faster
	SeverityBoost        int               `json:"severityBoost"`        // Boost severity by this amount
	DedicatedTeamID      *string           `json:"dedicatedTeamId,omitempty"`
	Metadata             map[string]string `json:"metadata,omitempty"`
	CreatedAt            time.Time         `json:"createdAt"`
	UpdatedAt            time.Time         `json:"updatedAt"`
}

// TierConfig provides the routing configuration for a customer tier.
type TierConfig struct {
	Tier                 *CustomerTier `json:"tier"`
	SeverityBoost        int           `json:"severityBoost"`
	EscalationMultiplier float64       `json:"escalationMultiplier"`
	DedicatedTeamID      *string       `json:"dedicatedTeamId,omitempty"`
}

// Customer represents a customer with tier assignment.
type Customer struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	AccountID   string            `json:"accountId"`
	TierID      string            `json:"tierId"`
	Description string            `json:"description,omitempty"`
	Domains     []string          `json:"domains,omitempty"`
	IPRanges    []string          `json:"ipRanges,omitempty"` // CIDR notation
	Contacts    []CustomerContact `json:"contacts,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// CustomerContact represents a contact person for a customer.
type CustomerContact struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone,omitempty"`
	Role    string `json:"role,omitempty"`
	Primary bool   `json:"primary"`
}

// IPRange represents a parsed IP range for matching.
type IPRange struct {
	CIDR    string
	Network *net.IPNet
}

// ParseIPRanges parses CIDR strings into IPRange structs.
func ParseIPRanges(cidrs []string) ([]IPRange, error) {
	ranges := make([]IPRange, 0, len(cidrs))
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, IPRange{
			CIDR:    cidr,
			Network: network,
		})
	}
	return ranges, nil
}

// ContainsIP checks if the given IP address is within any of the ranges.
func ContainsIP(ranges []IPRange, ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, r := range ranges {
		if r.Network.Contains(ip) {
			return true
		}
	}
	return false
}

// ListCustomerTiersFilter defines filters for listing customer tiers.
type ListCustomerTiersFilter struct {
	PageSize  int
	PageToken string
}

// ListCustomersFilter defines filters for listing customers.
type ListCustomersFilter struct {
	TierID    string
	AccountID string
	Domain    string
	PageSize  int
	PageToken string
}
