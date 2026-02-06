package site

import (
	"context"
	"errors"
	"testing"
	"time"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// mockStore is a mock implementation of Store for testing.
type mockStore struct {
	sites map[string]*Site
	teams map[string]*Team
}

func newMockStore() *mockStore {
	return &mockStore{
		sites: make(map[string]*Site),
		teams: make(map[string]*Team),
	}
}

func (m *mockStore) GetByCode(ctx context.Context, code string) (*Site, error) {
	site, ok := m.sites[code]
	if !ok {
		return nil, ErrSiteNotFound
	}
	// Return a copy to simulate DB behavior
	copy := *site
	return &copy, nil
}

func (m *mockStore) GetByID(ctx context.Context, id string) (*Site, error) {
	for _, site := range m.sites {
		if site.ID == id {
			// Return a copy to simulate DB behavior
			copy := *site
			return &copy, nil
		}
	}
	return nil, ErrSiteNotFound
}

func (m *mockStore) List(ctx context.Context, filter *ListSitesFilter) ([]*Site, string, error) {
	var sites []*Site
	for _, site := range m.sites {
		sites = append(sites, site)
	}
	return sites, "", nil
}

func (m *mockStore) Create(ctx context.Context, site *Site) (*Site, error) {
	m.sites[site.Code] = site
	return site, nil
}

func (m *mockStore) Update(ctx context.Context, site *Site) (*Site, error) {
	if _, ok := m.sites[site.Code]; !ok {
		return nil, ErrSiteNotFound
	}
	m.sites[site.Code] = site
	return site, nil
}

func (m *mockStore) Delete(ctx context.Context, id string) error {
	for code, site := range m.sites {
		if site.ID == id {
			delete(m.sites, code)
			return nil
		}
	}
	return ErrSiteNotFound
}

func (m *mockStore) GetTeamByID(ctx context.Context, id string) (*Team, error) {
	team, ok := m.teams[id]
	if !ok {
		return nil, ErrTeamNotFound
	}
	return team, nil
}

func TestResolver_Resolve(t *testing.T) {
	store := newMockStore()

	// Add test sites
	store.sites["dfw1"] = &Site{
		ID:       "site-1",
		Code:     "dfw1",
		Name:     "Dallas DC 1",
		Timezone: "America/Chicago",
	}
	store.sites["nyc2"] = &Site{
		ID:       "site-2",
		Code:     "nyc2",
		Name:     "New York DC 2",
		Timezone: "America/New_York",
	}

	resolver := NewResolver(store, DefaultResolverConfig())
	defer resolver.Stop()

	tests := []struct {
		name       string
		labels     map[string]string
		wantCode   string
		wantErr    bool
		wantErrIs  error
	}{
		{
			name: "resolve from site label",
			labels: map[string]string{
				"site": "dfw1",
			},
			wantCode: "dfw1",
			wantErr:  false,
		},
		{
			name: "resolve from datacenter label",
			labels: map[string]string{
				"datacenter": "nyc2",
			},
			wantCode: "nyc2",
			wantErr:  false,
		},
		{
			name: "resolve from dc label",
			labels: map[string]string{
				"dc": "dfw1",
			},
			wantCode: "dfw1",
			wantErr:  false,
		},
		{
			name: "site label takes priority over datacenter",
			labels: map[string]string{
				"site":       "dfw1",
				"datacenter": "nyc2",
			},
			wantCode: "dfw1",
			wantErr:  false,
		},
		{
			name: "resolve from instance hostname",
			labels: map[string]string{
				"instance": "dfw1-router01:9090",
			},
			wantCode: "dfw1",
			wantErr:  false,
		},
		{
			name: "normalize uppercase site code",
			labels: map[string]string{
				"site": "DFW1",
			},
			wantCode: "dfw1",
			wantErr:  false,
		},
		{
			name:      "no labels returns error",
			labels:    nil,
			wantErr:   true,
			wantErrIs: ErrNoSiteResolved,
		},
		{
			name:      "empty labels returns error",
			labels:    map[string]string{},
			wantErr:   true,
			wantErrIs: ErrNoSiteResolved,
		},
		{
			name: "unknown site code returns error",
			labels: map[string]string{
				"site": "unknown",
			},
			wantErr:   true,
			wantErrIs: ErrNoSiteResolved,
		},
		{
			name: "fallback from unknown site to known datacenter",
			labels: map[string]string{
				"site":       "unknown",
				"datacenter": "dfw1",
			},
			wantCode: "dfw1",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			site, err := resolver.Resolve(context.Background(), tt.labels)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Errorf("Resolve() error = %v, wantErrIs %v", err, tt.wantErrIs)
				return
			}
			if err == nil && site.Code != tt.wantCode {
				t.Errorf("Resolve() got code %v, want %v", site.Code, tt.wantCode)
			}
		})
	}
}

func TestResolver_Enrich(t *testing.T) {
	store := newMockStore()

	primaryTeamID := "team-1"
	secondaryTeamID := "team-2"

	store.sites["dfw1"] = &Site{
		ID:              "site-1",
		Code:            "dfw1",
		Name:            "Dallas DC 1",
		Timezone:        "America/Chicago",
		PrimaryTeamID:   &primaryTeamID,
		SecondaryTeamID: &secondaryTeamID,
		BusinessHours: &BusinessHours{
			Start: "09:00",
			End:   "17:00",
			Days:  []int{1, 2, 3, 4, 5},
		},
	}

	store.teams["team-1"] = &Team{
		ID:   "team-1",
		Name: "NOC Team 1",
	}
	store.teams["team-2"] = &Team{
		ID:   "team-2",
		Name: "NOC Team 2",
	}

	resolver := NewResolver(store, DefaultResolverConfig())
	defer resolver.Stop()

	tests := []struct {
		name                string
		alert               *routingv1.Alert
		wantSiteCode        string
		wantPrimaryTeamID   string
		wantSecondaryTeamID string
		wantCustomerTier    string
		wantErr             bool
	}{
		{
			name: "full enrichment with teams",
			alert: &routingv1.Alert{
				Id:      "alert-1",
				Summary: "Test alert",
				Labels: map[string]string{
					"site":          "dfw1",
					"customer_tier": "premium",
				},
			},
			wantSiteCode:        "dfw1",
			wantPrimaryTeamID:   "team-1",
			wantSecondaryTeamID: "team-2",
			wantCustomerTier:    "premium",
			wantErr:             false,
		},
		{
			name: "enrichment with tier label fallback",
			alert: &routingv1.Alert{
				Id:      "alert-2",
				Summary: "Test alert 2",
				Labels: map[string]string{
					"site": "dfw1",
					"tier": "enterprise",
				},
			},
			wantSiteCode:        "dfw1",
			wantPrimaryTeamID:   "team-1",
			wantSecondaryTeamID: "team-2",
			wantCustomerTier:    "enterprise",
			wantErr:             false,
		},
		{
			name: "no site resolved",
			alert: &routingv1.Alert{
				Id:      "alert-3",
				Summary: "Test alert 3",
				Labels: map[string]string{
					"other": "value",
				},
			},
			wantSiteCode:     "",
			wantCustomerTier: "",
			wantErr:          false, // Should not error, just no site
		},
		{
			name:    "nil alert returns error",
			alert:   nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enriched, err := resolver.Enrich(context.Background(), tt.alert)
			if (err != nil) != tt.wantErr {
				t.Errorf("Enrich() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if enriched.Original != tt.alert {
				t.Error("Enrich() did not preserve original alert")
			}

			if tt.wantSiteCode != "" {
				if enriched.Site == nil {
					t.Error("Enrich() expected site but got nil")
					return
				}
				if enriched.Site.Code != tt.wantSiteCode {
					t.Errorf("Enrich() site code = %v, want %v", enriched.Site.Code, tt.wantSiteCode)
				}
			}

			if tt.wantPrimaryTeamID != "" {
				if enriched.PrimaryTeam == nil {
					t.Error("Enrich() expected primary team but got nil")
					return
				}
				if enriched.PrimaryTeam.ID != tt.wantPrimaryTeamID {
					t.Errorf("Enrich() primary team ID = %v, want %v", enriched.PrimaryTeam.ID, tt.wantPrimaryTeamID)
				}
			}

			if tt.wantSecondaryTeamID != "" {
				if enriched.SecondaryTeam == nil {
					t.Error("Enrich() expected secondary team but got nil")
					return
				}
				if enriched.SecondaryTeam.ID != tt.wantSecondaryTeamID {
					t.Errorf("Enrich() secondary team ID = %v, want %v", enriched.SecondaryTeam.ID, tt.wantSecondaryTeamID)
				}
			}

			if enriched.CustomerTier != tt.wantCustomerTier {
				t.Errorf("Enrich() customer tier = %v, want %v", enriched.CustomerTier, tt.wantCustomerTier)
			}
		})
	}
}

func TestResolver_IsBusinessHours(t *testing.T) {
	store := newMockStore()

	store.sites["dfw1"] = &Site{
		ID:       "site-1",
		Code:     "dfw1",
		Name:     "Dallas DC 1",
		Timezone: "UTC",
		BusinessHours: &BusinessHours{
			Start: "09:00",
			End:   "17:00",
			Days:  []int{1, 2, 3, 4, 5},
		},
	}
	store.sites["nyc2"] = &Site{
		ID:       "site-2",
		Code:     "nyc2",
		Name:     "New York DC 2",
		Timezone: "UTC",
		// No business hours configured
	}

	resolver := NewResolver(store, DefaultResolverConfig())
	defer resolver.Stop()

	tests := []struct {
		name    string
		siteID  string
		at      time.Time
		want    bool
		wantErr bool
	}{
		{
			name:   "within business hours",
			siteID: "site-1",
			at:     time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), // Monday 10am
			want:   true,
		},
		{
			name:   "outside business hours - early",
			siteID: "site-1",
			at:     time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC), // Monday 8am
			want:   false,
		},
		{
			name:   "outside business hours - late",
			siteID: "site-1",
			at:     time.Date(2024, 1, 15, 18, 0, 0, 0, time.UTC), // Monday 6pm
			want:   false,
		},
		{
			name:   "weekend",
			siteID: "site-1",
			at:     time.Date(2024, 1, 13, 10, 0, 0, 0, time.UTC), // Saturday 10am
			want:   false,
		},
		{
			name:   "no business hours configured - always true",
			siteID: "site-2",
			at:     time.Date(2024, 1, 13, 3, 0, 0, 0, time.UTC), // Saturday 3am
			want:   true,
		},
		{
			name:    "unknown site",
			siteID:  "unknown",
			at:      time.Now(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolver.IsBusinessHours(context.Background(), tt.siteID, tt.at)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsBusinessHours() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && got != tt.want {
				t.Errorf("IsBusinessHours() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolver_CacheInvalidation(t *testing.T) {
	store := newMockStore()

	store.sites["dfw1"] = &Site{
		ID:       "site-1",
		Code:     "dfw1",
		Name:     "Dallas DC 1",
		Timezone: "UTC",
	}

	resolver := NewResolver(store, DefaultResolverConfig())
	defer resolver.Stop()

	// First resolution should cache
	site, err := resolver.Resolve(context.Background(), map[string]string{"site": "dfw1"})
	if err != nil {
		t.Fatalf("First resolve failed: %v", err)
	}
	if site.Name != "Dallas DC 1" {
		t.Errorf("Got name %v, want Dallas DC 1", site.Name)
	}

	// Update the store directly
	store.sites["dfw1"].Name = "Dallas Updated"

	// Should still get cached value
	site, err = resolver.Resolve(context.Background(), map[string]string{"site": "dfw1"})
	if err != nil {
		t.Fatalf("Second resolve failed: %v", err)
	}
	if site.Name != "Dallas DC 1" {
		t.Errorf("Expected cached value 'Dallas DC 1', got %v", site.Name)
	}

	// Invalidate cache
	resolver.InvalidateCache("dfw1")

	// Should now get updated value
	site, err = resolver.Resolve(context.Background(), map[string]string{"site": "dfw1"})
	if err != nil {
		t.Fatalf("Third resolve failed: %v", err)
	}
	if site.Name != "Dallas Updated" {
		t.Errorf("Expected updated value 'Dallas Updated', got %v", site.Name)
	}
}

func TestResolver_ResolutionPriority(t *testing.T) {
	store := newMockStore()

	store.sites["dfw1"] = &Site{ID: "site-1", Code: "dfw1", Name: "Site Label"}
	store.sites["nyc2"] = &Site{ID: "site-2", Code: "nyc2", Name: "Datacenter Label"}
	store.sites["lax1"] = &Site{ID: "site-3", Code: "lax1", Name: "POP Label"}
	store.sites["ord3"] = &Site{ID: "site-4", Code: "ord3", Name: "Location Label"}

	resolver := NewResolver(store, DefaultResolverConfig())
	defer resolver.Stop()

	// All labels present - site should win
	labels := map[string]string{
		"site":       "dfw1",
		"datacenter": "nyc2",
		"pop":        "lax1",
		"location":   "ord3",
		"instance":   "ord3-server01:9090",
	}

	site, err := resolver.Resolve(context.Background(), labels)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if site.Code != "dfw1" {
		t.Errorf("Expected dfw1 (site label priority), got %v", site.Code)
	}

	// Remove site label - datacenter should win
	delete(labels, "site")
	site, err = resolver.Resolve(context.Background(), labels)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if site.Code != "nyc2" {
		t.Errorf("Expected nyc2 (datacenter priority), got %v", site.Code)
	}

	// Remove datacenter - pop should win
	delete(labels, "datacenter")
	site, err = resolver.Resolve(context.Background(), labels)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if site.Code != "lax1" {
		t.Errorf("Expected lax1 (pop priority), got %v", site.Code)
	}
}
