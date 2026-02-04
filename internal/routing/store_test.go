// Package routing provides the routing rule store implementation.
package routing

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRoutingRuleStore is an in-memory implementation for testing.
type MockRoutingRuleStore struct {
	rules map[uuid.UUID]*RoutingRule
}

// NewMockRoutingRuleStore creates a new mock store.
func NewMockRoutingRuleStore() *MockRoutingRuleStore {
	return &MockRoutingRuleStore{
		rules: make(map[uuid.UUID]*RoutingRule),
	}
}

func (m *MockRoutingRuleStore) CreateRule(ctx context.Context, rule *RoutingRule) (*RoutingRule, error) {
	rule.ID = uuid.New()
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	m.rules[rule.ID] = rule
	return rule, nil
}

func (m *MockRoutingRuleStore) GetRule(ctx context.Context, id uuid.UUID) (*RoutingRule, error) {
	rule, ok := m.rules[id]
	if !ok {
		return nil, nil
	}
	return rule, nil
}

func (m *MockRoutingRuleStore) ListRules(ctx context.Context, params ListRulesParams) ([]*RoutingRule, error) {
	rules := make([]*RoutingRule, 0)
	for _, r := range m.rules {
		// Apply filters
		if len(params.EnabledFilter) > 0 {
			match := false
			for _, e := range params.EnabledFilter {
				if r.Enabled == e {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		if len(params.TagsFilter) > 0 {
			match := false
			for _, ft := range params.TagsFilter {
				for _, rt := range r.Tags {
					if ft == rt {
						match = true
						break
					}
				}
			}
			if !match {
				continue
			}
		}
		rules = append(rules, r)
	}
	return rules, nil
}

func (m *MockRoutingRuleStore) UpdateRule(ctx context.Context, rule *RoutingRule) (*RoutingRule, error) {
	existing, ok := m.rules[rule.ID]
	if !ok {
		return nil, nil
	}
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()
	m.rules[rule.ID] = rule
	return rule, nil
}

func (m *MockRoutingRuleStore) DeleteRule(ctx context.Context, id uuid.UUID) error {
	delete(m.rules, id)
	return nil
}

func (m *MockRoutingRuleStore) ReorderRules(ctx context.Context, priorities map[uuid.UUID]int32) error {
	for id, priority := range priorities {
		if rule, ok := m.rules[id]; ok {
			rule.Priority = priority
			rule.UpdatedAt = time.Now()
		}
	}
	return nil
}

func (m *MockRoutingRuleStore) GetEnabledRulesByPriority(ctx context.Context) ([]*RoutingRule, error) {
	rules := make([]*RoutingRule, 0)
	for _, r := range m.rules {
		if r.Enabled {
			rules = append(rules, r)
		}
	}
	return rules, nil
}

// Verify MockRoutingRuleStore implements RoutingRuleStore interface
var _ RoutingRuleStore = (*MockRoutingRuleStore)(nil)

func TestMockRoutingRuleStore_CreateRule(t *testing.T) {
	store := NewMockRoutingRuleStore()
	ctx := context.Background()

	rule := &RoutingRule{
		Name:        "Test Rule",
		Description: "Test description",
		Priority:    100,
		Enabled:     true,
		Conditions: []RoutingCondition{
			{Type: "label", Field: "severity", Operator: "equals", Values: []string{"critical"}},
		},
		Actions: []RoutingAction{
			{Type: "notify_team", Config: map[string]interface{}{"team_id": "team-1"}},
		},
		Terminal: false,
		Tags:     []string{"network", "critical"},
	}

	created, err := store.CreateRule(ctx, rule)
	require.NoError(t, err)
	require.NotNil(t, created)

	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, "Test Rule", created.Name)
	assert.Equal(t, "Test description", created.Description)
	assert.Equal(t, int32(100), created.Priority)
	assert.True(t, created.Enabled)
	assert.Len(t, created.Conditions, 1)
	assert.Len(t, created.Actions, 1)
	assert.False(t, created.Terminal)
	assert.ElementsMatch(t, []string{"network", "critical"}, created.Tags)
	assert.False(t, created.CreatedAt.IsZero())
	assert.False(t, created.UpdatedAt.IsZero())
}

func TestMockRoutingRuleStore_GetRule(t *testing.T) {
	store := NewMockRoutingRuleStore()
	ctx := context.Background()

	rule := &RoutingRule{
		Name:     "Test Rule",
		Priority: 100,
		Enabled:  true,
	}

	created, err := store.CreateRule(ctx, rule)
	require.NoError(t, err)

	// Get existing rule
	fetched, err := store.GetRule(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, "Test Rule", fetched.Name)

	// Get non-existing rule
	fetched, err = store.GetRule(ctx, uuid.New())
	require.NoError(t, err)
	assert.Nil(t, fetched)
}

func TestMockRoutingRuleStore_ListRules(t *testing.T) {
	store := NewMockRoutingRuleStore()
	ctx := context.Background()

	// Create test rules
	rules := []*RoutingRule{
		{Name: "Rule 1", Priority: 10, Enabled: true, Tags: []string{"network"}},
		{Name: "Rule 2", Priority: 20, Enabled: false, Tags: []string{"database"}},
		{Name: "Rule 3", Priority: 30, Enabled: true, Tags: []string{"network", "critical"}},
	}

	for _, r := range rules {
		_, err := store.CreateRule(ctx, r)
		require.NoError(t, err)
	}

	// List all rules
	result, err := store.ListRules(ctx, ListRulesParams{})
	require.NoError(t, err)
	assert.Len(t, result, 3)

	// List enabled rules only
	result, err = store.ListRules(ctx, ListRulesParams{EnabledFilter: []bool{true}})
	require.NoError(t, err)
	assert.Len(t, result, 2)

	// List by tags
	result, err = store.ListRules(ctx, ListRulesParams{TagsFilter: []string{"network"}})
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestMockRoutingRuleStore_UpdateRule(t *testing.T) {
	store := NewMockRoutingRuleStore()
	ctx := context.Background()

	rule := &RoutingRule{
		Name:     "Test Rule",
		Priority: 100,
		Enabled:  true,
	}

	created, err := store.CreateRule(ctx, rule)
	require.NoError(t, err)

	// Update the rule
	created.Name = "Updated Rule"
	created.Priority = 50
	created.Enabled = false

	updated, err := store.UpdateRule(ctx, created)
	require.NoError(t, err)
	require.NotNil(t, updated)

	assert.Equal(t, "Updated Rule", updated.Name)
	assert.Equal(t, int32(50), updated.Priority)
	assert.False(t, updated.Enabled)
	assert.True(t, updated.UpdatedAt.After(updated.CreatedAt))

	// Update non-existing rule
	nonExisting := &RoutingRule{ID: uuid.New(), Name: "Non-existing"}
	result, err := store.UpdateRule(ctx, nonExisting)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestMockRoutingRuleStore_DeleteRule(t *testing.T) {
	store := NewMockRoutingRuleStore()
	ctx := context.Background()

	rule := &RoutingRule{
		Name:     "Test Rule",
		Priority: 100,
		Enabled:  true,
	}

	created, err := store.CreateRule(ctx, rule)
	require.NoError(t, err)

	// Delete the rule
	err = store.DeleteRule(ctx, created.ID)
	require.NoError(t, err)

	// Verify it's deleted
	fetched, err := store.GetRule(ctx, created.ID)
	require.NoError(t, err)
	assert.Nil(t, fetched)
}

func TestMockRoutingRuleStore_ReorderRules(t *testing.T) {
	store := NewMockRoutingRuleStore()
	ctx := context.Background()

	// Create test rules
	rule1 := &RoutingRule{Name: "Rule 1", Priority: 10, Enabled: true}
	rule2 := &RoutingRule{Name: "Rule 2", Priority: 20, Enabled: true}
	rule3 := &RoutingRule{Name: "Rule 3", Priority: 30, Enabled: true}

	created1, _ := store.CreateRule(ctx, rule1)
	created2, _ := store.CreateRule(ctx, rule2)
	created3, _ := store.CreateRule(ctx, rule3)

	// Reorder rules
	priorities := map[uuid.UUID]int32{
		created1.ID: 30,
		created2.ID: 10,
		created3.ID: 20,
	}

	err := store.ReorderRules(ctx, priorities)
	require.NoError(t, err)

	// Verify new priorities
	fetched1, _ := store.GetRule(ctx, created1.ID)
	fetched2, _ := store.GetRule(ctx, created2.ID)
	fetched3, _ := store.GetRule(ctx, created3.ID)

	assert.Equal(t, int32(30), fetched1.Priority)
	assert.Equal(t, int32(10), fetched2.Priority)
	assert.Equal(t, int32(20), fetched3.Priority)
}

func TestMockRoutingRuleStore_GetEnabledRulesByPriority(t *testing.T) {
	store := NewMockRoutingRuleStore()
	ctx := context.Background()

	// Create test rules (mix of enabled and disabled)
	rules := []*RoutingRule{
		{Name: "Rule 1", Priority: 10, Enabled: true},
		{Name: "Rule 2", Priority: 20, Enabled: false},
		{Name: "Rule 3", Priority: 5, Enabled: true},
	}

	for _, r := range rules {
		_, err := store.CreateRule(ctx, r)
		require.NoError(t, err)
	}

	// Get enabled rules
	result, err := store.GetEnabledRulesByPriority(ctx)
	require.NoError(t, err)
	assert.Len(t, result, 2)

	// All returned rules should be enabled
	for _, r := range result {
		assert.True(t, r.Enabled)
	}
}

func TestRoutingCondition_JSON(t *testing.T) {
	condition := RoutingCondition{
		Type:     "label",
		Field:    "severity",
		Operator: "equals",
		Values:   []string{"critical", "warning"},
	}

	// Test marshaling
	data, err := json.Marshal(condition)
	require.NoError(t, err)

	// Test unmarshaling
	var parsed RoutingCondition
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, condition.Type, parsed.Type)
	assert.Equal(t, condition.Field, parsed.Field)
	assert.Equal(t, condition.Operator, parsed.Operator)
	assert.ElementsMatch(t, condition.Values, parsed.Values)
}

func TestRoutingAction_JSON(t *testing.T) {
	action := RoutingAction{
		Type: "notify_team",
		Config: map[string]interface{}{
			"team_id": "team-123",
			"urgent":  true,
		},
	}

	// Test marshaling
	data, err := json.Marshal(action)
	require.NoError(t, err)

	// Test unmarshaling
	var parsed RoutingAction
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, action.Type, parsed.Type)
	assert.Equal(t, "team-123", parsed.Config["team_id"])
	assert.Equal(t, true, parsed.Config["urgent"])
}

func TestTimeCondition_JSON(t *testing.T) {
	tc := TimeCondition{
		Timezone:     "America/New_York",
		DaysOfWeek:   []int{1, 2, 3, 4, 5},
		StartTime:    "09:00",
		EndTime:      "17:00",
		ExcludeDates: []string{"2024-12-25", "2024-01-01"},
	}

	// Test marshaling
	data, err := json.Marshal(tc)
	require.NoError(t, err)

	// Test unmarshaling
	var parsed TimeCondition
	err = json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, tc.Timezone, parsed.Timezone)
	assert.ElementsMatch(t, tc.DaysOfWeek, parsed.DaysOfWeek)
	assert.Equal(t, tc.StartTime, parsed.StartTime)
	assert.Equal(t, tc.EndTime, parsed.EndTime)
	assert.ElementsMatch(t, tc.ExcludeDates, parsed.ExcludeDates)
}
