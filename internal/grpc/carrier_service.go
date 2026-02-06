// Package grpc provides gRPC service implementations.
package grpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/kneutral-org/alerting-system/internal/carrier"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// CarrierService implements the CarrierServiceServer interface.
type CarrierService struct {
	routingv1.UnimplementedCarrierServiceServer
	store      carrier.Store
	resolver   carrier.Resolver
	bgpHandler *carrier.BGPHandler
	logger     zerolog.Logger
}

// NewCarrierService creates a new CarrierService.
func NewCarrierService(store carrier.Store, logger zerolog.Logger) *CarrierService {
	return &CarrierService{
		store:      store,
		resolver:   carrier.NewResolver(store),
		bgpHandler: carrier.NewBGPHandler(store, logger),
		logger:     logger.With().Str("service", "carrier").Logger(),
	}
}

// =============================================================================
// Carrier CRUD (5 RPCs)
// =============================================================================

// CreateCarrier creates a new carrier.
func (s *CarrierService) CreateCarrier(ctx context.Context, req *routingv1.CreateCarrierRequest) (*routingv1.CarrierConfig, error) {
	if req.Carrier == nil {
		return nil, status.Error(codes.InvalidArgument, "carrier is required")
	}

	if req.Carrier.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "carrier name is required")
	}

	if req.Carrier.Asn == "" {
		return nil, status.Error(codes.InvalidArgument, "carrier ASN is required")
	}

	s.logger.Info().
		Str("name", req.Carrier.Name).
		Str("asn", req.Carrier.Asn).
		Msg("creating carrier")

	// Convert from proto to internal model
	c := protoToCarrier(req.Carrier)

	created, err := s.store.Create(ctx, c)
	if err != nil {
		if errors.Is(err, carrier.ErrDuplicateASN) {
			return nil, status.Error(codes.AlreadyExists, "carrier with this ASN already exists")
		}
		if errors.Is(err, carrier.ErrDuplicateName) {
			return nil, status.Error(codes.AlreadyExists, "carrier name already exists")
		}
		if errors.Is(err, carrier.ErrInvalidCarrier) {
			return nil, status.Errorf(codes.InvalidArgument, "invalid carrier: %v", err)
		}
		s.logger.Error().Err(err).Msg("failed to create carrier")
		return nil, status.Error(codes.Internal, "failed to create carrier")
	}

	s.logger.Info().
		Str("id", created.ID).
		Str("name", created.Name).
		Int("asn", created.ASN).
		Msg("carrier created")

	return carrierToProto(created), nil
}

// GetCarrier retrieves a carrier by ID.
func (s *CarrierService) GetCarrier(ctx context.Context, req *routingv1.GetCarrierRequest) (*routingv1.CarrierConfig, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	c, err := s.store.GetByID(ctx, req.Id)
	if err != nil {
		if errors.Is(err, carrier.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "carrier not found")
		}
		s.logger.Error().Err(err).Str("id", req.Id).Msg("failed to get carrier")
		return nil, status.Error(codes.Internal, "failed to get carrier")
	}

	return carrierToProto(c), nil
}

// GetCarrierByASN retrieves a carrier by ASN.
func (s *CarrierService) GetCarrierByASN(ctx context.Context, req *routingv1.GetCarrierByASNRequest) (*routingv1.CarrierConfig, error) {
	if req.Asn == "" {
		return nil, status.Error(codes.InvalidArgument, "ASN is required")
	}

	var asn int
	_, err := fmt.Sscanf(req.Asn, "%d", &asn)
	if err != nil || asn <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid ASN format")
	}

	c, err := s.store.GetByASN(ctx, asn)
	if err != nil {
		if errors.Is(err, carrier.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "carrier not found")
		}
		s.logger.Error().Err(err).Str("asn", req.Asn).Msg("failed to get carrier by ASN")
		return nil, status.Error(codes.Internal, "failed to get carrier by ASN")
	}

	return carrierToProto(c), nil
}

// ListCarriers retrieves carriers with optional filters.
func (s *CarrierService) ListCarriers(ctx context.Context, req *routingv1.ListCarriersRequest) (*routingv1.ListCarriersResponse, error) {
	filter := &carrier.CarrierFilter{
		PageSize:  int(req.PageSize),
		PageToken: req.PageToken,
	}

	carriers, err := s.store.List(ctx, filter)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to list carriers")
		return nil, status.Error(codes.Internal, "failed to list carriers")
	}

	resp := &routingv1.ListCarriersResponse{
		Carriers: make([]*routingv1.CarrierConfig, 0, len(carriers)),
	}

	for _, c := range carriers {
		resp.Carriers = append(resp.Carriers, carrierToProto(c))
	}

	return resp, nil
}

// UpdateCarrier updates an existing carrier.
func (s *CarrierService) UpdateCarrier(ctx context.Context, req *routingv1.UpdateCarrierRequest) (*routingv1.CarrierConfig, error) {
	if req.Carrier == nil || req.Carrier.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "carrier with id is required")
	}

	s.logger.Info().
		Str("id", req.Carrier.Id).
		Str("name", req.Carrier.Name).
		Msg("updating carrier")

	// Get existing carrier to preserve created_at
	existing, err := s.store.GetByID(ctx, req.Carrier.Id)
	if err != nil {
		if errors.Is(err, carrier.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "carrier not found")
		}
		s.logger.Error().Err(err).Str("id", req.Carrier.Id).Msg("failed to get carrier for update")
		return nil, status.Error(codes.Internal, "failed to update carrier")
	}

	// Convert from proto and preserve created_at
	c := protoToCarrier(req.Carrier)
	c.CreatedAt = existing.CreatedAt

	updated, err := s.store.Update(ctx, c)
	if err != nil {
		if errors.Is(err, carrier.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "carrier not found")
		}
		if errors.Is(err, carrier.ErrDuplicateASN) {
			return nil, status.Error(codes.AlreadyExists, "carrier with this ASN already exists")
		}
		if errors.Is(err, carrier.ErrDuplicateName) {
			return nil, status.Error(codes.AlreadyExists, "carrier name already exists")
		}
		s.logger.Error().Err(err).Str("id", req.Carrier.Id).Msg("failed to update carrier")
		return nil, status.Error(codes.Internal, "failed to update carrier")
	}

	s.logger.Info().
		Str("id", updated.ID).
		Msg("carrier updated")

	return carrierToProto(updated), nil
}

// DeleteCarrier deletes a carrier by ID.
func (s *CarrierService) DeleteCarrier(ctx context.Context, req *routingv1.DeleteCarrierRequest) (*routingv1.DeleteCarrierResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	s.logger.Info().Str("id", req.Id).Msg("deleting carrier")

	err := s.store.Delete(ctx, req.Id)
	if err != nil {
		if errors.Is(err, carrier.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "carrier not found")
		}
		s.logger.Error().Err(err).Str("id", req.Id).Msg("failed to delete carrier")
		return nil, status.Error(codes.Internal, "failed to delete carrier")
	}

	s.logger.Info().Str("id", req.Id).Msg("carrier deleted")

	return &routingv1.DeleteCarrierResponse{Success: true}, nil
}

// =============================================================================
// Helper functions for proto conversion
// =============================================================================

// protoToCarrier converts a protobuf CarrierConfig to internal Carrier model.
func protoToCarrier(pb *routingv1.CarrierConfig) *carrier.Carrier {
	if pb == nil {
		return nil
	}

	var asn int
	_, _ = fmt.Sscanf(pb.Asn, "%d", &asn)

	return &carrier.Carrier{
		ID:               pb.Id,
		Name:             pb.Name,
		ASN:              asn,
		NOCEmail:         pb.NocEmail,
		NOCPhone:         pb.NocPhone,
		NOCPortalURL:     pb.NocPortalUrl,
		TeamID:           pb.TeamId,
		AutoTicket:       pb.AutoTicket,
		TicketProviderID: pb.TicketProviderId,
		// Default type if not specified
		Type:     carrier.CarrierTypePeering,
		Priority: 5,
	}
}

// carrierToProto converts an internal Carrier model to protobuf CarrierConfig.
func carrierToProto(c *carrier.Carrier) *routingv1.CarrierConfig {
	if c == nil {
		return nil
	}

	return &routingv1.CarrierConfig{
		Id:               c.ID,
		Name:             c.Name,
		Asn:              fmt.Sprintf("%d", c.ASN),
		NocEmail:         c.NOCEmail,
		NocPhone:         c.NOCPhone,
		NocPortalUrl:     c.NOCPortalURL,
		TeamId:           c.TeamID,
		AutoTicket:       c.AutoTicket,
		TicketProviderId: c.TicketProviderID,
	}
}

// Ensure CarrierService implements the interface
var _ routingv1.CarrierServiceServer = (*CarrierService)(nil)
