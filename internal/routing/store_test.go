package routing

import (
	"context"
	"testing"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

func TestInMemoryStore_CreateRule(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	rule := &routingv1.RoutingRule{
		Name:        "Test Rule",
		Description: "A test routing rule",
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
	}

	created, err := store.CreateRule(ctx, rule)
	if err != nil {
		t.Fatalf("CreateRule() error = %v", err)
	}

	if created.Id == "" {
		t.Error("CreateRule() should generate an ID")
	}

	if created.Name != rule.Name {
		t.Errorf("CreateRule() name = %q, want %q", created.Name, rule.Name)
	}

	if created.CreatedAt == nil {
		t.Error("CreateRule() should set CreatedAt")
	}

	if created.UpdatedAt == nil {
		t.Error("CreateRule() should set UpdatedAt")
	}
}

func TestInMemoryStore_CreateRule_DuplicatePriority(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	rule1 := &routingv1.RoutingRule{
		Name:     "Rule 1",
		Priority: 1,
		Enabled:  true,
	}

	rule2 := &routingv1.RoutingRule{
		Name:     "Rule 2",
		Priority: 1, // Same priority
		Enabled:  true,
	}

	_, err := store.CreateRule(ctx, rule1)
	if err != nil {
		t.Fatalf("CreateRule() error = %v", err)
	}

	_, err = store.CreateRule(ctx, rule2)
	if err != ErrDuplicatePriority {
		t.Errorf("CreateRule() error = %v, want %v", err, ErrDuplicatePriority)
	}
}

func TestInMemoryStore_GetRule(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	rule := &routingv1.RoutingRule{
		Name:     "Test Rule",
		Priority: 1,
		Enabled:  true,
	}

	created, _ := store.CreateRule(ctx, rule)

	got, err := store.GetRule(ctx, created.Id)
	if err != nil {
		t.Fatalf("GetRule() error = %v", err)
	}

	if got.Id != created.Id {
		t.Errorf("GetRule() id = %q, want %q", got.Id, created.Id)
	}

	if got.Name != created.Name {
		t.Errorf("GetRule() name = %q, want %q", got.Name, created.Name)
	}
}

func TestInMemoryStore_GetRule_NotFound(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_, err := store.GetRule(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("GetRule() error = %v, want %v", err, ErrNotFound)
	}
}

func TestInMemoryStore_ListRules(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create some rules
	for i := 1; i <= 5; i++ {
		rule := &routingv1.RoutingRule{
			Name:     "Rule " + string(rune('A'-1+i)),
			Priority: int32(i),
			Enabled:  i%2 == 0, // Even numbered rules are enabled
		}
		_, _ = store.CreateRule(ctx, rule)
	}

	// List all rules
	resp, err := store.ListRules(ctx, &routingv1.ListRoutingRulesRequest{})
	if err != nil {
		t.Fatalf("ListRules() error = %v", err)
	}

	if len(resp.Rules) != 5 {
		t.Errorf("ListRules() count = %d, want 5", len(resp.Rules))
	}

	// Verify sorted by priority
	for i := 1; i < len(resp.Rules); i++ {
		if resp.Rules[i].Priority < resp.Rules[i-1].Priority {
			t.Error("ListRules() should be sorted by priority")
		}
	}

	// List only enabled rules
	resp, err = store.ListRules(ctx, &routingv1.ListRoutingRulesRequest{EnabledOnly: true})
	if err != nil {
		t.Fatalf("ListRules() error = %v", err)
	}

	if len(resp.Rules) != 2 {
		t.Errorf("ListRules(EnabledOnly) count = %d, want 2", len(resp.Rules))
	}
}

func TestInMemoryStore_UpdateRule(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	rule := &routingv1.RoutingRule{
		Name:        "Original Name",
		Description: "Original Description",
		Priority:    1,
		Enabled:     true,
	}

	created, _ := store.CreateRule(ctx, rule)

	// Update the rule
	created.Name = "Updated Name"
	created.Description = "Updated Description"
	created.Enabled = false

	updated, err := store.UpdateRule(ctx, created)
	if err != nil {
		t.Fatalf("UpdateRule() error = %v", err)
	}

	if updated.Name != "Updated Name" {
		t.Errorf("UpdateRule() name = %q, want %q", updated.Name, "Updated Name")
	}

	if updated.Enabled != false {
		t.Error("UpdateRule() enabled should be false")
	}

	if updated.UpdatedAt.AsTime().Before(updated.CreatedAt.AsTime()) {
		t.Error("UpdateRule() UpdatedAt should be after CreatedAt")
	}
}

func TestInMemoryStore_UpdateRule_NotFound(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	rule := &routingv1.RoutingRule{
		Id:   "nonexistent",
		Name: "Test",
	}

	_, err := store.UpdateRule(ctx, rule)
	if err != ErrNotFound {
		t.Errorf("UpdateRule() error = %v, want %v", err, ErrNotFound)
	}
}

func TestInMemoryStore_DeleteRule(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	rule := &routingv1.RoutingRule{
		Name:     "To Be Deleted",
		Priority: 1,
		Enabled:  true,
	}

	created, _ := store.CreateRule(ctx, rule)

	err := store.DeleteRule(ctx, created.Id)
	if err != nil {
		t.Fatalf("DeleteRule() error = %v", err)
	}

	_, err = store.GetRule(ctx, created.Id)
	if err != ErrNotFound {
		t.Error("DeleteRule() rule should be deleted")
	}
}

func TestInMemoryStore_DeleteRule_NotFound(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	err := store.DeleteRule(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("DeleteRule() error = %v, want %v", err, ErrNotFound)
	}
}

func TestInMemoryStore_ReorderRules(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create rules
	rule1, _ := store.CreateRule(ctx, &routingv1.RoutingRule{
		Name:     "Rule A",
		Priority: 1,
		Enabled:  true,
	})

	rule2, _ := store.CreateRule(ctx, &routingv1.RoutingRule{
		Name:     "Rule B",
		Priority: 2,
		Enabled:  true,
	})

	// Reorder
	priorities := map[string]int32{
		rule1.Id: 10,
		rule2.Id: 5,
	}

	updated, err := store.ReorderRules(ctx, priorities)
	if err != nil {
		t.Fatalf("ReorderRules() error = %v", err)
	}

	if len(updated) != 2 {
		t.Errorf("ReorderRules() returned %d rules, want 2", len(updated))
	}

	// Verify new priorities
	got1, _ := store.GetRule(ctx, rule1.Id)
	if got1.Priority != 10 {
		t.Errorf("Rule1 priority = %d, want 10", got1.Priority)
	}

	got2, _ := store.GetRule(ctx, rule2.Id)
	if got2.Priority != 5 {
		t.Errorf("Rule2 priority = %d, want 5", got2.Priority)
	}
}

func TestInMemoryStore_GetEnabledRulesByPriority(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create mixed enabled/disabled rules
	_, _ = store.CreateRule(ctx, &routingv1.RoutingRule{
		Name:     "Enabled Low Priority",
		Priority: 3,
		Enabled:  true,
	})

	_, _ = store.CreateRule(ctx, &routingv1.RoutingRule{
		Name:     "Disabled",
		Priority: 1,
		Enabled:  false,
	})

	_, _ = store.CreateRule(ctx, &routingv1.RoutingRule{
		Name:     "Enabled High Priority",
		Priority: 2,
		Enabled:  true,
	})

	rules, err := store.GetEnabledRulesByPriority(ctx)
	if err != nil {
		t.Fatalf("GetEnabledRulesByPriority() error = %v", err)
	}

	if len(rules) != 2 {
		t.Errorf("GetEnabledRulesByPriority() count = %d, want 2", len(rules))
	}

	// Should be sorted by priority
	if rules[0].Priority > rules[1].Priority {
		t.Error("GetEnabledRulesByPriority() should be sorted by priority")
	}

	// All should be enabled
	for _, r := range rules {
		if !r.Enabled {
			t.Error("GetEnabledRulesByPriority() returned disabled rule")
		}
	}
}

func TestInMemoryStore_AuditLogs(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	// Create audit log
	log := &routingv1.RoutingAuditLog{
		AlertId: "alert-1",
		Evaluations: []*routingv1.RuleEvaluation{
			{
				RuleId:   "rule-1",
				RuleName: "Test Rule",
				Matched:  true,
			},
		},
	}

	err := store.CreateAuditLog(ctx, log)
	if err != nil {
		t.Fatalf("CreateAuditLog() error = %v", err)
	}

	if log.Id == "" {
		t.Error("CreateAuditLog() should generate an ID")
	}

	// Get audit logs
	resp, err := store.GetAuditLogs(ctx, &routingv1.GetRoutingAuditLogsRequest{
		AlertId: "alert-1",
	})
	if err != nil {
		t.Fatalf("GetAuditLogs() error = %v", err)
	}

	if len(resp.Logs) != 1 {
		t.Errorf("GetAuditLogs() count = %d, want 1", len(resp.Logs))
	}
}

func TestInMemoryStore_CreateRule_Nil(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_, err := store.CreateRule(ctx, nil)
	if err != ErrInvalidRule {
		t.Errorf("CreateRule(nil) error = %v, want %v", err, ErrInvalidRule)
	}
}

func TestInMemoryStore_UpdateRule_Nil(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_, err := store.UpdateRule(ctx, nil)
	if err != ErrInvalidRule {
		t.Errorf("UpdateRule(nil) error = %v, want %v", err, ErrInvalidRule)
	}
}

func TestInMemoryStore_UpdateRule_NoID(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	_, err := store.UpdateRule(ctx, &routingv1.RoutingRule{Name: "Test"})
	if err != ErrInvalidRule {
		t.Errorf("UpdateRule(no id) error = %v, want %v", err, ErrInvalidRule)
	}
}
