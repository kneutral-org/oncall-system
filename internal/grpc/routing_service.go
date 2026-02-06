// Package grpc provides gRPC service implementations.
package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kneutral-org/alerting-system/internal/routing"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// RoutingService implements the RoutingServiceServer interface.
type RoutingService struct {
	routingv1.UnimplementedRoutingServiceServer
	store     routing.Store
	evaluator *routing.Evaluator
	logger    zerolog.Logger
}

// NewRoutingService creates a new RoutingService.
func NewRoutingService(store routing.Store, logger zerolog.Logger) *RoutingService {
	return &RoutingService{
		store:     store,
		evaluator: routing.NewEvaluator(),
		logger:    logger.With().Str("service", "routing").Logger(),
	}
}

// CreateRoutingRule creates a new routing rule.
func (s *RoutingService) CreateRoutingRule(ctx context.Context, req *routingv1.CreateRoutingRuleRequest) (*routingv1.RoutingRule, error) {
	if req.Rule == nil {
		return nil, status.Error(codes.InvalidArgument, "rule is required")
	}

	if req.Rule.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "rule name is required")
	}

	s.logger.Info().
		Str("name", req.Rule.Name).
		Int32("priority", req.Rule.Priority).
		Msg("creating routing rule")

	rule, err := s.store.CreateRule(ctx, req.Rule)
	if err != nil {
		if errors.Is(err, routing.ErrDuplicatePriority) {
			return nil, status.Error(codes.AlreadyExists, "priority already exists")
		}
		s.logger.Error().Err(err).Msg("failed to create routing rule")
		return nil, status.Error(codes.Internal, "failed to create routing rule")
	}

	s.logger.Info().
		Str("id", rule.Id).
		Str("name", rule.Name).
		Msg("routing rule created")

	return rule, nil
}

// GetRoutingRule retrieves a routing rule by ID.
func (s *RoutingService) GetRoutingRule(ctx context.Context, req *routingv1.GetRoutingRuleRequest) (*routingv1.RoutingRule, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	rule, err := s.store.GetRule(ctx, req.Id)
	if err != nil {
		if errors.Is(err, routing.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "routing rule not found")
		}
		s.logger.Error().Err(err).Str("id", req.Id).Msg("failed to get routing rule")
		return nil, status.Error(codes.Internal, "failed to get routing rule")
	}

	return rule, nil
}

// ListRoutingRules retrieves routing rules with optional filters.
func (s *RoutingService) ListRoutingRules(ctx context.Context, req *routingv1.ListRoutingRulesRequest) (*routingv1.ListRoutingRulesResponse, error) {
	resp, err := s.store.ListRules(ctx, req)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to list routing rules")
		return nil, status.Error(codes.Internal, "failed to list routing rules")
	}

	return resp, nil
}

// UpdateRoutingRule updates an existing routing rule.
func (s *RoutingService) UpdateRoutingRule(ctx context.Context, req *routingv1.UpdateRoutingRuleRequest) (*routingv1.RoutingRule, error) {
	if req.Rule == nil || req.Rule.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "rule with id is required")
	}

	s.logger.Info().
		Str("id", req.Rule.Id).
		Str("name", req.Rule.Name).
		Msg("updating routing rule")

	rule, err := s.store.UpdateRule(ctx, req.Rule)
	if err != nil {
		if errors.Is(err, routing.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "routing rule not found")
		}
		if errors.Is(err, routing.ErrDuplicatePriority) {
			return nil, status.Error(codes.AlreadyExists, "priority already exists")
		}
		s.logger.Error().Err(err).Str("id", req.Rule.Id).Msg("failed to update routing rule")
		return nil, status.Error(codes.Internal, "failed to update routing rule")
	}

	s.logger.Info().
		Str("id", rule.Id).
		Msg("routing rule updated")

	return rule, nil
}

// DeleteRoutingRule deletes a routing rule by ID.
func (s *RoutingService) DeleteRoutingRule(ctx context.Context, req *routingv1.DeleteRoutingRuleRequest) (*routingv1.DeleteRoutingRuleResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	s.logger.Info().Str("id", req.Id).Msg("deleting routing rule")

	err := s.store.DeleteRule(ctx, req.Id)
	if err != nil {
		if errors.Is(err, routing.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "routing rule not found")
		}
		s.logger.Error().Err(err).Str("id", req.Id).Msg("failed to delete routing rule")
		return nil, status.Error(codes.Internal, "failed to delete routing rule")
	}

	s.logger.Info().Str("id", req.Id).Msg("routing rule deleted")

	return &routingv1.DeleteRoutingRuleResponse{Success: true}, nil
}

// ReorderRoutingRules updates the priorities of multiple rules.
func (s *RoutingService) ReorderRoutingRules(ctx context.Context, req *routingv1.ReorderRoutingRulesRequest) (*routingv1.ReorderRoutingRulesResponse, error) {
	if len(req.RulePriorities) == 0 {
		return nil, status.Error(codes.InvalidArgument, "rule priorities are required")
	}

	s.logger.Info().
		Int("count", len(req.RulePriorities)).
		Msg("reordering routing rules")

	rules, err := s.store.ReorderRules(ctx, req.RulePriorities)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to reorder routing rules")
		return nil, status.Error(codes.Internal, "failed to reorder routing rules")
	}

	return &routingv1.ReorderRoutingRulesResponse{UpdatedRules: rules}, nil
}

// TestRoutingRule tests a routing rule against a sample alert (dry-run).
func (s *RoutingService) TestRoutingRule(ctx context.Context, req *routingv1.TestRoutingRuleRequest) (*routingv1.TestRoutingRuleResponse, error) {
	if req.Rule == nil {
		return nil, status.Error(codes.InvalidArgument, "rule is required")
	}
	if req.SampleAlert == nil {
		return nil, status.Error(codes.InvalidArgument, "sample alert is required")
	}

	// Use simulate time if provided, otherwise use current time
	evalTime := time.Now()
	if req.SimulateTime != nil {
		evalTime = req.SimulateTime.AsTime()
	}

	s.logger.Debug().
		Str("rule_name", req.Rule.Name).
		Str("alert_id", req.SampleAlert.Id).
		Time("eval_time", evalTime).
		Msg("testing routing rule")

	// Evaluate the rule
	eval := s.evaluator.EvaluateRule(req.Rule, req.SampleAlert, evalTime)

	resp := &routingv1.TestRoutingRuleResponse{
		Matched:              eval.Matched,
		ConditionResults:     eval.ConditionResults,
		TimeConditionMatched: eval.TimeConditionMatched,
		TimeConditionReason:  eval.TimeConditionReason,
	}

	if eval.Matched {
		resp.MatchedActions = req.Rule.Actions
	}

	return resp, nil
}

// SimulateRouting simulates the full routing pipeline for an alert.
func (s *RoutingService) SimulateRouting(ctx context.Context, req *routingv1.SimulateRoutingRequest) (*routingv1.SimulateRoutingResponse, error) {
	if req.Alert == nil {
		return nil, status.Error(codes.InvalidArgument, "alert is required")
	}

	// Use simulate time if provided, otherwise use current time
	evalTime := time.Now()
	if req.SimulateTime != nil {
		evalTime = req.SimulateTime.AsTime()
	}

	s.logger.Debug().
		Str("alert_id", req.Alert.Id).
		Time("eval_time", evalTime).
		Bool("include_disabled", req.IncludeDisabled).
		Msg("simulating routing")

	// Get all rules
	var rules []*routingv1.RoutingRule
	var err error

	if req.IncludeDisabled {
		listResp, err := s.store.ListRules(ctx, &routingv1.ListRoutingRulesRequest{})
		if err != nil {
			s.logger.Error().Err(err).Msg("failed to list rules for simulation")
			return nil, status.Error(codes.Internal, "failed to list rules")
		}
		rules = listResp.Rules
	} else {
		rules, err = s.store.GetEnabledRulesByPriority(ctx)
		if err != nil {
			s.logger.Error().Err(err).Msg("failed to get enabled rules for simulation")
			return nil, status.Error(codes.Internal, "failed to get enabled rules")
		}
	}

	// Evaluate all rules
	evaluations, matchedActions := s.evaluator.EvaluateRules(rules, req.Alert, evalTime)

	// Convert actions to ActionExecution for simulation
	var actionExecs []*routingv1.ActionExecution
	for _, action := range matchedActions {
		exec := &routingv1.ActionExecution{
			ActionType:   action.Type,
			Success:      true, // Simulation assumes success
			ErrorMessage: "",
		}
		actionExecs = append(actionExecs, exec)
	}

	resp := &routingv1.SimulateRoutingResponse{
		Evaluations:       evaluations,
		Actions:           actionExecs,
		MaintenanceResult: nil, // TODO: Check maintenance windows
		Warnings:          []string{},
	}

	// Add warnings for potential issues
	if len(rules) == 0 {
		resp.Warnings = append(resp.Warnings, "no routing rules defined")
	}

	matchedCount := 0
	for _, eval := range evaluations {
		if eval.Matched {
			matchedCount++
		}
	}

	if matchedCount == 0 {
		resp.Warnings = append(resp.Warnings, "no rules matched the alert")
	}

	return resp, nil
}

// GetRoutingAuditLogs retrieves routing audit logs.
func (s *RoutingService) GetRoutingAuditLogs(ctx context.Context, req *routingv1.GetRoutingAuditLogsRequest) (*routingv1.GetRoutingAuditLogsResponse, error) {
	resp, err := s.store.GetAuditLogs(ctx, req)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to get routing audit logs")
		return nil, status.Error(codes.Internal, "failed to get routing audit logs")
	}

	return resp, nil
}

// RouteAlert executes routing for an alert (internal use by alert engine).
func (s *RoutingService) RouteAlert(ctx context.Context, req *routingv1.RouteAlertRequest) (*routingv1.RouteAlertResponse, error) {
	if req.Alert == nil {
		return nil, status.Error(codes.InvalidArgument, "alert is required")
	}

	startTime := time.Now()

	s.logger.Info().
		Str("alert_id", req.Alert.Id).
		Str("fingerprint", req.Alert.Fingerprint).
		Msg("routing alert")

	// Get enabled rules
	rules, err := s.store.GetEnabledRulesByPriority(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to get enabled rules")
		return nil, status.Error(codes.Internal, "failed to get enabled rules")
	}

	// Evaluate rules
	evalTime := time.Now()
	evaluations, matchedActions := s.evaluator.EvaluateRules(rules, req.Alert, evalTime)

	// Create audit log
	auditLog := &routingv1.RoutingAuditLog{
		AlertId:     req.Alert.Id,
		Timestamp:   timestamppb.New(evalTime),
		Evaluations: evaluations,
		Executions:  make([]*routingv1.ActionExecution, 0),
	}

	// Create alert snapshot
	alertSnapshot, err := structpb.NewStruct(map[string]interface{}{
		"id":          req.Alert.Id,
		"summary":     req.Alert.Summary,
		"status":      req.Alert.Status.String(),
		"source":      req.Alert.Source.String(),
		"fingerprint": req.Alert.Fingerprint,
		"labels":      req.Alert.Labels,
		"annotations": req.Alert.Annotations,
	})
	if err == nil {
		auditLog.AlertSnapshot = alertSnapshot
	}

	resp := &routingv1.RouteAlertResponse{
		AuditLog:        auditLog,
		NotificationIds: []string{},
	}

	// Process matched actions
	for _, action := range matchedActions {
		exec := &routingv1.ActionExecution{
			ActionType:   action.Type,
			ExecutedAt:   timestamppb.Now(),
			Success:      false,
			ErrorMessage: "",
		}

		// Execute the action based on type
		switch action.Type {
		case routingv1.ActionType_ACTION_TYPE_SUPPRESS:
			exec.Success = true
			resp.Suppressed = true
			if action.Suppress != nil {
				resp.SuppressionReason = action.Suppress.Reason
			}

		case routingv1.ActionType_ACTION_TYPE_ESCALATE:
			if action.Escalate != nil {
				resp.EscalationStarted = true
				resp.EscalationId = action.Escalate.EscalationPolicyId
				exec.Success = true
			}

		case routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM,
			routingv1.ActionType_ACTION_TYPE_NOTIFY_CHANNEL,
			routingv1.ActionType_ACTION_TYPE_NOTIFY_USER,
			routingv1.ActionType_ACTION_TYPE_NOTIFY_ONCALL,
			routingv1.ActionType_ACTION_TYPE_NOTIFY_WEBHOOK:
			// TODO: Actually send notifications via notification service
			// For now, just mark as successful
			exec.Success = true
			exec.NotificationIds = []string{} // Would be populated after sending

		case routingv1.ActionType_ACTION_TYPE_CREATE_TICKET:
			// TODO: Create ticket via ticket integration
			exec.Success = true

		case routingv1.ActionType_ACTION_TYPE_SET_LABEL:
			// TODO: Update alert labels
			exec.Success = true

		default:
			exec.Success = false
			exec.ErrorMessage = "unknown action type"
		}

		auditLog.Executions = append(auditLog.Executions, exec)

		// Collect notification IDs
		if exec.Success && len(exec.NotificationIds) > 0 {
			resp.NotificationIds = append(resp.NotificationIds, exec.NotificationIds...)
		}
	}

	// Save audit log
	if err := s.store.CreateAuditLog(ctx, auditLog); err != nil {
		s.logger.Warn().Err(err).Msg("failed to save routing audit log")
		// Don't fail the request, just log the error
	}

	processingTime := time.Since(startTime)
	s.logger.Info().
		Str("alert_id", req.Alert.Id).
		Int("rules_evaluated", len(evaluations)).
		Int("actions_executed", len(auditLog.Executions)).
		Bool("suppressed", resp.Suppressed).
		Bool("escalation_started", resp.EscalationStarted).
		Dur("processing_time", processingTime).
		Msg("alert routed")

	return resp, nil
}

// Ensure RoutingService implements the interface
var _ routingv1.RoutingServiceServer = (*RoutingService)(nil)
