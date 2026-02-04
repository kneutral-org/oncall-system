// Package site provides the site store implementation.
package site

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SiteType represents the type of a site.
type SiteType string

const (
	SiteTypeDatacenter SiteType = "datacenter"
	SiteTypePOP        SiteType = "pop"
	SiteTypeOffice     SiteType = "office"
	SiteTypeColocation SiteType = "colocation"
)

// BusinessHours represents the business hours for a site.
type BusinessHours struct {
	DaysOfWeek []int  `json:"daysOfWeek"`
	StartTime  string `json:"startTime"`
	EndTime    string `json:"endTime"`
	Timezone   string `json:"timezone"`
}

// Site represents a site domain model.
type Site struct {
	ID                 uuid.UUID              `json:"id"`
	Name               string                 `json:"name"`
	Code               string                 `json:"code"`
	SiteType           SiteType               `json:"siteType"`
	Region             string                 `json:"region,omitempty"`
	Country            string                 `json:"country,omitempty"`
	City               string                 `json:"city,omitempty"`
	Timezone           string                 `json:"timezone"`
	Tier               int                    `json:"tier"`
	PrimaryTeamID      *uuid.UUID             `json:"primaryTeamId,omitempty"`
	SecondaryTeamID    *uuid.UUID             `json:"secondaryTeamId,omitempty"`
	EscalationPolicyID *uuid.UUID             `json:"escalationPolicyId,omitempty"`
	BusinessHours      *BusinessHours         `json:"businessHours,omitempty"`
	Metadata           map[string]interface{} `json:"metadata"`
	CreatedAt          time.Time              `json:"createdAt"`
	UpdatedAt          time.Time              `json:"updatedAt"`
}

// ListSitesParams contains parameters for listing sites.
type ListSitesParams struct {
	SiteTypeFilter []SiteType
	RegionFilter   string
	CountryFilter  string
	TierFilter     []int
	Limit          int32
	Offset         int32
}

// SiteStore defines the interface for site persistence.
type SiteStore interface {
	// CreateSite creates a new site.
	CreateSite(ctx context.Context, site *Site) (*Site, error)

	// GetSite retrieves a site by ID.
	GetSite(ctx context.Context, id uuid.UUID) (*Site, error)

	// GetSiteByCode retrieves a site by its unique code.
	GetSiteByCode(ctx context.Context, code string) (*Site, error)

	// ListSites retrieves sites based on filter criteria.
	ListSites(ctx context.Context, params ListSitesParams) ([]*Site, error)

	// UpdateSite updates an existing site.
	UpdateSite(ctx context.Context, site *Site) (*Site, error)

	// DeleteSite deletes a site.
	DeleteSite(ctx context.Context, id uuid.UUID) error
}

// InMemorySiteStore is an in-memory implementation of SiteStore.
type InMemorySiteStore struct {
	mu        sync.RWMutex
	sites     map[uuid.UUID]*Site
	codeIndex map[string]uuid.UUID
}

// NewInMemorySiteStore creates a new in-memory site store.
func NewInMemorySiteStore() *InMemorySiteStore {
	return &InMemorySiteStore{
		sites:     make(map[uuid.UUID]*Site),
		codeIndex: make(map[string]uuid.UUID),
	}
}

// CreateSite creates a new site.
func (s *InMemorySiteStore) CreateSite(ctx context.Context, site *Site) (*Site, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	site.ID = uuid.New()
	site.CreatedAt = time.Now()
	site.UpdatedAt = time.Now()

	if site.Timezone == "" {
		site.Timezone = "UTC"
	}
	if site.SiteType == "" {
		site.SiteType = SiteTypeDatacenter
	}
	if site.Tier == 0 {
		site.Tier = 3
	}
	if site.Metadata == nil {
		site.Metadata = make(map[string]interface{})
	}

	// Deep copy
	stored := deepCopySite(site)
	s.sites[site.ID] = stored
	s.codeIndex[strings.ToLower(site.Code)] = site.ID

	return site, nil
}

// GetSite retrieves a site by ID.
func (s *InMemorySiteStore) GetSite(ctx context.Context, id uuid.UUID) (*Site, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	site, ok := s.sites[id]
	if !ok {
		return nil, nil
	}

	return deepCopySite(site), nil
}

// GetSiteByCode retrieves a site by its unique code.
func (s *InMemorySiteStore) GetSiteByCode(ctx context.Context, code string) (*Site, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.codeIndex[strings.ToLower(code)]
	if !ok {
		return nil, nil
	}

	site, ok := s.sites[id]
	if !ok {
		return nil, nil
	}

	return deepCopySite(site), nil
}

// ListSites retrieves sites based on filter criteria.
func (s *InMemorySiteStore) ListSites(ctx context.Context, params ListSitesParams) ([]*Site, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}

	result := make([]*Site, 0)
	offset := int(params.Offset)
	count := 0

	for _, site := range s.sites {
		// Apply type filter
		if len(params.SiteTypeFilter) > 0 {
			found := false
			for _, st := range params.SiteTypeFilter {
				if site.SiteType == st {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Apply region filter
		if params.RegionFilter != "" && !strings.EqualFold(site.Region, params.RegionFilter) {
			continue
		}

		// Apply country filter
		if params.CountryFilter != "" && !strings.EqualFold(site.Country, params.CountryFilter) {
			continue
		}

		// Apply tier filter
		if len(params.TierFilter) > 0 {
			found := false
			for _, t := range params.TierFilter {
				if site.Tier == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if count < offset {
			count++
			continue
		}

		if int32(len(result)) >= limit {
			break
		}

		result = append(result, deepCopySite(site))
		count++
	}

	return result, nil
}

// UpdateSite updates an existing site.
func (s *InMemorySiteStore) UpdateSite(ctx context.Context, site *Site) (*Site, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.sites[site.ID]
	if !ok {
		return nil, nil
	}

	// Remove old code index if code changed
	if existing.Code != site.Code {
		delete(s.codeIndex, strings.ToLower(existing.Code))
		s.codeIndex[strings.ToLower(site.Code)] = site.ID
	}

	site.CreatedAt = existing.CreatedAt
	site.UpdatedAt = time.Now()

	stored := deepCopySite(site)
	s.sites[site.ID] = stored

	return site, nil
}

// DeleteSite deletes a site.
func (s *InMemorySiteStore) DeleteSite(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if site, ok := s.sites[id]; ok {
		delete(s.codeIndex, strings.ToLower(site.Code))
	}
	delete(s.sites, id)
	return nil
}

// deepCopySite creates a deep copy of a Site.
func deepCopySite(site *Site) *Site {
	copied := *site

	if site.PrimaryTeamID != nil {
		id := *site.PrimaryTeamID
		copied.PrimaryTeamID = &id
	}
	if site.SecondaryTeamID != nil {
		id := *site.SecondaryTeamID
		copied.SecondaryTeamID = &id
	}
	if site.EscalationPolicyID != nil {
		id := *site.EscalationPolicyID
		copied.EscalationPolicyID = &id
	}
	if site.BusinessHours != nil {
		bh := *site.BusinessHours
		bh.DaysOfWeek = make([]int, len(site.BusinessHours.DaysOfWeek))
		copy(bh.DaysOfWeek, site.BusinessHours.DaysOfWeek)
		copied.BusinessHours = &bh
	}
	if site.Metadata != nil {
		copied.Metadata = make(map[string]interface{})
		data, _ := json.Marshal(site.Metadata)
		_ = json.Unmarshal(data, &copied.Metadata)
	}

	return &copied
}

// Verify InMemorySiteStore implements SiteStore interface
var _ SiteStore = (*InMemorySiteStore)(nil)
