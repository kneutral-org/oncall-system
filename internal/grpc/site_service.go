// Package grpc provides gRPC service implementations.
package grpc

import (
	"context"
	"errors"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kneutral-org/alerting-system/internal/site"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// SiteService implements the SiteServiceServer interface.
type SiteService struct {
	routingv1.UnimplementedSiteServiceServer
	store  site.Store
	logger zerolog.Logger
}

// NewSiteService creates a new SiteService.
func NewSiteService(store site.Store, logger zerolog.Logger) *SiteService {
	return &SiteService{
		store:  store,
		logger: logger.With().Str("service", "site").Logger(),
	}
}

// CreateSite creates a new site.
func (s *SiteService) CreateSite(ctx context.Context, req *routingv1.CreateSiteRequest) (*routingv1.Site, error) {
	if req.Site == nil {
		return nil, status.Error(codes.InvalidArgument, "site is required")
	}

	if req.Site.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "site name is required")
	}

	if req.Site.Code == "" {
		return nil, status.Error(codes.InvalidArgument, "site code is required")
	}

	s.logger.Info().
		Str("name", req.Site.Name).
		Str("code", req.Site.Code).
		Str("type", req.Site.Type.String()).
		Msg("creating site")

	// Convert proto to internal model
	internalSite := protoToSite(req.Site)

	createdSite, err := s.store.Create(ctx, internalSite)
	if err != nil {
		if errors.Is(err, site.ErrDuplicateCode) {
			return nil, status.Error(codes.AlreadyExists, "site code already exists")
		}
		if errors.Is(err, site.ErrInvalidSite) {
			return nil, status.Error(codes.InvalidArgument, "invalid site data")
		}
		s.logger.Error().Err(err).Msg("failed to create site")
		return nil, status.Error(codes.Internal, "failed to create site")
	}

	s.logger.Info().
		Str("id", createdSite.ID).
		Str("code", createdSite.Code).
		Msg("site created")

	return siteToProto(createdSite), nil
}

// GetSite retrieves a site by ID.
func (s *SiteService) GetSite(ctx context.Context, req *routingv1.GetSiteRequest) (*routingv1.Site, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	foundSite, err := s.store.GetByID(ctx, req.Id)
	if err != nil {
		if errors.Is(err, site.ErrSiteNotFound) {
			return nil, status.Error(codes.NotFound, "site not found")
		}
		s.logger.Error().Err(err).Str("id", req.Id).Msg("failed to get site")
		return nil, status.Error(codes.Internal, "failed to get site")
	}

	return siteToProto(foundSite), nil
}

// GetSiteByCode retrieves a site by its unique code.
func (s *SiteService) GetSiteByCode(ctx context.Context, req *routingv1.GetSiteByCodeRequest) (*routingv1.Site, error) {
	if req.Code == "" {
		return nil, status.Error(codes.InvalidArgument, "code is required")
	}

	foundSite, err := s.store.GetByCode(ctx, req.Code)
	if err != nil {
		if errors.Is(err, site.ErrSiteNotFound) {
			return nil, status.Error(codes.NotFound, "site not found")
		}
		s.logger.Error().Err(err).Str("code", req.Code).Msg("failed to get site by code")
		return nil, status.Error(codes.Internal, "failed to get site by code")
	}

	return siteToProto(foundSite), nil
}

// ListSites retrieves sites with optional filters.
func (s *SiteService) ListSites(ctx context.Context, req *routingv1.ListSitesRequest) (*routingv1.ListSitesResponse, error) {
	// Build filter from request
	filter := &site.ListSitesFilter{
		PageSize:  int(req.PageSize),
		PageToken: req.PageToken,
	}

	// Apply type filter
	if req.Type != routingv1.SiteType_SITE_TYPE_UNSPECIFIED {
		filter.SiteType = protoSiteTypeToInternal(req.Type)
	}

	// Apply region filter
	if req.Region != "" {
		filter.Region = req.Region
	}

	sites, nextPageToken, err := s.store.List(ctx, filter)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to list sites")
		return nil, status.Error(codes.Internal, "failed to list sites")
	}

	// Convert to proto
	protoSites := make([]*routingv1.Site, 0, len(sites))
	for _, s := range sites {
		protoSites = append(protoSites, siteToProto(s))
	}

	return &routingv1.ListSitesResponse{
		Sites:         protoSites,
		NextPageToken: nextPageToken,
		TotalCount:    int32(len(protoSites)),
	}, nil
}

// UpdateSite updates an existing site.
func (s *SiteService) UpdateSite(ctx context.Context, req *routingv1.UpdateSiteRequest) (*routingv1.Site, error) {
	if req.Site == nil || req.Site.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "site with id is required")
	}

	s.logger.Info().
		Str("id", req.Site.Id).
		Str("name", req.Site.Name).
		Msg("updating site")

	// Convert proto to internal model
	internalSite := protoToSite(req.Site)

	updatedSite, err := s.store.Update(ctx, internalSite)
	if err != nil {
		if errors.Is(err, site.ErrSiteNotFound) {
			return nil, status.Error(codes.NotFound, "site not found")
		}
		if errors.Is(err, site.ErrInvalidSite) {
			return nil, status.Error(codes.InvalidArgument, "invalid site data")
		}
		s.logger.Error().Err(err).Str("id", req.Site.Id).Msg("failed to update site")
		return nil, status.Error(codes.Internal, "failed to update site")
	}

	s.logger.Info().
		Str("id", updatedSite.ID).
		Msg("site updated")

	return siteToProto(updatedSite), nil
}

// DeleteSite deletes a site by ID.
func (s *SiteService) DeleteSite(ctx context.Context, req *routingv1.DeleteSiteRequest) (*routingv1.DeleteSiteResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	s.logger.Info().Str("id", req.Id).Msg("deleting site")

	err := s.store.Delete(ctx, req.Id)
	if err != nil {
		if errors.Is(err, site.ErrSiteNotFound) {
			return nil, status.Error(codes.NotFound, "site not found")
		}
		s.logger.Error().Err(err).Str("id", req.Id).Msg("failed to delete site")
		return nil, status.Error(codes.Internal, "failed to delete site")
	}

	s.logger.Info().Str("id", req.Id).Msg("site deleted")

	return &routingv1.DeleteSiteResponse{Success: true}, nil
}

// =============================================================================
// Conversion helpers
// =============================================================================

// protoToSite converts a proto Site to an internal site.Site.
func protoToSite(p *routingv1.Site) *site.Site {
	if p == nil {
		return nil
	}

	s := &site.Site{
		ID:       p.Id,
		Name:     p.Name,
		Code:     p.Code,
		SiteType: protoSiteTypeToInternal(p.Type),
		Region:   p.Region,
		Country:  p.Country,
		City:     p.City,
		Timezone: p.Timezone,
		Labels:   make(map[string]string),
	}

	// Handle tier
	if p.Tier > 0 {
		tier := int(p.Tier)
		s.Tier = &tier
	}

	// Handle team IDs
	if p.PrimaryTeamId != "" {
		s.PrimaryTeamID = &p.PrimaryTeamId
	}
	if p.SecondaryTeamId != "" {
		s.SecondaryTeamID = &p.SecondaryTeamId
	}
	if p.EscalationPolicyId != "" {
		s.DefaultEscalationPolicyID = &p.EscalationPolicyId
	}

	// Handle metadata as labels
	if p.Metadata != nil {
		for k, v := range p.Metadata {
			s.Labels[k] = v
		}
	}

	// Handle business hours
	if len(p.BusinessHours) > 0 && p.BusinessHours[0] != nil {
		tw := p.BusinessHours[0]
		days := make([]int, 0, len(tw.DaysOfWeek))
		for _, d := range tw.DaysOfWeek {
			days = append(days, int(d))
		}
		s.BusinessHours = &site.BusinessHours{
			Start: tw.StartTime,
			End:   tw.EndTime,
			Days:  days,
		}
	}

	// Handle timestamps
	if p.CreatedAt != nil {
		s.CreatedAt = p.CreatedAt.AsTime()
	}
	if p.UpdatedAt != nil {
		s.UpdatedAt = p.UpdatedAt.AsTime()
	}

	return s
}

// siteToProto converts an internal site.Site to a proto Site.
func siteToProto(s *site.Site) *routingv1.Site {
	if s == nil {
		return nil
	}

	p := &routingv1.Site{
		Id:        s.ID,
		Name:      s.Name,
		Code:      s.Code,
		Type:      internalSiteTypeToProto(s.SiteType),
		Region:    s.Region,
		Country:   s.Country,
		City:      s.City,
		Timezone:  s.Timezone,
		Metadata:  make(map[string]string),
		CreatedAt: timestamppb.New(s.CreatedAt),
		UpdatedAt: timestamppb.New(s.UpdatedAt),
	}

	// Handle tier
	if s.Tier != nil {
		p.Tier = int32(*s.Tier)
	}

	// Handle team IDs
	if s.PrimaryTeamID != nil {
		p.PrimaryTeamId = *s.PrimaryTeamID
	}
	if s.SecondaryTeamID != nil {
		p.SecondaryTeamId = *s.SecondaryTeamID
	}
	if s.DefaultEscalationPolicyID != nil {
		p.EscalationPolicyId = *s.DefaultEscalationPolicyID
	}

	// Handle labels as metadata
	if s.Labels != nil {
		for k, v := range s.Labels {
			p.Metadata[k] = v
		}
	}

	// Handle business hours
	if s.BusinessHours != nil {
		days := make([]int32, 0, len(s.BusinessHours.Days))
		for _, d := range s.BusinessHours.Days {
			days = append(days, int32(d))
		}
		p.BusinessHours = []*routingv1.TimeWindow{
			{
				DaysOfWeek: days,
				StartTime:  s.BusinessHours.Start,
				EndTime:    s.BusinessHours.End,
			},
		}
	}

	return p
}

// protoSiteTypeToInternal converts a proto SiteType to an internal site.SiteType.
func protoSiteTypeToInternal(t routingv1.SiteType) site.SiteType {
	switch t {
	case routingv1.SiteType_SITE_TYPE_DATACENTER:
		return site.SiteTypeDatacenter
	case routingv1.SiteType_SITE_TYPE_POP:
		return site.SiteTypePOP
	case routingv1.SiteType_SITE_TYPE_COLOCATION:
		return site.SiteTypeHub // Map colocation to hub
	case routingv1.SiteType_SITE_TYPE_EDGE:
		return site.SiteTypeCustomerPremise // Map edge to customer premise
	case routingv1.SiteType_SITE_TYPE_OFFICE:
		return site.SiteTypeHub // Map office to hub
	default:
		return site.SiteTypeDatacenter
	}
}

// internalSiteTypeToProto converts an internal site.SiteType to a proto SiteType.
func internalSiteTypeToProto(t site.SiteType) routingv1.SiteType {
	switch t {
	case site.SiteTypeDatacenter:
		return routingv1.SiteType_SITE_TYPE_DATACENTER
	case site.SiteTypePOP:
		return routingv1.SiteType_SITE_TYPE_POP
	case site.SiteTypeHub:
		return routingv1.SiteType_SITE_TYPE_COLOCATION
	case site.SiteTypeCustomerPremise:
		return routingv1.SiteType_SITE_TYPE_EDGE
	default:
		return routingv1.SiteType_SITE_TYPE_UNSPECIFIED
	}
}

// Ensure SiteService implements the interface
var _ routingv1.SiteServiceServer = (*SiteService)(nil)
