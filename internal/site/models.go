// Package site provides site resolution and enrichment for alerts.
package site

import (
	"time"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// SiteType represents the type of site.
type SiteType string

const (
	SiteTypeDatacenter      SiteType = "datacenter"
	SiteTypePOP             SiteType = "pop"
	SiteTypeHub             SiteType = "hub"
	SiteTypeCustomerPremise SiteType = "customer_premise"
)

// Site represents a physical or logical location.
type Site struct {
	ID                        string            `json:"id"`
	Name                      string            `json:"name"`
	Code                      string            `json:"code"`
	SiteType                  SiteType          `json:"siteType"`
	Tier                      *int              `json:"tier,omitempty"`
	Region                    string            `json:"region,omitempty"`
	Country                   string            `json:"country,omitempty"`
	City                      string            `json:"city,omitempty"`
	Address                   string            `json:"address,omitempty"`
	Timezone                  string            `json:"timezone"`
	PrimaryTeamID             *string           `json:"primaryTeamId,omitempty"`
	SecondaryTeamID           *string           `json:"secondaryTeamId,omitempty"`
	DefaultEscalationPolicyID *string           `json:"defaultEscalationPolicyId,omitempty"`
	ParentSiteID              *string           `json:"parentSiteId,omitempty"`
	Labels                    map[string]string `json:"labels"`
	BusinessHours             *BusinessHours    `json:"businessHours,omitempty"`
	CreatedAt                 time.Time         `json:"createdAt"`
	UpdatedAt                 time.Time         `json:"updatedAt"`
}

// BusinessHours represents the business hours configuration for a site.
type BusinessHours struct {
	Start string `json:"start"` // Format: "HH:MM" (24-hour)
	End   string `json:"end"`   // Format: "HH:MM" (24-hour)
	Days  []int  `json:"days"`  // 0=Sunday, 1=Monday, ..., 6=Saturday
}

// Team represents an on-call team.
type Team struct {
	ID                          string    `json:"id"`
	Name                        string    `json:"name"`
	Description                 string    `json:"description,omitempty"`
	DefaultEscalationPolicyID   *string   `json:"defaultEscalationPolicyId,omitempty"`
	DefaultNotificationChannelID *string   `json:"defaultNotificationChannelId,omitempty"`
	CreatedAt                   time.Time `json:"createdAt"`
	UpdatedAt                   time.Time `json:"updatedAt"`
}

// EscalationPolicy represents an escalation policy for alerts.
type EscalationPolicy struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// EnrichedAlert contains the original alert with resolved site metadata.
type EnrichedAlert struct {
	Original         *routingv1.Alert  `json:"original"`
	Site             *Site             `json:"site,omitempty"`
	PrimaryTeam      *Team             `json:"primaryTeam,omitempty"`
	SecondaryTeam    *Team             `json:"secondaryTeam,omitempty"`
	EscalationPolicy *EscalationPolicy `json:"escalationPolicy,omitempty"`
	IsBusinessHours  bool              `json:"isBusinessHours"`
	CustomerTier     string            `json:"customerTier,omitempty"`
	ResolvedSiteCode string            `json:"resolvedSiteCode,omitempty"`
	ResolutionMethod string            `json:"resolutionMethod,omitempty"`
}

// ListSitesFilter defines filters for listing sites.
type ListSitesFilter struct {
	SiteType   SiteType
	Region     string
	ParentSiteID string
	LabelKey   string
	LabelValue string
	PageSize   int
	PageToken  string
}
