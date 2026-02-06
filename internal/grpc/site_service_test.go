package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kneutral-org/alerting-system/internal/site"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// mockSiteStore is a mock implementation of site.Store for testing.
type mockSiteStore struct {
	sites       map[string]*site.Site
	sitesByCode map[string]*site.Site
	createErr   error
	getErr      error
	listErr     error
	updateErr   error
	deleteErr   error
}

func newMockSiteStore() *mockSiteStore {
	return &mockSiteStore{
		sites:       make(map[string]*site.Site),
		sitesByCode: make(map[string]*site.Site),
	}
}

func (m *mockSiteStore) GetByCode(ctx context.Context, code string) (*site.Site, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	s, ok := m.sitesByCode[code]
	if !ok {
		return nil, site.ErrSiteNotFound
	}
	return s, nil
}

func (m *mockSiteStore) GetByID(ctx context.Context, id string) (*site.Site, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	s, ok := m.sites[id]
	if !ok {
		return nil, site.ErrSiteNotFound
	}
	return s, nil
}

func (m *mockSiteStore) List(ctx context.Context, filter *site.ListSitesFilter) ([]*site.Site, string, error) {
	if m.listErr != nil {
		return nil, "", m.listErr
	}
	result := make([]*site.Site, 0, len(m.sites))
	for _, s := range m.sites {
		// Apply filters
		if filter != nil {
			if filter.SiteType != "" && s.SiteType != filter.SiteType {
				continue
			}
			if filter.Region != "" && s.Region != filter.Region {
				continue
			}
		}
		result = append(result, s)
	}
	return result, "", nil
}

func (m *mockSiteStore) Create(ctx context.Context, s *site.Site) (*site.Site, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if _, exists := m.sitesByCode[s.Code]; exists {
		return nil, site.ErrDuplicateCode
	}
	if s.ID == "" {
		s.ID = "generated-id"
	}
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()
	m.sites[s.ID] = s
	m.sitesByCode[s.Code] = s
	return s, nil
}

func (m *mockSiteStore) Update(ctx context.Context, s *site.Site) (*site.Site, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	if _, exists := m.sites[s.ID]; !exists {
		return nil, site.ErrSiteNotFound
	}
	s.UpdatedAt = time.Now()
	m.sites[s.ID] = s
	m.sitesByCode[s.Code] = s
	return s, nil
}

func (m *mockSiteStore) Delete(ctx context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	s, exists := m.sites[id]
	if !exists {
		return site.ErrSiteNotFound
	}
	delete(m.sites, id)
	delete(m.sitesByCode, s.Code)
	return nil
}

func (m *mockSiteStore) GetTeamByID(ctx context.Context, id string) (*site.Team, error) {
	return nil, site.ErrTeamNotFound
}

// Helper to add a site to the mock store
func (m *mockSiteStore) addSite(s *site.Site) {
	m.sites[s.ID] = s
	m.sitesByCode[s.Code] = s
}

func TestSiteService_CreateSite(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("success", func(t *testing.T) {
		store := newMockSiteStore()
		svc := NewSiteService(store, logger)

		req := &routingv1.CreateSiteRequest{
			Site: &routingv1.Site{
				Name:     "Test Datacenter",
				Code:     "NYC-DC1",
				Type:     routingv1.SiteType_SITE_TYPE_DATACENTER,
				Region:   "us-east-1",
				Country:  "USA",
				City:     "New York",
				Timezone: "America/New_York",
				Tier:     1,
			},
		}

		resp, err := svc.CreateSite(context.Background(), req)
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Id)
		assert.Equal(t, "Test Datacenter", resp.Name)
		assert.Equal(t, "NYC-DC1", resp.Code)
		assert.Equal(t, routingv1.SiteType_SITE_TYPE_DATACENTER, resp.Type)
		assert.Equal(t, "us-east-1", resp.Region)
		assert.Equal(t, int32(1), resp.Tier)
	})

	t.Run("nil site", func(t *testing.T) {
		store := newMockSiteStore()
		svc := NewSiteService(store, logger)

		_, err := svc.CreateSite(context.Background(), &routingv1.CreateSiteRequest{})
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Contains(t, st.Message(), "site is required")
	})

	t.Run("missing name", func(t *testing.T) {
		store := newMockSiteStore()
		svc := NewSiteService(store, logger)

		req := &routingv1.CreateSiteRequest{
			Site: &routingv1.Site{
				Code: "NYC-DC1",
			},
		}

		_, err := svc.CreateSite(context.Background(), req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Contains(t, st.Message(), "name is required")
	})

	t.Run("missing code", func(t *testing.T) {
		store := newMockSiteStore()
		svc := NewSiteService(store, logger)

		req := &routingv1.CreateSiteRequest{
			Site: &routingv1.Site{
				Name: "Test Datacenter",
			},
		}

		_, err := svc.CreateSite(context.Background(), req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Contains(t, st.Message(), "code is required")
	})

	t.Run("duplicate code", func(t *testing.T) {
		store := newMockSiteStore()
		store.addSite(&site.Site{
			ID:   "existing-id",
			Name: "Existing Site",
			Code: "NYC-DC1",
		})
		svc := NewSiteService(store, logger)

		req := &routingv1.CreateSiteRequest{
			Site: &routingv1.Site{
				Name: "New Site",
				Code: "NYC-DC1",
			},
		}

		_, err := svc.CreateSite(context.Background(), req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.AlreadyExists, st.Code())
	})
}

func TestSiteService_GetSite(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("success", func(t *testing.T) {
		store := newMockSiteStore()
		now := time.Now()
		store.addSite(&site.Site{
			ID:        "site-123",
			Name:      "Test Datacenter",
			Code:      "NYC-DC1",
			SiteType:  site.SiteTypeDatacenter,
			Region:    "us-east-1",
			Timezone:  "America/New_York",
			CreatedAt: now,
			UpdatedAt: now,
		})
		svc := NewSiteService(store, logger)

		resp, err := svc.GetSite(context.Background(), &routingv1.GetSiteRequest{Id: "site-123"})
		require.NoError(t, err)
		assert.Equal(t, "site-123", resp.Id)
		assert.Equal(t, "Test Datacenter", resp.Name)
		assert.Equal(t, "NYC-DC1", resp.Code)
	})

	t.Run("missing id", func(t *testing.T) {
		store := newMockSiteStore()
		svc := NewSiteService(store, logger)

		_, err := svc.GetSite(context.Background(), &routingv1.GetSiteRequest{})
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("not found", func(t *testing.T) {
		store := newMockSiteStore()
		svc := NewSiteService(store, logger)

		_, err := svc.GetSite(context.Background(), &routingv1.GetSiteRequest{Id: "nonexistent"})
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

func TestSiteService_GetSiteByCode(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("success", func(t *testing.T) {
		store := newMockSiteStore()
		now := time.Now()
		store.addSite(&site.Site{
			ID:        "site-123",
			Name:      "Test Datacenter",
			Code:      "NYC-DC1",
			SiteType:  site.SiteTypeDatacenter,
			Region:    "us-east-1",
			Timezone:  "America/New_York",
			CreatedAt: now,
			UpdatedAt: now,
		})
		svc := NewSiteService(store, logger)

		resp, err := svc.GetSiteByCode(context.Background(), &routingv1.GetSiteByCodeRequest{Code: "NYC-DC1"})
		require.NoError(t, err)
		assert.Equal(t, "site-123", resp.Id)
		assert.Equal(t, "NYC-DC1", resp.Code)
	})

	t.Run("missing code", func(t *testing.T) {
		store := newMockSiteStore()
		svc := NewSiteService(store, logger)

		_, err := svc.GetSiteByCode(context.Background(), &routingv1.GetSiteByCodeRequest{})
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("not found", func(t *testing.T) {
		store := newMockSiteStore()
		svc := NewSiteService(store, logger)

		_, err := svc.GetSiteByCode(context.Background(), &routingv1.GetSiteByCodeRequest{Code: "NONEXISTENT"})
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

func TestSiteService_ListSites(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("success - all sites", func(t *testing.T) {
		store := newMockSiteStore()
		now := time.Now()
		store.addSite(&site.Site{
			ID:        "site-1",
			Name:      "NYC Datacenter",
			Code:      "NYC-DC1",
			SiteType:  site.SiteTypeDatacenter,
			Region:    "us-east-1",
			CreatedAt: now,
			UpdatedAt: now,
		})
		store.addSite(&site.Site{
			ID:        "site-2",
			Name:      "LAX POP",
			Code:      "LAX-POP1",
			SiteType:  site.SiteTypePOP,
			Region:    "us-west-2",
			CreatedAt: now,
			UpdatedAt: now,
		})
		svc := NewSiteService(store, logger)

		resp, err := svc.ListSites(context.Background(), &routingv1.ListSitesRequest{})
		require.NoError(t, err)
		assert.Len(t, resp.Sites, 2)
	})

	t.Run("filter by type", func(t *testing.T) {
		store := newMockSiteStore()
		now := time.Now()
		store.addSite(&site.Site{
			ID:        "site-1",
			Name:      "NYC Datacenter",
			Code:      "NYC-DC1",
			SiteType:  site.SiteTypeDatacenter,
			Region:    "us-east-1",
			CreatedAt: now,
			UpdatedAt: now,
		})
		store.addSite(&site.Site{
			ID:        "site-2",
			Name:      "LAX POP",
			Code:      "LAX-POP1",
			SiteType:  site.SiteTypePOP,
			Region:    "us-west-2",
			CreatedAt: now,
			UpdatedAt: now,
		})
		svc := NewSiteService(store, logger)

		resp, err := svc.ListSites(context.Background(), &routingv1.ListSitesRequest{
			Type: routingv1.SiteType_SITE_TYPE_DATACENTER,
		})
		require.NoError(t, err)
		assert.Len(t, resp.Sites, 1)
		assert.Equal(t, "NYC-DC1", resp.Sites[0].Code)
	})

	t.Run("filter by region", func(t *testing.T) {
		store := newMockSiteStore()
		now := time.Now()
		store.addSite(&site.Site{
			ID:        "site-1",
			Name:      "NYC Datacenter",
			Code:      "NYC-DC1",
			SiteType:  site.SiteTypeDatacenter,
			Region:    "us-east-1",
			CreatedAt: now,
			UpdatedAt: now,
		})
		store.addSite(&site.Site{
			ID:        "site-2",
			Name:      "LAX POP",
			Code:      "LAX-POP1",
			SiteType:  site.SiteTypePOP,
			Region:    "us-west-2",
			CreatedAt: now,
			UpdatedAt: now,
		})
		svc := NewSiteService(store, logger)

		resp, err := svc.ListSites(context.Background(), &routingv1.ListSitesRequest{
			Region: "us-west-2",
		})
		require.NoError(t, err)
		assert.Len(t, resp.Sites, 1)
		assert.Equal(t, "LAX-POP1", resp.Sites[0].Code)
	})
}

func TestSiteService_UpdateSite(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("success", func(t *testing.T) {
		store := newMockSiteStore()
		now := time.Now()
		store.addSite(&site.Site{
			ID:        "site-123",
			Name:      "Test Datacenter",
			Code:      "NYC-DC1",
			SiteType:  site.SiteTypeDatacenter,
			Region:    "us-east-1",
			CreatedAt: now,
			UpdatedAt: now,
		})
		svc := NewSiteService(store, logger)

		req := &routingv1.UpdateSiteRequest{
			Site: &routingv1.Site{
				Id:       "site-123",
				Name:     "Updated Datacenter",
				Code:     "NYC-DC1",
				Type:     routingv1.SiteType_SITE_TYPE_DATACENTER,
				Region:   "us-east-2",
				Timezone: "America/New_York",
			},
		}

		resp, err := svc.UpdateSite(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, "site-123", resp.Id)
		assert.Equal(t, "Updated Datacenter", resp.Name)
		assert.Equal(t, "us-east-2", resp.Region)
	})

	t.Run("nil site", func(t *testing.T) {
		store := newMockSiteStore()
		svc := NewSiteService(store, logger)

		_, err := svc.UpdateSite(context.Background(), &routingv1.UpdateSiteRequest{})
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("missing id", func(t *testing.T) {
		store := newMockSiteStore()
		svc := NewSiteService(store, logger)

		req := &routingv1.UpdateSiteRequest{
			Site: &routingv1.Site{
				Name: "Updated Datacenter",
			},
		}

		_, err := svc.UpdateSite(context.Background(), req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("not found", func(t *testing.T) {
		store := newMockSiteStore()
		svc := NewSiteService(store, logger)

		req := &routingv1.UpdateSiteRequest{
			Site: &routingv1.Site{
				Id:   "nonexistent",
				Name: "Updated Datacenter",
				Code: "NYC-DC1",
			},
		}

		_, err := svc.UpdateSite(context.Background(), req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

func TestSiteService_DeleteSite(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("success", func(t *testing.T) {
		store := newMockSiteStore()
		now := time.Now()
		store.addSite(&site.Site{
			ID:        "site-123",
			Name:      "Test Datacenter",
			Code:      "NYC-DC1",
			SiteType:  site.SiteTypeDatacenter,
			CreatedAt: now,
			UpdatedAt: now,
		})
		svc := NewSiteService(store, logger)

		resp, err := svc.DeleteSite(context.Background(), &routingv1.DeleteSiteRequest{Id: "site-123"})
		require.NoError(t, err)
		assert.True(t, resp.Success)

		// Verify site is deleted
		_, err = store.GetByID(context.Background(), "site-123")
		assert.ErrorIs(t, err, site.ErrSiteNotFound)
	})

	t.Run("missing id", func(t *testing.T) {
		store := newMockSiteStore()
		svc := NewSiteService(store, logger)

		_, err := svc.DeleteSite(context.Background(), &routingv1.DeleteSiteRequest{})
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("not found", func(t *testing.T) {
		store := newMockSiteStore()
		svc := NewSiteService(store, logger)

		_, err := svc.DeleteSite(context.Background(), &routingv1.DeleteSiteRequest{Id: "nonexistent"})
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.NotFound, st.Code())
	})
}

func TestConversions(t *testing.T) {
	t.Run("protoToSite", func(t *testing.T) {
		now := time.Now()
		tier := int32(2)
		p := &routingv1.Site{
			Id:                 "site-123",
			Name:               "Test Site",
			Code:               "NYC-DC1",
			Type:               routingv1.SiteType_SITE_TYPE_DATACENTER,
			Region:             "us-east-1",
			Country:            "USA",
			City:               "New York",
			Timezone:           "America/New_York",
			Tier:               tier,
			PrimaryTeamId:      "team-1",
			SecondaryTeamId:    "team-2",
			EscalationPolicyId: "policy-1",
			Metadata:           map[string]string{"env": "production"},
			BusinessHours: []*routingv1.TimeWindow{
				{
					DaysOfWeek: []int32{1, 2, 3, 4, 5},
					StartTime:  "09:00",
					EndTime:    "17:00",
				},
			},
			CreatedAt: timestamppb.New(now),
			UpdatedAt: timestamppb.New(now),
		}

		s := protoToSite(p)
		assert.Equal(t, "site-123", s.ID)
		assert.Equal(t, "Test Site", s.Name)
		assert.Equal(t, "NYC-DC1", s.Code)
		assert.Equal(t, site.SiteTypeDatacenter, s.SiteType)
		assert.Equal(t, "us-east-1", s.Region)
		assert.Equal(t, "USA", s.Country)
		assert.Equal(t, "New York", s.City)
		assert.Equal(t, "America/New_York", s.Timezone)
		assert.NotNil(t, s.Tier)
		assert.Equal(t, 2, *s.Tier)
		assert.NotNil(t, s.PrimaryTeamID)
		assert.Equal(t, "team-1", *s.PrimaryTeamID)
		assert.NotNil(t, s.SecondaryTeamID)
		assert.Equal(t, "team-2", *s.SecondaryTeamID)
		assert.NotNil(t, s.DefaultEscalationPolicyID)
		assert.Equal(t, "policy-1", *s.DefaultEscalationPolicyID)
		assert.Equal(t, "production", s.Labels["env"])
		assert.NotNil(t, s.BusinessHours)
		assert.Equal(t, "09:00", s.BusinessHours.Start)
		assert.Equal(t, "17:00", s.BusinessHours.End)
		assert.Equal(t, []int{1, 2, 3, 4, 5}, s.BusinessHours.Days)
	})

	t.Run("protoToSite nil", func(t *testing.T) {
		assert.Nil(t, protoToSite(nil))
	})

	t.Run("siteToProto", func(t *testing.T) {
		now := time.Now()
		tier := 2
		primaryTeam := "team-1"
		secondaryTeam := "team-2"
		escPolicy := "policy-1"

		s := &site.Site{
			ID:                        "site-123",
			Name:                      "Test Site",
			Code:                      "NYC-DC1",
			SiteType:                  site.SiteTypeDatacenter,
			Region:                    "us-east-1",
			Country:                   "USA",
			City:                      "New York",
			Timezone:                  "America/New_York",
			Tier:                      &tier,
			PrimaryTeamID:             &primaryTeam,
			SecondaryTeamID:           &secondaryTeam,
			DefaultEscalationPolicyID: &escPolicy,
			Labels:                    map[string]string{"env": "production"},
			BusinessHours: &site.BusinessHours{
				Start: "09:00",
				End:   "17:00",
				Days:  []int{1, 2, 3, 4, 5},
			},
			CreatedAt: now,
			UpdatedAt: now,
		}

		p := siteToProto(s)
		assert.Equal(t, "site-123", p.Id)
		assert.Equal(t, "Test Site", p.Name)
		assert.Equal(t, "NYC-DC1", p.Code)
		assert.Equal(t, routingv1.SiteType_SITE_TYPE_DATACENTER, p.Type)
		assert.Equal(t, "us-east-1", p.Region)
		assert.Equal(t, "USA", p.Country)
		assert.Equal(t, "New York", p.City)
		assert.Equal(t, "America/New_York", p.Timezone)
		assert.Equal(t, int32(2), p.Tier)
		assert.Equal(t, "team-1", p.PrimaryTeamId)
		assert.Equal(t, "team-2", p.SecondaryTeamId)
		assert.Equal(t, "policy-1", p.EscalationPolicyId)
		assert.Equal(t, "production", p.Metadata["env"])
		assert.Len(t, p.BusinessHours, 1)
		assert.Equal(t, "09:00", p.BusinessHours[0].StartTime)
		assert.Equal(t, "17:00", p.BusinessHours[0].EndTime)
		assert.Equal(t, []int32{1, 2, 3, 4, 5}, p.BusinessHours[0].DaysOfWeek)
	})

	t.Run("siteToProto nil", func(t *testing.T) {
		assert.Nil(t, siteToProto(nil))
	})

	t.Run("site type conversions", func(t *testing.T) {
		// Test all site type conversions
		testCases := []struct {
			protoType    routingv1.SiteType
			internalType site.SiteType
		}{
			{routingv1.SiteType_SITE_TYPE_DATACENTER, site.SiteTypeDatacenter},
			{routingv1.SiteType_SITE_TYPE_POP, site.SiteTypePOP},
			{routingv1.SiteType_SITE_TYPE_COLOCATION, site.SiteTypeHub},
			{routingv1.SiteType_SITE_TYPE_EDGE, site.SiteTypeCustomerPremise},
		}

		for _, tc := range testCases {
			// Proto to internal
			internal := protoSiteTypeToInternal(tc.protoType)
			assert.Equal(t, tc.internalType, internal)

			// Internal to proto
			proto := internalSiteTypeToProto(tc.internalType)
			assert.Equal(t, tc.protoType, proto)
		}
	})
}
