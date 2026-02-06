// Package grpc provides gRPC service implementations.
package grpc

import (
	"context"
	"errors"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/kneutral-org/alerting-system/internal/customer"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// CustomerTierService implements the CustomerTierServiceServer interface.
type CustomerTierService struct {
	routingv1.UnimplementedCustomerTierServiceServer
	tierStore     customer.TierStore
	customerStore customer.Store
	resolver      customer.Resolver
	logger        zerolog.Logger
}

// NewCustomerTierService creates a new CustomerTierService.
func NewCustomerTierService(
	tierStore customer.TierStore,
	customerStore customer.Store,
	resolver customer.Resolver,
	logger zerolog.Logger,
) *CustomerTierService {
	return &CustomerTierService{
		tierStore:     tierStore,
		customerStore: customerStore,
		resolver:      resolver,
		logger:        logger.With().Str("service", "customer_tier").Logger(),
	}
}

// CreateCustomerTier creates a new customer tier.
func (s *CustomerTierService) CreateCustomerTier(ctx context.Context, req *routingv1.CreateCustomerTierRequest) (*routingv1.CustomerTier, error) {
	if req.Tier == nil {
		return nil, status.Error(codes.InvalidArgument, "tier is required")
	}

	if req.Tier.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "tier name is required")
	}

	s.logger.Info().
		Str("name", req.Tier.Name).
		Int32("level", req.Tier.Level).
		Msg("creating customer tier")

	tier := protoToTier(req.Tier)

	created, err := s.tierStore.Create(ctx, tier)
	if err != nil {
		if errors.Is(err, customer.ErrDuplicateTierName) {
			return nil, status.Error(codes.AlreadyExists, "tier name already exists")
		}
		if errors.Is(err, customer.ErrDuplicateTierLevel) {
			return nil, status.Error(codes.AlreadyExists, "tier level already exists")
		}
		s.logger.Error().Err(err).Msg("failed to create customer tier")
		return nil, status.Error(codes.Internal, "failed to create customer tier")
	}

	s.logger.Info().
		Str("id", created.ID).
		Str("name", created.Name).
		Msg("customer tier created")

	return tierToProto(created), nil
}

// GetCustomerTier retrieves a customer tier by ID.
func (s *CustomerTierService) GetCustomerTier(ctx context.Context, req *routingv1.GetCustomerTierRequest) (*routingv1.CustomerTier, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	tier, err := s.tierStore.GetByID(ctx, req.Id)
	if err != nil {
		if errors.Is(err, customer.ErrTierNotFound) {
			return nil, status.Error(codes.NotFound, "customer tier not found")
		}
		s.logger.Error().Err(err).Str("id", req.Id).Msg("failed to get customer tier")
		return nil, status.Error(codes.Internal, "failed to get customer tier")
	}

	return tierToProto(tier), nil
}

// ListCustomerTiers retrieves customer tiers with optional filters.
func (s *CustomerTierService) ListCustomerTiers(ctx context.Context, req *routingv1.ListCustomerTiersRequest) (*routingv1.ListCustomerTiersResponse, error) {
	filter := &customer.ListCustomerTiersFilter{
		PageSize:  int(req.PageSize),
		PageToken: req.PageToken,
	}

	tiers, nextPageToken, err := s.tierStore.List(ctx, filter)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to list customer tiers")
		return nil, status.Error(codes.Internal, "failed to list customer tiers")
	}

	protoTiers := make([]*routingv1.CustomerTier, 0, len(tiers))
	for _, tier := range tiers {
		protoTiers = append(protoTiers, tierToProto(tier))
	}

	return &routingv1.ListCustomerTiersResponse{
		Tiers:         protoTiers,
		NextPageToken: nextPageToken,
	}, nil
}

// UpdateCustomerTier updates an existing customer tier.
func (s *CustomerTierService) UpdateCustomerTier(ctx context.Context, req *routingv1.UpdateCustomerTierRequest) (*routingv1.CustomerTier, error) {
	if req.Tier == nil || req.Tier.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "tier with id is required")
	}

	s.logger.Info().
		Str("id", req.Tier.Id).
		Str("name", req.Tier.Name).
		Msg("updating customer tier")

	tier := protoToTier(req.Tier)

	updated, err := s.tierStore.Update(ctx, tier)
	if err != nil {
		if errors.Is(err, customer.ErrTierNotFound) {
			return nil, status.Error(codes.NotFound, "customer tier not found")
		}
		if errors.Is(err, customer.ErrDuplicateTierName) {
			return nil, status.Error(codes.AlreadyExists, "tier name already exists")
		}
		if errors.Is(err, customer.ErrDuplicateTierLevel) {
			return nil, status.Error(codes.AlreadyExists, "tier level already exists")
		}
		s.logger.Error().Err(err).Str("id", req.Tier.Id).Msg("failed to update customer tier")
		return nil, status.Error(codes.Internal, "failed to update customer tier")
	}

	s.logger.Info().
		Str("id", updated.ID).
		Msg("customer tier updated")

	return tierToProto(updated), nil
}

// DeleteCustomerTier deletes a customer tier by ID.
func (s *CustomerTierService) DeleteCustomerTier(ctx context.Context, req *routingv1.DeleteCustomerTierRequest) (*routingv1.DeleteCustomerTierResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	s.logger.Info().Str("id", req.Id).Msg("deleting customer tier")

	err := s.tierStore.Delete(ctx, req.Id)
	if err != nil {
		if errors.Is(err, customer.ErrTierNotFound) {
			return nil, status.Error(codes.NotFound, "customer tier not found")
		}
		s.logger.Error().Err(err).Str("id", req.Id).Msg("failed to delete customer tier")
		return nil, status.Error(codes.Internal, "failed to delete customer tier")
	}

	s.logger.Info().Str("id", req.Id).Msg("customer tier deleted")

	return &routingv1.DeleteCustomerTierResponse{Success: true}, nil
}

// ResolveCustomerTier resolves a customer tier from labels.
func (s *CustomerTierService) ResolveCustomerTier(ctx context.Context, req *routingv1.ResolveCustomerTierRequest) (*routingv1.ResolveCustomerTierResponse, error) {
	var tier *customer.CustomerTier
	var err error

	// Try to resolve by customer ID first
	if req.CustomerId != "" {
		var tierConfig *customer.TierConfig
		tierConfig, err = s.resolver.GetTierConfig(ctx, req.CustomerId)
		if err == nil && tierConfig != nil && tierConfig.Tier != nil {
			tier = tierConfig.Tier
		}
	}

	// Try to resolve from labels if no customer ID or not found
	if tier == nil && len(req.Labels) > 0 {
		_, tierConfig, err := s.resolver.ResolveWithTier(ctx, req.Labels)
		if err == nil && tierConfig != nil && tierConfig.Tier != nil {
			tier = tierConfig.Tier
		}
	}

	if tier == nil {
		return &routingv1.ResolveCustomerTierResponse{
			Found: false,
		}, nil
	}

	return &routingv1.ResolveCustomerTierResponse{
		Tier:  tierToProto(tier),
		Found: true,
	}, nil
}

// Helper functions to convert between proto and domain types

func protoToTier(p *routingv1.CustomerTier) *customer.CustomerTier {
	tier := &customer.CustomerTier{
		ID:                   p.Id,
		Name:                 p.Name,
		Level:                int(p.Level),
		EscalationMultiplier: float64(p.EscalationMultiplier),
		Metadata:             p.Metadata,
	}

	if p.CriticalResponse != nil {
		tier.CriticalResponseTime = p.CriticalResponse.AsDuration()
	}
	if p.HighResponse != nil {
		tier.HighResponseTime = p.HighResponse.AsDuration()
	}
	if p.MediumResponse != nil {
		tier.MediumResponseTime = p.MediumResponse.AsDuration()
	}

	if p.DedicatedTeamId != "" {
		tier.DedicatedTeamID = &p.DedicatedTeamId
	}

	return tier
}

func tierToProto(t *customer.CustomerTier) *routingv1.CustomerTier {
	proto := &routingv1.CustomerTier{
		Id:                   t.ID,
		Name:                 t.Name,
		Level:                int32(t.Level),
		CriticalResponse:     durationpb.New(t.CriticalResponseTime),
		HighResponse:         durationpb.New(t.HighResponseTime),
		MediumResponse:       durationpb.New(t.MediumResponseTime),
		EscalationMultiplier: float32(t.EscalationMultiplier),
		Metadata:             t.Metadata,
	}

	if t.DedicatedTeamID != nil {
		proto.DedicatedTeamId = *t.DedicatedTeamID
	}

	return proto
}

// Ensure CustomerTierService implements the interface
var _ routingv1.CustomerTierServiceServer = (*CustomerTierService)(nil)
