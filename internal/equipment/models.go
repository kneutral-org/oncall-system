// Package equipment provides equipment type management and resolution for alerts.
package equipment

import (
	"time"
)

// Category represents the category of equipment.
type Category string

const (
	CategoryNetwork  Category = "network"
	CategoryCompute  Category = "compute"
	CategoryStorage  Category = "storage"
	CategorySecurity Category = "security"
)

// EquipmentType represents a type of network or datacenter equipment.
type EquipmentType struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`             // e.g., "router", "switch", "firewall", "server"
	Category         Category          `json:"category"`         // network, compute, storage, security
	Vendor           string            `json:"vendor,omitempty"` // cisco, juniper, arista, etc.
	Criticality      int               `json:"criticality"`      // 1-5 (5 = most critical)
	DefaultTeamID    string            `json:"defaultTeamId,omitempty"`
	EscalationPolicy string            `json:"escalationPolicy,omitempty"`
	RoutingRules     []string          `json:"routingRules,omitempty"` // IDs of routing rules specific to this type
	Metadata         map[string]string `json:"metadata,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
}

// ListEquipmentTypesFilter defines filters for listing equipment types.
type ListEquipmentTypesFilter struct {
	Category    Category
	Vendor      string
	Criticality int
	PageSize    int
	PageToken   string
}

// ResolutionMethod indicates how the equipment type was resolved.
type ResolutionMethod string

const (
	ResolutionMethodDirectLabel    ResolutionMethod = "direct_label"
	ResolutionMethodDeviceType     ResolutionMethod = "device_type"
	ResolutionMethodJobPattern     ResolutionMethod = "job_pattern"
	ResolutionMethodHostnamePrefix ResolutionMethod = "hostname_prefix"
	ResolutionMethodNotResolved    ResolutionMethod = "not_resolved"
)

// ResolvedEquipment contains the resolved equipment type with metadata.
type ResolvedEquipment struct {
	EquipmentType    *EquipmentType   `json:"equipmentType,omitempty"`
	ResolutionMethod ResolutionMethod `json:"resolutionMethod"`
	MatchedValue     string           `json:"matchedValue,omitempty"`
}
