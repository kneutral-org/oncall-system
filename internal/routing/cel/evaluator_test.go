package cel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

func TestNewEvaluator(t *testing.T) {
	eval, err := NewEvaluator()
	require.NoError(t, err)
	require.NotNil(t, eval)
	assert.NotNil(t, eval.Cache())
	assert.NotNil(t, eval.Env())
}

func TestEvaluator_Compile(t *testing.T) {
	eval, err := NewEvaluator()
	require.NoError(t, err)

	tests := []struct {
		name        string
		expression  string
		shouldError bool
		errorType   error
	}{
		{
			name:        "valid label check",
			expression:  `alert_labels["severity"] == "critical"`,
			shouldError: false,
		},
		{
			name:        "valid contains check",
			expression:  `hasLabel(alert_labels, "environment")`,
			shouldError: false,
		},
		{
			name:        "valid severity comparison",
			expression:  `severityAtLeast(alert_severity, "warning")`,
			shouldError: false,
		},
		{
			name:        "empty expression",
			expression:  "",
			shouldError: true,
			errorType:   ErrEmptyExpression,
		},
		{
			name:        "syntax error",
			expression:  `alert_labels["severity" ==`,
			shouldError: true,
			errorType:   ErrCompilationFailed,
		},
		{
			name:        "non-boolean result",
			expression:  `alert_severity`,
			shouldError: true,
			errorType:   ErrNotBoolean,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled, err := eval.Compile(tt.expression)
			if tt.shouldError {
				assert.Error(t, err)
				if tt.errorType != nil {
					assert.ErrorIs(t, err, tt.errorType)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, compiled)
				assert.Equal(t, tt.expression, compiled.Expression)
			}
		})
	}
}

func TestEvaluator_Evaluate(t *testing.T) {
	eval, err := NewEvaluator()
	require.NoError(t, err)

	alert := &routingv1.Alert{
		Id:        "alert-123",
		Summary:   "High CPU usage detected",
		Details:   "CPU usage exceeded 90% for 5 minutes",
		ServiceId: "service-001",
		Source:    routingv1.AlertSource_ALERT_SOURCE_PROMETHEUS,
		Labels: map[string]string{
			"severity":    "critical",
			"environment": "production",
			"team":        "platform",
			"region":      "us-east-1",
		},
		Annotations: map[string]string{
			"runbook": "https://runbooks.example.com/cpu",
			"grafana": "https://grafana.example.com/d/cpu",
		},
		CreatedAt: timestamppb.New(time.Now()),
	}

	ctx := &EvalContext{
		Now: time.Now(),
	}

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "label equals match",
			expression: `alert_labels["severity"] == "critical"`,
			expected:   true,
		},
		{
			name:       "label equals no match",
			expression: `alert_labels["severity"] == "warning"`,
			expected:   false,
		},
		{
			name:       "hasLabel exists",
			expression: `hasLabel(alert_labels, "environment")`,
			expected:   true,
		},
		{
			name:       "hasLabel not exists",
			expression: `hasLabel(alert_labels, "nonexistent")`,
			expected:   false,
		},
		{
			name:       "labelEquals match",
			expression: `labelEquals(alert_labels, "environment", "production")`,
			expected:   true,
		},
		{
			name:       "labelEquals no match",
			expression: `labelEquals(alert_labels, "environment", "staging")`,
			expected:   false,
		},
		{
			name:       "severityAtLeast critical >= warning",
			expression: `severityAtLeast(alert_labels["severity"], "warning")`,
			expected:   true,
		},
		{
			name:       "severityAtLeast warning >= critical",
			expression: `severityAtLeast("warning", "critical")`,
			expected:   false,
		},
		{
			name:       "startsWith match",
			expression: `startsWith(alert_labels["region"], "us-")`,
			expected:   true,
		},
		{
			name:       "startsWith no match",
			expression: `startsWith(alert_labels["region"], "eu-")`,
			expected:   false,
		},
		{
			name:       "endsWith match",
			expression: `endsWith(alert_labels["region"], "-1")`,
			expected:   true,
		},
		{
			name:       "matches regex",
			expression: `alert_labels["region"].matches("^us-east-[0-9]+$")`,
			expected:   true,
		},
		{
			name:       "matches regex no match",
			expression: `alert_labels["region"].matches("^eu-.*$")`,
			expected:   false,
		},
		{
			name:       "complex condition AND",
			expression: `labelEquals(alert_labels, "severity", "critical") && hasLabel(alert_labels, "environment")`,
			expected:   true,
		},
		{
			name:       "complex condition OR",
			expression: `labelEquals(alert_labels, "severity", "warning") || labelEquals(alert_labels, "environment", "production")`,
			expected:   true,
		},
		{
			name:       "check summary contains",
			expression: `alert_summary.contains("CPU")`,
			expected:   true,
		},
		{
			name:       "check service id",
			expression: `alert_service_id == "service-001"`,
			expected:   true,
		},
		{
			name:       "annotation check",
			expression: `hasLabel(alert_annotations, "runbook")`,
			expected:   true,
		},
		{
			name:       "labelIn match",
			expression: `labelIn(alert_labels, "severity", ["critical", "high"])`,
			expected:   true,
		},
		{
			name:       "labelIn no match",
			expression: `labelIn(alert_labels, "severity", ["warning", "info"])`,
			expected:   false,
		},
		{
			name:       "labelMatches regex",
			expression: `labelMatches(alert_labels, "team", "^plat.*$")`,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.EvaluateExpression(tt.expression, alert, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluator_EvaluateWithSite(t *testing.T) {
	eval, err := NewEvaluator()
	require.NoError(t, err)

	alert := &routingv1.Alert{
		Id: "alert-456",
		Labels: map[string]string{
			"severity": "high",
			"site":     "dc-east-1",
		},
	}

	site := &routingv1.Site{
		Id:       "site-001",
		Name:     "East Coast DC 1",
		Code:     "dc-east-1",
		Type:     routingv1.SiteType_SITE_TYPE_DATACENTER,
		Region:   "us-east",
		Country:  "USA",
		City:     "New York",
		Tier:     1,
		Timezone: "America/New_York",
		Metadata: map[string]string{
			"criticality": "high",
		},
	}

	ctx := &EvalContext{
		Site: site,
		Now:  time.Now(),
	}

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "site tier check",
			expression: `site_tier == 1`,
			expected:   true,
		},
		{
			name:       "site region check",
			expression: `site_region == "us-east"`,
			expected:   true,
		},
		{
			name:       "site type check",
			expression: `site_type == "SITE_TYPE_DATACENTER"`,
			expected:   true,
		},
		{
			name:       "site available check",
			expression: `site_available == true`,
			expected:   true,
		},
		{
			name:       "combined site and alert check",
			expression: `site_tier == 1 && labelEquals(alert_labels, "severity", "high")`,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.EvaluateExpression(tt.expression, alert, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluator_EvaluateWithCustomer(t *testing.T) {
	eval, err := NewEvaluator()
	require.NoError(t, err)

	alert := &routingv1.Alert{
		Id: "alert-789",
		Labels: map[string]string{
			"severity":    "critical",
			"customer_id": "cust-001",
		},
	}

	customer := &routingv1.CustomerTier{
		Id:    "tier-001",
		Name:  "Enterprise",
		Level: 1,
		Metadata: map[string]string{
			"sla": "platinum",
		},
	}

	ctx := &EvalContext{
		Customer: customer,
		Now:      time.Now(),
	}

	tests := []struct {
		name       string
		expression string
		expected   bool
	}{
		{
			name:       "customer tier check",
			expression: `customer_tier == 1`,
			expected:   true,
		},
		{
			name:       "customer name check",
			expression: `customer_name == "Enterprise"`,
			expected:   true,
		},
		{
			name:       "combined customer and severity check",
			expression: `customer_tier <= 2 && severityAtLeast(alert_labels["severity"], "high")`,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := eval.EvaluateExpression(tt.expression, alert, ctx)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvaluator_Validate(t *testing.T) {
	eval, err := NewEvaluator()
	require.NoError(t, err)

	tests := []struct {
		name        string
		expression  string
		shouldError bool
	}{
		{
			name:        "valid expression",
			expression:  `alert_labels["severity"] == "critical"`,
			shouldError: false,
		},
		{
			name:        "empty expression",
			expression:  "",
			shouldError: true,
		},
		{
			name:        "invalid syntax",
			expression:  `alert_labels[`,
			shouldError: true,
		},
		{
			name:        "non-boolean return",
			expression:  `alert_severity`,
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := eval.Validate(tt.expression)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEvaluator_BatchEvaluate(t *testing.T) {
	eval, err := NewEvaluator()
	require.NoError(t, err)

	alert := &routingv1.Alert{
		Id: "alert-batch",
		Labels: map[string]string{
			"severity":    "critical",
			"environment": "production",
		},
	}

	expressions := []string{
		`alert_labels["severity"] == "critical"`,
		`alert_labels["environment"] == "production"`,
		`alert_labels["severity"] == "warning"`,
		`hasLabel(alert_labels, "nonexistent")`,
	}

	results := eval.BatchEvaluate(expressions, alert, nil)

	require.Len(t, results, 4)
	assert.True(t, results[0].Matched)
	assert.True(t, results[1].Matched)
	assert.False(t, results[2].Matched)
	assert.False(t, results[3].Matched)
}

func TestEvaluator_NilAlert(t *testing.T) {
	eval, err := NewEvaluator()
	require.NoError(t, err)

	// Expression that should work with nil alert (using defaults)
	result, err := eval.EvaluateExpression(`alert_severity == ""`, nil, nil)
	require.NoError(t, err)
	assert.True(t, result)
}

func TestEvaluator_CacheHits(t *testing.T) {
	eval, err := NewEvaluator()
	require.NoError(t, err)

	expression := `alert_labels["severity"] == "critical"`
	alert := &routingv1.Alert{
		Labels: map[string]string{"severity": "critical"},
	}

	// First evaluation (cache miss)
	result1 := eval.EvaluateWithDetails(expression, alert, nil)
	require.NoError(t, result1.Error)
	assert.True(t, result1.Matched)
	assert.False(t, result1.CacheHit)

	// Second evaluation (cache hit)
	result2 := eval.EvaluateWithDetails(expression, alert, nil)
	require.NoError(t, result2.Error)
	assert.True(t, result2.Matched)
	assert.True(t, result2.CacheHit)

	// Cache should have one entry
	assert.Equal(t, 1, eval.Cache().Size())
}
