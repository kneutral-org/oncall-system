package routing

import (
	"testing"
	"time"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

func TestEvaluator_EvaluateCondition_Label(t *testing.T) {
	evaluator := NewEvaluator()

	tests := []struct {
		name      string
		condition *routingv1.RoutingCondition
		alert     *routingv1.Alert
		wantMatch bool
	}{
		{
			name: "label equals match",
			condition: &routingv1.RoutingCondition{
				Type:        routingv1.ConditionType_CONDITION_TYPE_LABEL,
				Field:       "team",
				Operator:    routingv1.ConditionOperator_CONDITION_OPERATOR_EQUALS,
				StringValue: "platform",
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{"team": "platform"},
			},
			wantMatch: true,
		},
		{
			name: "label equals no match",
			condition: &routingv1.RoutingCondition{
				Type:        routingv1.ConditionType_CONDITION_TYPE_LABEL,
				Field:       "team",
				Operator:    routingv1.ConditionOperator_CONDITION_OPERATOR_EQUALS,
				StringValue: "platform",
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{"team": "infrastructure"},
			},
			wantMatch: false,
		},
		{
			name: "label not equals match",
			condition: &routingv1.RoutingCondition{
				Type:        routingv1.ConditionType_CONDITION_TYPE_LABEL,
				Field:       "team",
				Operator:    routingv1.ConditionOperator_CONDITION_OPERATOR_NOT_EQUALS,
				StringValue: "platform",
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{"team": "infrastructure"},
			},
			wantMatch: true,
		},
		{
			name: "label contains match",
			condition: &routingv1.RoutingCondition{
				Type:        routingv1.ConditionType_CONDITION_TYPE_LABEL,
				Field:       "service",
				Operator:    routingv1.ConditionOperator_CONDITION_OPERATOR_CONTAINS,
				StringValue: "api",
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{"service": "user-api-service"},
			},
			wantMatch: true,
		},
		{
			name: "label starts with match",
			condition: &routingv1.RoutingCondition{
				Type:        routingv1.ConditionType_CONDITION_TYPE_LABEL,
				Field:       "environment",
				Operator:    routingv1.ConditionOperator_CONDITION_OPERATOR_STARTS_WITH,
				StringValue: "prod",
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{"environment": "production"},
			},
			wantMatch: true,
		},
		{
			name: "label ends with match",
			condition: &routingv1.RoutingCondition{
				Type:        routingv1.ConditionType_CONDITION_TYPE_LABEL,
				Field:       "host",
				Operator:    routingv1.ConditionOperator_CONDITION_OPERATOR_ENDS_WITH,
				StringValue: ".example.com",
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{"host": "server1.example.com"},
			},
			wantMatch: true,
		},
		{
			name: "label regex match",
			condition: &routingv1.RoutingCondition{
				Type:         routingv1.ConditionType_CONDITION_TYPE_LABEL,
				Field:        "instance",
				Operator:     routingv1.ConditionOperator_CONDITION_OPERATOR_REGEX,
				RegexPattern: "^web-[0-9]+$",
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{"instance": "web-123"},
			},
			wantMatch: true,
		},
		{
			name: "label in list match",
			condition: &routingv1.RoutingCondition{
				Type:       routingv1.ConditionType_CONDITION_TYPE_LABEL,
				Field:      "severity",
				Operator:   routingv1.ConditionOperator_CONDITION_OPERATOR_IN,
				StringList: []string{"critical", "high"},
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{"severity": "critical"},
			},
			wantMatch: true,
		},
		{
			name: "label not in list match",
			condition: &routingv1.RoutingCondition{
				Type:       routingv1.ConditionType_CONDITION_TYPE_LABEL,
				Field:      "severity",
				Operator:   routingv1.ConditionOperator_CONDITION_OPERATOR_NOT_IN,
				StringList: []string{"info", "debug"},
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{"severity": "critical"},
			},
			wantMatch: true,
		},
		{
			name: "label exists match",
			condition: &routingv1.RoutingCondition{
				Type:     routingv1.ConditionType_CONDITION_TYPE_LABEL,
				Field:    "custom_label",
				Operator: routingv1.ConditionOperator_CONDITION_OPERATOR_EXISTS,
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{"custom_label": "value"},
			},
			wantMatch: true,
		},
		{
			name: "label not exists match",
			condition: &routingv1.RoutingCondition{
				Type:     routingv1.ConditionType_CONDITION_TYPE_LABEL,
				Field:    "missing_label",
				Operator: routingv1.ConditionOperator_CONDITION_OPERATOR_NOT_EXISTS,
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{"other_label": "value"},
			},
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluator.EvaluateCondition(tt.condition, tt.alert)
			if result.Matched != tt.wantMatch {
				t.Errorf("EvaluateCondition() matched = %v, want %v", result.Matched, tt.wantMatch)
			}
		})
	}
}

func TestEvaluator_EvaluateCondition_Severity(t *testing.T) {
	evaluator := NewEvaluator()

	tests := []struct {
		name      string
		condition *routingv1.RoutingCondition
		alert     *routingv1.Alert
		wantMatch bool
	}{
		{
			name: "severity equals critical",
			condition: &routingv1.RoutingCondition{
				Type:        routingv1.ConditionType_CONDITION_TYPE_SEVERITY,
				Operator:    routingv1.ConditionOperator_CONDITION_OPERATOR_EQUALS,
				StringValue: "critical",
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{"severity": "critical"},
			},
			wantMatch: true,
		},
		{
			name: "severity in list",
			condition: &routingv1.RoutingCondition{
				Type:       routingv1.ConditionType_CONDITION_TYPE_SEVERITY,
				Operator:   routingv1.ConditionOperator_CONDITION_OPERATOR_IN,
				StringList: []string{"critical", "high"},
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{"severity": "high"},
			},
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluator.EvaluateCondition(tt.condition, tt.alert)
			if result.Matched != tt.wantMatch {
				t.Errorf("EvaluateCondition() matched = %v, want %v", result.Matched, tt.wantMatch)
			}
		})
	}
}

func TestEvaluator_EvaluateCondition_Site(t *testing.T) {
	evaluator := NewEvaluator()

	tests := []struct {
		name      string
		condition *routingv1.RoutingCondition
		alert     *routingv1.Alert
		wantMatch bool
	}{
		{
			name: "site equals",
			condition: &routingv1.RoutingCondition{
				Type:        routingv1.ConditionType_CONDITION_TYPE_SITE,
				Operator:    routingv1.ConditionOperator_CONDITION_OPERATOR_EQUALS,
				StringValue: "IAD1",
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{"site": "IAD1"},
			},
			wantMatch: true,
		},
		{
			name: "site from datacenter label",
			condition: &routingv1.RoutingCondition{
				Type:        routingv1.ConditionType_CONDITION_TYPE_SITE,
				Operator:    routingv1.ConditionOperator_CONDITION_OPERATOR_EQUALS,
				StringValue: "SJC2",
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{"datacenter": "SJC2"},
			},
			wantMatch: true,
		},
		{
			name: "site in list",
			condition: &routingv1.RoutingCondition{
				Type:       routingv1.ConditionType_CONDITION_TYPE_SITE,
				Operator:   routingv1.ConditionOperator_CONDITION_OPERATOR_IN,
				StringList: []string{"IAD1", "IAD2", "SJC1"},
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{"site": "IAD2"},
			},
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluator.EvaluateCondition(tt.condition, tt.alert)
			if result.Matched != tt.wantMatch {
				t.Errorf("EvaluateCondition() matched = %v, want %v", result.Matched, tt.wantMatch)
			}
		})
	}
}

func TestEvaluator_EvaluateRule(t *testing.T) {
	evaluator := NewEvaluator()

	tests := []struct {
		name      string
		rule      *routingv1.RoutingRule
		alert     *routingv1.Alert
		wantMatch bool
	}{
		{
			name: "single condition match",
			rule: &routingv1.RoutingRule{
				Id:       "rule-1",
				Name:     "Critical Alerts",
				Enabled:  true,
				Priority: 1,
				Conditions: []*routingv1.RoutingCondition{
					{
						Type:        routingv1.ConditionType_CONDITION_TYPE_LABEL,
						Field:       "severity",
						Operator:    routingv1.ConditionOperator_CONDITION_OPERATOR_EQUALS,
						StringValue: "critical",
					},
				},
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{"severity": "critical"},
			},
			wantMatch: true,
		},
		{
			name: "multiple conditions all match (AND)",
			rule: &routingv1.RoutingRule{
				Id:       "rule-2",
				Name:     "Critical Production Alerts",
				Enabled:  true,
				Priority: 1,
				Conditions: []*routingv1.RoutingCondition{
					{
						Type:        routingv1.ConditionType_CONDITION_TYPE_LABEL,
						Field:       "severity",
						Operator:    routingv1.ConditionOperator_CONDITION_OPERATOR_EQUALS,
						StringValue: "critical",
					},
					{
						Type:        routingv1.ConditionType_CONDITION_TYPE_LABEL,
						Field:       "environment",
						Operator:    routingv1.ConditionOperator_CONDITION_OPERATOR_EQUALS,
						StringValue: "production",
					},
				},
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{
					"severity":    "critical",
					"environment": "production",
				},
			},
			wantMatch: true,
		},
		{
			name: "multiple conditions one fails (AND)",
			rule: &routingv1.RoutingRule{
				Id:       "rule-3",
				Name:     "Critical Production Alerts",
				Enabled:  true,
				Priority: 1,
				Conditions: []*routingv1.RoutingCondition{
					{
						Type:        routingv1.ConditionType_CONDITION_TYPE_LABEL,
						Field:       "severity",
						Operator:    routingv1.ConditionOperator_CONDITION_OPERATOR_EQUALS,
						StringValue: "critical",
					},
					{
						Type:        routingv1.ConditionType_CONDITION_TYPE_LABEL,
						Field:       "environment",
						Operator:    routingv1.ConditionOperator_CONDITION_OPERATOR_EQUALS,
						StringValue: "production",
					},
				},
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{
					"severity":    "critical",
					"environment": "staging",
				},
			},
			wantMatch: false,
		},
		{
			name: "no conditions always matches",
			rule: &routingv1.RoutingRule{
				Id:         "rule-4",
				Name:       "Catch All",
				Enabled:    true,
				Priority:   100,
				Conditions: []*routingv1.RoutingCondition{},
			},
			alert: &routingv1.Alert{
				Labels: map[string]string{"any": "value"},
			},
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluator.EvaluateRule(tt.rule, tt.alert, time.Now())
			if result.Matched != tt.wantMatch {
				t.Errorf("EvaluateRule() matched = %v, want %v", result.Matched, tt.wantMatch)
			}
		})
	}
}

func TestEvaluator_EvaluateTimeCondition(t *testing.T) {
	evaluator := NewEvaluator()

	// Create a fixed time for testing: Monday 10:30 AM UTC
	testTime := time.Date(2024, 1, 8, 10, 30, 0, 0, time.UTC) // Monday

	tests := []struct {
		name          string
		timeCondition *routingv1.TimeCondition
		wantMatch     bool
	}{
		{
			name: "within business hours",
			timeCondition: &routingv1.TimeCondition{
				Timezone: "UTC",
				Windows: []*routingv1.TimeWindow{
					{
						DaysOfWeek: []int32{1, 2, 3, 4, 5}, // Mon-Fri
						StartTime:  "09:00",
						EndTime:    "18:00",
					},
				},
			},
			wantMatch: true,
		},
		{
			name: "outside business hours",
			timeCondition: &routingv1.TimeCondition{
				Timezone: "UTC",
				Windows: []*routingv1.TimeWindow{
					{
						DaysOfWeek: []int32{1, 2, 3, 4, 5}, // Mon-Fri
						StartTime:  "18:00",
						EndTime:    "23:00",
					},
				},
			},
			wantMatch: false,
		},
		{
			name: "weekend only - no match on Monday",
			timeCondition: &routingv1.TimeCondition{
				Timezone: "UTC",
				Windows: []*routingv1.TimeWindow{
					{
						DaysOfWeek: []int32{0, 6}, // Sat-Sun
						StartTime:  "00:00",
						EndTime:    "23:59",
					},
				},
			},
			wantMatch: false,
		},
		{
			name: "inverted window - outside hours",
			timeCondition: &routingv1.TimeCondition{
				Timezone: "UTC",
				Windows: []*routingv1.TimeWindow{
					{
						DaysOfWeek: []int32{1, 2, 3, 4, 5},
						StartTime:  "09:00",
						EndTime:    "18:00",
						Invert:     true, // Active OUTSIDE these hours
					},
				},
			},
			wantMatch: false, // We're inside 09:00-18:00, but inverted means we want outside
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, _ := evaluator.evaluateTimeCondition(tt.timeCondition, testTime)
			if matched != tt.wantMatch {
				t.Errorf("evaluateTimeCondition() matched = %v, want %v", matched, tt.wantMatch)
			}
		})
	}
}

func TestEvaluator_EvaluateRules_TerminalRule(t *testing.T) {
	evaluator := NewEvaluator()

	rules := []*routingv1.RoutingRule{
		{
			Id:       "rule-1",
			Name:     "Critical Handler",
			Enabled:  true,
			Priority: 1,
			Terminal: true,
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
		{
			Id:       "rule-2",
			Name:     "All Alerts Logger",
			Enabled:  true,
			Priority: 2,
			Terminal: false,
			Conditions: []*routingv1.RoutingCondition{
				{
					Type:     routingv1.ConditionType_CONDITION_TYPE_LABEL,
					Field:    "severity",
					Operator: routingv1.ConditionOperator_CONDITION_OPERATOR_EXISTS,
				},
			},
			Actions: []*routingv1.RoutingAction{
				{Type: routingv1.ActionType_ACTION_TYPE_SET_LABEL},
			},
		},
	}

	alert := &routingv1.Alert{
		Labels: map[string]string{"severity": "critical"},
	}

	evaluations, actions := evaluator.EvaluateRules(rules, alert, time.Now())

	// Should have evaluated first rule and stopped (terminal)
	if len(evaluations) != 1 {
		t.Errorf("Expected 1 evaluation (terminal rule matched), got %d", len(evaluations))
	}

	if len(actions) != 1 {
		t.Errorf("Expected 1 action, got %d", len(actions))
	}

	if !evaluations[0].Terminal {
		t.Error("Expected first evaluation to be terminal")
	}
}

func TestSeverityLevel(t *testing.T) {
	tests := []struct {
		severity string
		want     int
	}{
		{"critical", 5},
		{"CRITICAL", 5},
		{"fatal", 5},
		{"p1", 5},
		{"high", 4},
		{"error", 4},
		{"p2", 4},
		{"warning", 3},
		{"warn", 3},
		{"medium", 3},
		{"p3", 3},
		{"info", 2},
		{"low", 2},
		{"p4", 2},
		{"debug", 1},
		{"p5", 1},
		{"unknown", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			got := SeverityLevel(tt.severity)
			if got != tt.want {
				t.Errorf("SeverityLevel(%q) = %d, want %d", tt.severity, got, tt.want)
			}
		})
	}
}

func TestCompareSeverity(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"critical", "high", 1},
		{"high", "critical", -1},
		{"critical", "critical", 0},
		{"info", "warning", -1},
		{"warning", "info", 1},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := CompareSeverity(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("CompareSeverity(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
