// Package grpc provides gRPC service implementations.
package grpc

import (
	"context"
	"errors"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/kneutral-org/alerting-system/internal/equipment"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// EquipmentTypeService implements the EquipmentTypeServiceServer interface.
type EquipmentTypeService struct {
	routingv1.UnimplementedEquipmentTypeServiceServer
	store    equipment.Store
	resolver equipment.Resolver
	logger   zerolog.Logger
}

// NewEquipmentTypeService creates a new EquipmentTypeService.
func NewEquipmentTypeService(store equipment.Store, resolver equipment.Resolver, logger zerolog.Logger) *EquipmentTypeService {
	return &EquipmentTypeService{
		store:    store,
		resolver: resolver,
		logger:   logger.With().Str("service", "equipment_type").Logger(),
	}
}

// CreateEquipmentType creates a new equipment type.
func (s *EquipmentTypeService) CreateEquipmentType(ctx context.Context, req *routingv1.CreateEquipmentTypeRequest) (*routingv1.EquipmentType, error) {
	if req.EquipmentType == nil {
		return nil, status.Error(codes.InvalidArgument, "equipment_type is required")
	}

	if req.EquipmentType.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "equipment_type name is required")
	}

	s.logger.Info().
		Str("name", req.EquipmentType.Name).
		Str("category", req.EquipmentType.Category).
		Msg("creating equipment type")

	// Convert proto to internal model
	internalEq := protoToEquipmentType(req.EquipmentType)

	createdEq, err := s.store.Create(ctx, internalEq)
	if err != nil {
		if errors.Is(err, equipment.ErrDuplicateName) {
			return nil, status.Error(codes.AlreadyExists, "equipment type name already exists")
		}
		if errors.Is(err, equipment.ErrInvalidEquipmentType) {
			return nil, status.Error(codes.InvalidArgument, "invalid equipment type data")
		}
		s.logger.Error().Err(err).Msg("failed to create equipment type")
		return nil, status.Error(codes.Internal, "failed to create equipment type")
	}

	s.logger.Info().
		Str("id", createdEq.ID).
		Str("name", createdEq.Name).
		Msg("equipment type created")

	return equipmentTypeToProto(createdEq), nil
}

// GetEquipmentType retrieves an equipment type by ID.
func (s *EquipmentTypeService) GetEquipmentType(ctx context.Context, req *routingv1.GetEquipmentTypeRequest) (*routingv1.EquipmentType, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	foundEq, err := s.store.GetByID(ctx, req.Id)
	if err != nil {
		if errors.Is(err, equipment.ErrEquipmentTypeNotFound) {
			return nil, status.Error(codes.NotFound, "equipment type not found")
		}
		s.logger.Error().Err(err).Str("id", req.Id).Msg("failed to get equipment type")
		return nil, status.Error(codes.Internal, "failed to get equipment type")
	}

	return equipmentTypeToProto(foundEq), nil
}

// GetEquipmentTypeByName retrieves an equipment type by its name.
func (s *EquipmentTypeService) GetEquipmentTypeByName(ctx context.Context, req *routingv1.GetEquipmentTypeByNameRequest) (*routingv1.EquipmentType, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	foundEq, err := s.store.GetByName(ctx, req.Name)
	if err != nil {
		if errors.Is(err, equipment.ErrEquipmentTypeNotFound) {
			return nil, status.Error(codes.NotFound, "equipment type not found")
		}
		s.logger.Error().Err(err).Str("name", req.Name).Msg("failed to get equipment type by name")
		return nil, status.Error(codes.Internal, "failed to get equipment type by name")
	}

	return equipmentTypeToProto(foundEq), nil
}

// ListEquipmentTypes retrieves equipment types with optional filters.
func (s *EquipmentTypeService) ListEquipmentTypes(ctx context.Context, req *routingv1.ListEquipmentTypesRequest) (*routingv1.ListEquipmentTypesResponse, error) {
	// Build filter from request
	filter := &equipment.ListEquipmentTypesFilter{
		PageSize:  int(req.PageSize),
		PageToken: req.PageToken,
	}

	// Apply category filter
	if req.Category != "" {
		filter.Category = equipment.Category(req.Category)
	}

	// Apply vendor filter
	if req.Vendor != "" {
		filter.Vendor = req.Vendor
	}

	// Apply criticality filter
	if req.Criticality > 0 {
		filter.Criticality = int(req.Criticality)
	}

	types, nextPageToken, err := s.store.List(ctx, filter)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to list equipment types")
		return nil, status.Error(codes.Internal, "failed to list equipment types")
	}

	// Convert to proto
	protoTypes := make([]*routingv1.EquipmentType, 0, len(types))
	for _, eq := range types {
		protoTypes = append(protoTypes, equipmentTypeToProto(eq))
	}

	return &routingv1.ListEquipmentTypesResponse{
		EquipmentTypes: protoTypes,
		NextPageToken:  nextPageToken,
		TotalCount:     int32(len(protoTypes)),
	}, nil
}

// UpdateEquipmentType updates an existing equipment type.
func (s *EquipmentTypeService) UpdateEquipmentType(ctx context.Context, req *routingv1.UpdateEquipmentTypeRequest) (*routingv1.EquipmentType, error) {
	if req.EquipmentType == nil || req.EquipmentType.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "equipment_type with id is required")
	}

	s.logger.Info().
		Str("id", req.EquipmentType.Id).
		Str("name", req.EquipmentType.Name).
		Msg("updating equipment type")

	// Convert proto to internal model
	internalEq := protoToEquipmentType(req.EquipmentType)

	updatedEq, err := s.store.Update(ctx, internalEq)
	if err != nil {
		if errors.Is(err, equipment.ErrEquipmentTypeNotFound) {
			return nil, status.Error(codes.NotFound, "equipment type not found")
		}
		if errors.Is(err, equipment.ErrInvalidEquipmentType) {
			return nil, status.Error(codes.InvalidArgument, "invalid equipment type data")
		}
		s.logger.Error().Err(err).Str("id", req.EquipmentType.Id).Msg("failed to update equipment type")
		return nil, status.Error(codes.Internal, "failed to update equipment type")
	}

	// Invalidate resolver cache
	if s.resolver != nil {
		s.resolver.InvalidateCache(updatedEq.Name)
	}

	s.logger.Info().
		Str("id", updatedEq.ID).
		Msg("equipment type updated")

	return equipmentTypeToProto(updatedEq), nil
}

// DeleteEquipmentType deletes an equipment type by ID.
func (s *EquipmentTypeService) DeleteEquipmentType(ctx context.Context, req *routingv1.DeleteEquipmentTypeRequest) (*routingv1.DeleteEquipmentTypeResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	s.logger.Info().Str("id", req.Id).Msg("deleting equipment type")

	// Get the equipment type first to get the name for cache invalidation
	existingEq, err := s.store.GetByID(ctx, req.Id)
	if err != nil && !errors.Is(err, equipment.ErrEquipmentTypeNotFound) {
		s.logger.Error().Err(err).Str("id", req.Id).Msg("failed to get equipment type for deletion")
	}

	err = s.store.Delete(ctx, req.Id)
	if err != nil {
		if errors.Is(err, equipment.ErrEquipmentTypeNotFound) {
			return nil, status.Error(codes.NotFound, "equipment type not found")
		}
		s.logger.Error().Err(err).Str("id", req.Id).Msg("failed to delete equipment type")
		return nil, status.Error(codes.Internal, "failed to delete equipment type")
	}

	// Invalidate resolver cache
	if s.resolver != nil && existingEq != nil {
		s.resolver.InvalidateCache(existingEq.Name)
	}

	s.logger.Info().Str("id", req.Id).Msg("equipment type deleted")

	return &routingv1.DeleteEquipmentTypeResponse{Success: true}, nil
}

// ResolveEquipmentType resolves an equipment type from alert labels.
func (s *EquipmentTypeService) ResolveEquipmentType(ctx context.Context, req *routingv1.ResolveEquipmentTypeRequest) (*routingv1.ResolveEquipmentTypeResponse, error) {
	if len(req.Labels) == 0 {
		return &routingv1.ResolveEquipmentTypeResponse{
			Found:            false,
			ResolutionMethod: string(equipment.ResolutionMethodNotResolved),
		}, nil
	}

	resolved, err := s.resolver.Resolve(ctx, req.Labels)
	if err != nil {
		if errors.Is(err, equipment.ErrNoEquipmentResolved) {
			return &routingv1.ResolveEquipmentTypeResponse{
				Found:            false,
				ResolutionMethod: string(equipment.ResolutionMethodNotResolved),
			}, nil
		}
		s.logger.Error().Err(err).Msg("failed to resolve equipment type")
		return nil, status.Error(codes.Internal, "failed to resolve equipment type")
	}

	return &routingv1.ResolveEquipmentTypeResponse{
		EquipmentType:    equipmentTypeToProto(resolved.EquipmentType),
		Found:            true,
		ResolutionMethod: string(resolved.ResolutionMethod),
		MatchedValue:     resolved.MatchedValue,
	}, nil
}

// =============================================================================
// Conversion helpers
// =============================================================================

// protoToEquipmentType converts a proto EquipmentType to an internal equipment.EquipmentType.
func protoToEquipmentType(p *routingv1.EquipmentType) *equipment.EquipmentType {
	if p == nil {
		return nil
	}

	eq := &equipment.EquipmentType{
		ID:               p.Id,
		Name:             p.Name,
		Category:         equipment.Category(p.Category),
		Criticality:      int(p.SeverityBoost), // Map severity_boost to criticality
		EscalationPolicy: p.EscalationPolicyId,
	}

	// Handle required capabilities as metadata
	if len(p.RequiredCapabilities) > 0 {
		eq.Metadata = make(map[string]string)
		for i, cap := range p.RequiredCapabilities {
			eq.Metadata["capability_"+string(rune('0'+i))] = cap
		}
	}

	return eq
}

// equipmentTypeToProto converts an internal equipment.EquipmentType to a proto EquipmentType.
func equipmentTypeToProto(eq *equipment.EquipmentType) *routingv1.EquipmentType {
	if eq == nil {
		return nil
	}

	p := &routingv1.EquipmentType{
		Id:                 eq.ID,
		Name:               eq.Name,
		Category:           string(eq.Category),
		SeverityBoost:      int32(eq.Criticality), // Map criticality to severity_boost
		EscalationPolicyId: eq.EscalationPolicy,
	}

	// Extract required capabilities from metadata
	if eq.Metadata != nil {
		var caps []string
		for key, val := range eq.Metadata {
			if len(key) > 11 && key[:11] == "capability_" {
				caps = append(caps, val)
			}
		}
		p.RequiredCapabilities = caps
	}

	return p
}

// Ensure EquipmentTypeService implements the interface
var _ routingv1.EquipmentTypeServiceServer = (*EquipmentTypeService)(nil)
