package grpc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kneutral-org/alerting-system/internal/escalation"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// EscalationServiceServer implements the gRPC EscalationService.
type EscalationServiceServer struct {
	routingv1.UnimplementedEscalationServiceServer
	policyStore escalation.PolicyStore
	activeStore escalation.ActiveEscalationStore
}

// NewEscalationServiceServer creates a new EscalationServiceServer.
func NewEscalationServiceServer(policyStore escalation.PolicyStore, activeStore escalation.ActiveEscalationStore) *EscalationServiceServer {
	return &EscalationServiceServer{
		policyStore: policyStore,
		activeStore: activeStore,
	}
}

// CreateEscalationPolicy creates a new escalation policy.
func (s *EscalationServiceServer) CreateEscalationPolicy(ctx context.Context, req *routingv1.CreateEscalationPolicyRequest) (*routingv1.EscalationPolicy, error) {
	if req.GetPolicy() == nil {
		return nil, status.Error(codes.InvalidArgument, "policy is required")
	}

	protoPolicy := req.GetPolicy()
	if protoPolicy.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "policy name is required")
	}

	internalPolicy := protoEscalationPolicyToInternal(protoPolicy)

	created, err := s.policyStore.CreatePolicy(ctx, internalPolicy)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create escalation policy: %v", err)
	}

	return internalEscalationPolicyToProto(created), nil
}

// GetEscalationPolicy retrieves an escalation policy by ID.
func (s *EscalationServiceServer) GetEscalationPolicy(ctx context.Context, req *routingv1.GetEscalationPolicyRequest) (*routingv1.EscalationPolicy, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "policy id is required")
	}

	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid policy id: %v", err)
	}

	policy, err := s.policyStore.GetPolicy(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get escalation policy: %v", err)
	}
	if policy == nil {
		return nil, status.Error(codes.NotFound, "escalation policy not found")
	}

	return internalEscalationPolicyToProto(policy), nil
}

// ListEscalationPolicies lists escalation policies with optional pagination.
func (s *EscalationServiceServer) ListEscalationPolicies(ctx context.Context, req *routingv1.ListEscalationPoliciesRequest) (*routingv1.ListEscalationPoliciesResponse, error) {
	params := escalation.ListPoliciesParams{
		Limit: req.GetPageSize(),
	}

	if params.Limit <= 0 {
		params.Limit = 100
	}

	policies, err := s.policyStore.ListPolicies(ctx, params)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list escalation policies: %v", err)
	}

	protoPolicies := make([]*routingv1.EscalationPolicy, 0, len(policies))
	for _, p := range policies {
		protoPolicies = append(protoPolicies, internalEscalationPolicyToProto(p))
	}

	return &routingv1.ListEscalationPoliciesResponse{
		Policies:   protoPolicies,
		TotalCount: int32(len(protoPolicies)),
	}, nil
}

// UpdateEscalationPolicy updates an existing escalation policy.
func (s *EscalationServiceServer) UpdateEscalationPolicy(ctx context.Context, req *routingv1.UpdateEscalationPolicyRequest) (*routingv1.EscalationPolicy, error) {
	if req.GetPolicy() == nil {
		return nil, status.Error(codes.InvalidArgument, "policy is required")
	}

	protoPolicy := req.GetPolicy()
	if protoPolicy.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "policy id is required")
	}

	id, err := uuid.Parse(protoPolicy.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid policy id: %v", err)
	}

	// Check if policy exists
	existing, err := s.policyStore.GetPolicy(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get escalation policy: %v", err)
	}
	if existing == nil {
		return nil, status.Error(codes.NotFound, "escalation policy not found")
	}

	internalPolicy := protoEscalationPolicyToInternal(protoPolicy)
	internalPolicy.ID = id

	updated, err := s.policyStore.UpdatePolicy(ctx, internalPolicy)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to update escalation policy: %v", err)
	}
	if updated == nil {
		return nil, status.Error(codes.NotFound, "escalation policy not found")
	}

	return internalEscalationPolicyToProto(updated), nil
}

// DeleteEscalationPolicy deletes an escalation policy.
func (s *EscalationServiceServer) DeleteEscalationPolicy(ctx context.Context, req *routingv1.DeleteEscalationPolicyRequest) (*routingv1.DeleteEscalationPolicyResponse, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "policy id is required")
	}

	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid policy id: %v", err)
	}

	err = s.policyStore.DeletePolicy(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete escalation policy: %v", err)
	}

	return &routingv1.DeleteEscalationPolicyResponse{Success: true}, nil
}

// StartEscalation starts a new escalation for an alert.
// For MVP, this returns Unimplemented.
func (s *EscalationServiceServer) StartEscalation(ctx context.Context, req *routingv1.StartEscalationRequest) (*routingv1.StartEscalationResponse, error) {
	return nil, status.Error(codes.Unimplemented, "StartEscalation not implemented in MVP")
}

// GetEscalationStatus gets the status of an active escalation.
// For MVP, this returns Unimplemented.
func (s *EscalationServiceServer) GetEscalationStatus(ctx context.Context, req *routingv1.GetEscalationStatusRequest) (*routingv1.EscalationStatus, error) {
	return nil, status.Error(codes.Unimplemented, "GetEscalationStatus not implemented in MVP")
}

// StopEscalation stops an active escalation.
// For MVP, this returns Unimplemented.
func (s *EscalationServiceServer) StopEscalation(ctx context.Context, req *routingv1.StopEscalationRequest) (*routingv1.StopEscalationResponse, error) {
	return nil, status.Error(codes.Unimplemented, "StopEscalation not implemented in MVP")
}

// Conversion helpers

func protoEscalationPolicyToInternal(p *routingv1.EscalationPolicy) *escalation.EscalationPolicy {
	policy := &escalation.EscalationPolicy{
		Name:        p.GetName(),
		Description: p.GetDescription(),
		RepeatCount: p.GetRepeatCount(),
	}

	if p.GetId() != "" {
		if id, err := uuid.Parse(p.GetId()); err == nil {
			policy.ID = id
		}
	}

	// Convert steps
	for _, step := range p.GetSteps() {
		internalStep := escalation.EscalationStep{
			StepNumber:       step.GetStepNumber(),
			SkipConditionCel: step.GetSkipConditionCel(),
		}

		// Convert delay from Duration to seconds
		if step.GetDelay() != nil {
			internalStep.DelaySeconds = int32(step.GetDelay().GetSeconds())
		}

		// Convert targets
		for _, target := range step.GetTargets() {
			targetID := ""
			switch target.GetType() {
			case routingv1.EscalationTargetType_ESCALATION_TARGET_TYPE_USER:
				targetID = target.GetUserId()
			case routingv1.EscalationTargetType_ESCALATION_TARGET_TYPE_TEAM:
				targetID = target.GetTeamId()
			case routingv1.EscalationTargetType_ESCALATION_TARGET_TYPE_SCHEDULE:
				targetID = target.GetScheduleId()
			}
			internalStep.Targets = append(internalStep.Targets, escalation.EscalationTarget{
				Type:     target.GetType().String(),
				TargetID: targetID,
			})
		}

		policy.Steps = append(policy.Steps, internalStep)
	}

	// Convert exhausted action
	if p.GetExhaustedAction() != nil {
		policy.ExhaustedAction = &escalation.ExhaustedAction{
			Action: p.GetExhaustedAction().GetType().String(),
		}
	}

	return policy
}

func internalEscalationPolicyToProto(p *escalation.EscalationPolicy) *routingv1.EscalationPolicy {
	policy := &routingv1.EscalationPolicy{
		Id:          p.ID.String(),
		Name:        p.Name,
		Description: p.Description,
		RepeatCount: p.RepeatCount,
		CreatedAt:   timestamppb.New(p.CreatedAt),
		UpdatedAt:   timestamppb.New(p.UpdatedAt),
	}

	// Convert steps
	for _, step := range p.Steps {
		protoStep := &routingv1.EscalationStep{
			StepNumber:       step.StepNumber,
			Delay:            durationpb.New(time.Duration(step.DelaySeconds) * time.Second),
			SkipConditionCel: step.SkipConditionCel,
		}

		// Convert targets
		for _, target := range step.Targets {
			protoTarget := &routingv1.EscalationTarget{
				Type: parseEscalationTargetType(target.Type),
			}
			// Set the appropriate target ID field based on type
			switch protoTarget.Type {
			case routingv1.EscalationTargetType_ESCALATION_TARGET_TYPE_USER:
				protoTarget.UserId = target.TargetID
			case routingv1.EscalationTargetType_ESCALATION_TARGET_TYPE_TEAM:
				protoTarget.TeamId = target.TargetID
			case routingv1.EscalationTargetType_ESCALATION_TARGET_TYPE_SCHEDULE:
				protoTarget.ScheduleId = target.TargetID
			}
			protoStep.Targets = append(protoStep.Targets, protoTarget)
		}

		policy.Steps = append(policy.Steps, protoStep)
	}

	// Convert exhausted action
	if p.ExhaustedAction != nil {
		policy.ExhaustedAction = &routingv1.EscalationExhaustedAction{
			Type: parseExhaustedActionType(p.ExhaustedAction.Action),
		}
	}

	return policy
}

func parseEscalationTargetType(s string) routingv1.EscalationTargetType {
	switch s {
	case "ESCALATION_TARGET_TYPE_USER":
		return routingv1.EscalationTargetType_ESCALATION_TARGET_TYPE_USER
	case "ESCALATION_TARGET_TYPE_TEAM":
		return routingv1.EscalationTargetType_ESCALATION_TARGET_TYPE_TEAM
	case "ESCALATION_TARGET_TYPE_SCHEDULE":
		return routingv1.EscalationTargetType_ESCALATION_TARGET_TYPE_SCHEDULE
	case "ESCALATION_TARGET_TYPE_CHANNEL":
		return routingv1.EscalationTargetType_ESCALATION_TARGET_TYPE_CHANNEL
	default:
		return routingv1.EscalationTargetType_ESCALATION_TARGET_TYPE_UNSPECIFIED
	}
}

func parseExhaustedActionType(s string) routingv1.ExhaustedActionType {
	switch s {
	case "EXHAUSTED_ACTION_TYPE_STOP":
		return routingv1.ExhaustedActionType_EXHAUSTED_ACTION_TYPE_STOP
	case "EXHAUSTED_ACTION_TYPE_REPEAT":
		return routingv1.ExhaustedActionType_EXHAUSTED_ACTION_TYPE_REPEAT
	case "EXHAUSTED_ACTION_TYPE_NOTIFY_FALLBACK":
		return routingv1.ExhaustedActionType_EXHAUSTED_ACTION_TYPE_NOTIFY_FALLBACK
	case "EXHAUSTED_ACTION_TYPE_CREATE_INCIDENT":
		return routingv1.ExhaustedActionType_EXHAUSTED_ACTION_TYPE_CREATE_INCIDENT
	default:
		return routingv1.ExhaustedActionType_EXHAUSTED_ACTION_TYPE_UNSPECIFIED
	}
}
