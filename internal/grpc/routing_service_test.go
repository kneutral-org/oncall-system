package grpc

import (
	"context"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/kneutral-org/alerting-system/internal/routing"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

func newTestService() *RoutingService {
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	store := routing.NewInMemoryStore()
	return NewRoutingService(store, logger)
}

func TestRoutingService_CreateRoutingRule(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	rule, err := svc.CreateRoutingRule(ctx, &routingv1.CreateRoutingRuleRequest{
		Rule: &routingv1.RoutingRule{
			Name:        "Test Rule",
			Description: "A test rule",
			Priority:    1,
			Enabled:     true,
			Conditions: []*routingv1.RoutingCondition{
				{
					Type:        routingv1.ConditionType_CONDITION_TYPE_LABEL,
					Field:       "severity",
					Operator:    routingv1.ConditionOperator_CONDITION_OPERATOR_EQUALS,
					StringValue: "critical",
				},
			},
			Actions: []*routingv1.RoutingAction{
				{Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM},
			},
		},
	})

	if err != nil {
		t.Fatalf("CreateRoutingRule() error = %v", err)
	}

	if rule.Id == "" {
		t.Error("CreateRoutingRule() should return rule with ID")
	}

	if rule.Name != "Test Rule" {
		t.Errorf("CreateRoutingRule() name = %q, want %q", rule.Name, "Test Rule")
	}
}

func TestRoutingService_CreateRoutingRule_NilRule(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.CreateRoutingRule(ctx, &routingv1.CreateRoutingRuleRequest{Rule: nil})

	if err == nil {
		t.Fatal("CreateRoutingRule() should error for nil rule")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got %v", err)
	}

	if st.Code() != codes.InvalidArgument {
		t.Errorf("CreateRoutingRule() code = %v, want %v", st.Code(), codes.InvalidArgument)
	}
}

func TestRoutingService_CreateRoutingRule_EmptyName(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.CreateRoutingRule(ctx, &routingv1.CreateRoutingRuleRequest{
		Rule: &routingv1.RoutingRule{
			Name:     "",
			Priority: 1,
		},
	})

	if err == nil {
		t.Fatal("CreateRoutingRule() should error for empty name")
	}

	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("CreateRoutingRule() code = %v, want %v", st.Code(), codes.InvalidArgument)
	}
}

func TestRoutingService_GetRoutingRule(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	// Create a rule first
	created, _ := svc.CreateRoutingRule(ctx, &routingv1.CreateRoutingRuleRequest{
		Rule: &routingv1.RoutingRule{
			Name:     "Test Rule",
			Priority: 1,
			Enabled:  true,
		},
	})

	// Get the rule
	got, err := svc.GetRoutingRule(ctx, &routingv1.GetRoutingRuleRequest{Id: created.Id})
	if err != nil {
		t.Fatalf("GetRoutingRule() error = %v", err)
	}

	if got.Id != created.Id {
		t.Errorf("GetRoutingRule() id = %q, want %q", got.Id, created.Id)
	}
}

func TestRoutingService_GetRoutingRule_NotFound(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.GetRoutingRule(ctx, &routingv1.GetRoutingRuleRequest{Id: "nonexistent"})

	if err == nil {
		t.Fatal("GetRoutingRule() should error for nonexistent rule")
	}

	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Errorf("GetRoutingRule() code = %v, want %v", st.Code(), codes.NotFound)
	}
}

func TestRoutingService_GetRoutingRule_EmptyId(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.GetRoutingRule(ctx, &routingv1.GetRoutingRuleRequest{Id: ""})

	if err == nil {
		t.Fatal("GetRoutingRule() should error for empty id")
	}

	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("GetRoutingRule() code = %v, want %v", st.Code(), codes.InvalidArgument)
	}
}

func TestRoutingService_ListRoutingRules(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	// Create some rules
	for i := 1; i <= 3; i++ {
		_, _ = svc.CreateRoutingRule(ctx, &routingv1.CreateRoutingRuleRequest{
			Rule: &routingv1.RoutingRule{
				Name:     "Rule " + string(rune('A'-1+i)),
				Priority: int32(i),
				Enabled:  true,
			},
		})
	}

	resp, err := svc.ListRoutingRules(ctx, &routingv1.ListRoutingRulesRequest{})
	if err != nil {
		t.Fatalf("ListRoutingRules() error = %v", err)
	}

	if len(resp.Rules) != 3 {
		t.Errorf("ListRoutingRules() count = %d, want 3", len(resp.Rules))
	}
}

func TestRoutingService_UpdateRoutingRule(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	// Create a rule
	created, _ := svc.CreateRoutingRule(ctx, &routingv1.CreateRoutingRuleRequest{
		Rule: &routingv1.RoutingRule{
			Name:     "Original Name",
			Priority: 1,
			Enabled:  true,
		},
	})

	// Update the rule
	created.Name = "Updated Name"
	created.Enabled = false

	updated, err := svc.UpdateRoutingRule(ctx, &routingv1.UpdateRoutingRuleRequest{Rule: created})
	if err != nil {
		t.Fatalf("UpdateRoutingRule() error = %v", err)
	}

	if updated.Name != "Updated Name" {
		t.Errorf("UpdateRoutingRule() name = %q, want %q", updated.Name, "Updated Name")
	}

	if updated.Enabled != false {
		t.Error("UpdateRoutingRule() enabled should be false")
	}
}

func TestRoutingService_UpdateRoutingRule_NotFound(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.UpdateRoutingRule(ctx, &routingv1.UpdateRoutingRuleRequest{
		Rule: &routingv1.RoutingRule{
			Id:       "nonexistent",
			Name:     "Test",
			Priority: 1,
		},
	})

	if err == nil {
		t.Fatal("UpdateRoutingRule() should error for nonexistent rule")
	}

	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Errorf("UpdateRoutingRule() code = %v, want %v", st.Code(), codes.NotFound)
	}
}

func TestRoutingService_DeleteRoutingRule(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	// Create a rule
	created, _ := svc.CreateRoutingRule(ctx, &routingv1.CreateRoutingRuleRequest{
		Rule: &routingv1.RoutingRule{
			Name:     "To Delete",
			Priority: 1,
			Enabled:  true,
		},
	})

	// Delete the rule
	resp, err := svc.DeleteRoutingRule(ctx, &routingv1.DeleteRoutingRuleRequest{Id: created.Id})
	if err != nil {
		t.Fatalf("DeleteRoutingRule() error = %v", err)
	}

	if !resp.Success {
		t.Error("DeleteRoutingRule() success should be true")
	}

	// Verify deletion
	_, err = svc.GetRoutingRule(ctx, &routingv1.GetRoutingRuleRequest{Id: created.Id})
	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Error("DeleteRoutingRule() rule should be deleted")
	}
}

func TestRoutingService_DeleteRoutingRule_NotFound(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.DeleteRoutingRule(ctx, &routingv1.DeleteRoutingRuleRequest{Id: "nonexistent"})

	if err == nil {
		t.Fatal("DeleteRoutingRule() should error for nonexistent rule")
	}

	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Errorf("DeleteRoutingRule() code = %v, want %v", st.Code(), codes.NotFound)
	}
}

func TestRoutingService_ReorderRoutingRules(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	// Create rules
	rule1, _ := svc.CreateRoutingRule(ctx, &routingv1.CreateRoutingRuleRequest{
		Rule: &routingv1.RoutingRule{Name: "Rule A", Priority: 1, Enabled: true},
	})
	rule2, _ := svc.CreateRoutingRule(ctx, &routingv1.CreateRoutingRuleRequest{
		Rule: &routingv1.RoutingRule{Name: "Rule B", Priority: 2, Enabled: true},
	})

	// Reorder
	resp, err := svc.ReorderRoutingRules(ctx, &routingv1.ReorderRoutingRulesRequest{
		RulePriorities: map[string]int32{
			rule1.Id: 10,
			rule2.Id: 5,
		},
	})

	if err != nil {
		t.Fatalf("ReorderRoutingRules() error = %v", err)
	}

	if len(resp.UpdatedRules) != 2 {
		t.Errorf("ReorderRoutingRules() count = %d, want 2", len(resp.UpdatedRules))
	}
}

func TestRoutingService_ReorderRoutingRules_Empty(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.ReorderRoutingRules(ctx, &routingv1.ReorderRoutingRulesRequest{
		RulePriorities: map[string]int32{},
	})

	if err == nil {
		t.Fatal("ReorderRoutingRules() should error for empty priorities")
	}

	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("ReorderRoutingRules() code = %v, want %v", st.Code(), codes.InvalidArgument)
	}
}

func TestRoutingService_TestRoutingRule(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	resp, err := svc.TestRoutingRule(ctx, &routingv1.TestRoutingRuleRequest{
		Rule: &routingv1.RoutingRule{
			Name:     "Test Rule",
			Priority: 1,
			Enabled:  true,
			Conditions: []*routingv1.RoutingCondition{
				{
					Type:        routingv1.ConditionType_CONDITION_TYPE_LABEL,
					Field:       "severity",
					Operator:    routingv1.ConditionOperator_CONDITION_OPERATOR_EQUALS,
					StringValue: "critical",
				},
			},
			Actions: []*routingv1.RoutingAction{
				{Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM},
			},
		},
		SampleAlert: &routingv1.Alert{
			Id:      "alert-1",
			Summary: "Test Alert",
			Labels:  map[string]string{"severity": "critical"},
		},
	})

	if err != nil {
		t.Fatalf("TestRoutingRule() error = %v", err)
	}

	if !resp.Matched {
		t.Error("TestRoutingRule() should match")
	}

	if len(resp.MatchedActions) != 1 {
		t.Errorf("TestRoutingRule() actions = %d, want 1", len(resp.MatchedActions))
	}
}

func TestRoutingService_TestRoutingRule_NoMatch(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	resp, err := svc.TestRoutingRule(ctx, &routingv1.TestRoutingRuleRequest{
		Rule: &routingv1.RoutingRule{
			Name:     "Test Rule",
			Priority: 1,
			Enabled:  true,
			Conditions: []*routingv1.RoutingCondition{
				{
					Type:        routingv1.ConditionType_CONDITION_TYPE_LABEL,
					Field:       "severity",
					Operator:    routingv1.ConditionOperator_CONDITION_OPERATOR_EQUALS,
					StringValue: "critical",
				},
			},
		},
		SampleAlert: &routingv1.Alert{
			Id:      "alert-1",
			Summary: "Test Alert",
			Labels:  map[string]string{"severity": "warning"},
		},
	})

	if err != nil {
		t.Fatalf("TestRoutingRule() error = %v", err)
	}

	if resp.Matched {
		t.Error("TestRoutingRule() should not match")
	}
}

func TestRoutingService_TestRoutingRule_NilRule(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.TestRoutingRule(ctx, &routingv1.TestRoutingRuleRequest{
		Rule:        nil,
		SampleAlert: &routingv1.Alert{Id: "alert-1"},
	})

	if err == nil {
		t.Fatal("TestRoutingRule() should error for nil rule")
	}

	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("TestRoutingRule() code = %v, want %v", st.Code(), codes.InvalidArgument)
	}
}

func TestRoutingService_TestRoutingRule_NilAlert(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.TestRoutingRule(ctx, &routingv1.TestRoutingRuleRequest{
		Rule:        &routingv1.RoutingRule{Name: "Test", Priority: 1},
		SampleAlert: nil,
	})

	if err == nil {
		t.Fatal("TestRoutingRule() should error for nil alert")
	}

	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("TestRoutingRule() code = %v, want %v", st.Code(), codes.InvalidArgument)
	}
}

func TestRoutingService_SimulateRouting(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	// Create a rule
	_, _ = svc.CreateRoutingRule(ctx, &routingv1.CreateRoutingRuleRequest{
		Rule: &routingv1.RoutingRule{
			Name:     "Critical Alert Handler",
			Priority: 1,
			Enabled:  true,
			Conditions: []*routingv1.RoutingCondition{
				{
					Type:        routingv1.ConditionType_CONDITION_TYPE_LABEL,
					Field:       "severity",
					Operator:    routingv1.ConditionOperator_CONDITION_OPERATOR_EQUALS,
					StringValue: "critical",
				},
			},
			Actions: []*routingv1.RoutingAction{
				{Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM},
			},
		},
	})

	resp, err := svc.SimulateRouting(ctx, &routingv1.SimulateRoutingRequest{
		Alert: &routingv1.Alert{
			Id:      "alert-1",
			Summary: "Critical Alert",
			Labels:  map[string]string{"severity": "critical"},
		},
	})

	if err != nil {
		t.Fatalf("SimulateRouting() error = %v", err)
	}

	if len(resp.Evaluations) != 1 {
		t.Errorf("SimulateRouting() evaluations = %d, want 1", len(resp.Evaluations))
	}

	if !resp.Evaluations[0].Matched {
		t.Error("SimulateRouting() rule should match")
	}

	if len(resp.Actions) != 1 {
		t.Errorf("SimulateRouting() actions = %d, want 1", len(resp.Actions))
	}
}

func TestRoutingService_SimulateRouting_NoRules(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	resp, err := svc.SimulateRouting(ctx, &routingv1.SimulateRoutingRequest{
		Alert: &routingv1.Alert{
			Id:      "alert-1",
			Summary: "Test Alert",
			Labels:  map[string]string{"severity": "warning"},
		},
	})

	if err != nil {
		t.Fatalf("SimulateRouting() error = %v", err)
	}

	if len(resp.Warnings) == 0 {
		t.Error("SimulateRouting() should warn about no rules")
	}
}

func TestRoutingService_SimulateRouting_NilAlert(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.SimulateRouting(ctx, &routingv1.SimulateRoutingRequest{Alert: nil})

	if err == nil {
		t.Fatal("SimulateRouting() should error for nil alert")
	}

	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("SimulateRouting() code = %v, want %v", st.Code(), codes.InvalidArgument)
	}
}

func TestRoutingService_RouteAlert(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	// Create rules
	_, _ = svc.CreateRoutingRule(ctx, &routingv1.CreateRoutingRuleRequest{
		Rule: &routingv1.RoutingRule{
			Name:     "Suppress Known Issue",
			Priority: 1,
			Enabled:  true,
			Conditions: []*routingv1.RoutingCondition{
				{
					Type:        routingv1.ConditionType_CONDITION_TYPE_LABEL,
					Field:       "known_issue",
					Operator:    routingv1.ConditionOperator_CONDITION_OPERATOR_EQUALS,
					StringValue: "true",
				},
			},
			Actions: []*routingv1.RoutingAction{
				{
					Type: routingv1.ActionType_ACTION_TYPE_SUPPRESS,
					Suppress: &routingv1.SuppressAction{
						Reason: "Known issue - auto suppressed",
					},
				},
			},
		},
	})

	resp, err := svc.RouteAlert(ctx, &routingv1.RouteAlertRequest{
		Alert: &routingv1.Alert{
			Id:          "alert-1",
			Summary:     "Known Issue Alert",
			Fingerprint: "fp-123",
			Labels:      map[string]string{"known_issue": "true"},
		},
	})

	if err != nil {
		t.Fatalf("RouteAlert() error = %v", err)
	}

	if !resp.Suppressed {
		t.Error("RouteAlert() should suppress the alert")
	}

	if resp.SuppressionReason != "Known issue - auto suppressed" {
		t.Errorf("RouteAlert() suppression reason = %q", resp.SuppressionReason)
	}

	if resp.AuditLog == nil {
		t.Error("RouteAlert() should return audit log")
	}
}

func TestRoutingService_RouteAlert_NilAlert(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.RouteAlert(ctx, &routingv1.RouteAlertRequest{Alert: nil})

	if err == nil {
		t.Fatal("RouteAlert() should error for nil alert")
	}

	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Errorf("RouteAlert() code = %v, want %v", st.Code(), codes.InvalidArgument)
	}
}

func TestRoutingService_GetRoutingAuditLogs(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	// Route an alert to create audit log
	_, _ = svc.CreateRoutingRule(ctx, &routingv1.CreateRoutingRuleRequest{
		Rule: &routingv1.RoutingRule{
			Name:     "Test Rule",
			Priority: 1,
			Enabled:  true,
			Actions:  []*routingv1.RoutingAction{{Type: routingv1.ActionType_ACTION_TYPE_SET_LABEL}},
		},
	})

	_, _ = svc.RouteAlert(ctx, &routingv1.RouteAlertRequest{
		Alert: &routingv1.Alert{
			Id:      "alert-1",
			Summary: "Test",
			Labels:  map[string]string{},
		},
	})

	resp, err := svc.GetRoutingAuditLogs(ctx, &routingv1.GetRoutingAuditLogsRequest{
		AlertId: "alert-1",
	})

	if err != nil {
		t.Fatalf("GetRoutingAuditLogs() error = %v", err)
	}

	if len(resp.Logs) != 1 {
		t.Errorf("GetRoutingAuditLogs() count = %d, want 1", len(resp.Logs))
	}
}
