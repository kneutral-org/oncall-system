package cel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

func TestCustomFunctions_Contains(t *testing.T) {
	eval, err := NewEvaluator()
	require.NoError(t, err)

	alert := &routingv1.Alert{
		Labels: map[string]string{
			"severity": "critical",
		},
	}

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "contains map key exists",
			expression: `contains(alert_labels, "severity")`,
			expected:   true,
		},
		{
			name:       "contains map key not exists",
			expression: `contains(alert_labels, "nonexistent")`,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.EvaluateExpression(tt.expression, alert, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCustomFunctions_RegexMatch(t *testing.T) {
	eval, err := NewEvaluator()
	require.NoError(t, err)

	alert := &routingv1.Alert{
		Labels: map[string]string{
			"host":   "web-prod-001",
			"region": "us-east-1",
		},
	}

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "matches simple pattern",
			expression: `regexMatch(alert_labels["host"], "web-prod-.*")`,
			expected:   true,
		},
		{
			name:       "matches complex pattern",
			expression: `regexMatch(alert_labels["region"], "^us-(east|west)-[0-9]+$")`,
			expected:   true,
		},
		{
			name:       "matches no match",
			expression: `regexMatch(alert_labels["host"], "^db-.*$")`,
			expected:   false,
		},
		{
			name:       "matches invalid regex returns false",
			expression: `regexMatch(alert_labels["host"], "[invalid")`,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.EvaluateExpression(tt.expression, alert, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCustomFunctions_StartsEnds(t *testing.T) {
	eval, err := NewEvaluator()
	require.NoError(t, err)

	alert := &routingv1.Alert{
		Labels: map[string]string{
			"host": "web-prod-001.example.com",
		},
	}

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "startsWith match",
			expression: `startsWith(alert_labels["host"], "web-")`,
			expected:   true,
		},
		{
			name:       "startsWith no match",
			expression: `startsWith(alert_labels["host"], "db-")`,
			expected:   false,
		},
		{
			name:       "endsWith match",
			expression: `endsWith(alert_labels["host"], ".com")`,
			expected:   true,
		},
		{
			name:       "endsWith no match",
			expression: `endsWith(alert_labels["host"], ".net")`,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.EvaluateExpression(tt.expression, alert, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCustomFunctions_GetLabel(t *testing.T) {
	eval, err := NewEvaluator()
	require.NoError(t, err)

	alert := &routingv1.Alert{
		Labels: map[string]string{
			"severity":    "critical",
			"environment": "production",
		},
	}

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "getLabel existing key",
			expression: `getLabel(alert_labels, "severity", "unknown") == "critical"`,
			expected:   true,
		},
		{
			name:       "getLabel missing key uses default",
			expression: `getLabel(alert_labels, "team", "default-team") == "default-team"`,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.EvaluateExpression(tt.expression, alert, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCustomFunctions_LabelEquals(t *testing.T) {
	eval, err := NewEvaluator()
	require.NoError(t, err)

	alert := &routingv1.Alert{
		Labels: map[string]string{
			"severity": "critical",
		},
	}

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "labelEquals match",
			expression: `labelEquals(alert_labels, "severity", "critical")`,
			expected:   true,
		},
		{
			name:       "labelEquals no match",
			expression: `labelEquals(alert_labels, "severity", "warning")`,
			expected:   false,
		},
		{
			name:       "labelEquals missing key",
			expression: `labelEquals(alert_labels, "nonexistent", "value")`,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.EvaluateExpression(tt.expression, alert, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCustomFunctions_LabelIn(t *testing.T) {
	eval, err := NewEvaluator()
	require.NoError(t, err)

	alert := &routingv1.Alert{
		Labels: map[string]string{
			"severity": "high",
		},
	}

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "labelIn match",
			expression: `labelIn(alert_labels, "severity", ["critical", "high", "medium"])`,
			expected:   true,
		},
		{
			name:       "labelIn no match",
			expression: `labelIn(alert_labels, "severity", ["low", "info"])`,
			expected:   false,
		},
		{
			name:       "labelIn missing key",
			expression: `labelIn(alert_labels, "nonexistent", ["value1", "value2"])`,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.EvaluateExpression(tt.expression, alert, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCustomFunctions_LabelMatches(t *testing.T) {
	eval, err := NewEvaluator()
	require.NoError(t, err)

	alert := &routingv1.Alert{
		Labels: map[string]string{
			"host": "web-prod-001",
		},
	}

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "labelMatches match",
			expression: `labelMatches(alert_labels, "host", "web-prod-[0-9]+")`,
			expected:   true,
		},
		{
			name:       "labelMatches no match",
			expression: `labelMatches(alert_labels, "host", "^db-.*$")`,
			expected:   false,
		},
		{
			name:       "labelMatches missing key",
			expression: `labelMatches(alert_labels, "nonexistent", ".*")`,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.EvaluateExpression(tt.expression, alert, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCustomFunctions_Severity(t *testing.T) {
	eval, err := NewEvaluator()
	require.NoError(t, err)

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "severityLevel critical",
			expression: `severityLevel("critical") == 5`,
			expected:   true,
		},
		{
			name:       "severityLevel high",
			expression: `severityLevel("high") == 4`,
			expected:   true,
		},
		{
			name:       "severityLevel warning",
			expression: `severityLevel("warning") == 3`,
			expected:   true,
		},
		{
			name:       "severityLevel info",
			expression: `severityLevel("info") == 2`,
			expected:   true,
		},
		{
			name:       "severityLevel debug",
			expression: `severityLevel("debug") == 1`,
			expected:   true,
		},
		{
			name:       "severityLevel unknown",
			expression: `severityLevel("unknown") == 0`,
			expected:   true,
		},
		{
			name:       "severityAtLeast critical >= critical",
			expression: `severityAtLeast("critical", "critical")`,
			expected:   true,
		},
		{
			name:       "severityAtLeast high >= warning",
			expression: `severityAtLeast("high", "warning")`,
			expected:   true,
		},
		{
			name:       "severityAtLeast warning >= critical",
			expression: `severityAtLeast("warning", "critical")`,
			expected:   false,
		},
		{
			name:       "severityAtLeast p1 >= p2",
			expression: `severityAtLeast("p1", "p2")`,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.EvaluateExpression(tt.expression, nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCustomFunctions_StringManipulation(t *testing.T) {
	eval, err := NewEvaluator()
	require.NoError(t, err)

	alert := &routingv1.Alert{
		Labels: map[string]string{
			"team": "  Platform Team  ",
		},
	}

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "lower function",
			expression: `lower("CRITICAL") == "critical"`,
			expected:   true,
		},
		{
			name:       "upper function",
			expression: `upper("critical") == "CRITICAL"`,
			expected:   true,
		},
		{
			name:       "trim function",
			expression: `trim(alert_labels["team"]) == "Platform Team"`,
			expected:   true,
		},
		{
			name:       "split function",
			expression: `split("a,b,c", ",")[1] == "b"`,
			expected:   true,
		},
		{
			name:       "join function",
			expression: `join(["a", "b", "c"], "-") == "a-b-c"`,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.EvaluateExpression(tt.expression, alert, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCustomFunctions_HasLabel(t *testing.T) {
	eval, err := NewEvaluator()
	require.NoError(t, err)

	alert := &routingv1.Alert{
		Labels: map[string]string{
			"severity":    "critical",
			"environment": "production",
		},
	}

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "hasLabel exists",
			expression: `hasLabel(alert_labels, "severity")`,
			expected:   true,
		},
		{
			name:       "hasLabel not exists",
			expression: `hasLabel(alert_labels, "team")`,
			expected:   false,
		},
		{
			name:       "hasLabel empty labels",
			expression: `hasLabel(alert_annotations, "runbook")`,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.EvaluateExpression(tt.expression, alert, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
