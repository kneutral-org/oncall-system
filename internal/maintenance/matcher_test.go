package maintenance

import (
	"testing"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

func TestMatcher_Match_GlobalScope(t *testing.T) {
	matcher := NewMatcher()

	alert := &routingv1.Alert{
		Id:       "alert-1",
		Summary:  "Test alert",
		ServiceId: "my-service",
		Labels: map[string]string{
			"site":     "us-east-1",
			"severity": "critical",
		},
	}

	// Window with no scope (global)
	window := &routingv1.MaintenanceWindow{
		Id:               "window-1",
		Name:             "Global Maintenance",
		AffectedSites:    nil,
		AffectedServices: nil,
		AffectedLabels:   nil,
	}

	result := matcher.Match(alert, window)

	if !result.Matched {
		t.Error("expected global window to match all alerts")
	}

	if result.MatchType != MatchTypeGlobal {
		t.Errorf("expected match type %s, got %s", MatchTypeGlobal, result.MatchType)
	}
}

func TestMatcher_Match_SiteScope(t *testing.T) {
	matcher := NewMatcher()

	tests := []struct {
		name          string
		alertLabels   map[string]string
		affectedSites []string
		expectMatch   bool
	}{
		{
			name:          "exact site match",
			alertLabels:   map[string]string{"site": "us-east-1"},
			affectedSites: []string{"us-east-1"},
			expectMatch:   true,
		},
		{
			name:          "wildcard site match",
			alertLabels:   map[string]string{"site": "us-east-1"},
			affectedSites: []string{"us-*"},
			expectMatch:   true,
		},
		{
			name:          "datacenter label match",
			alertLabels:   map[string]string{"datacenter": "dc1"},
			affectedSites: []string{"dc1"},
			expectMatch:   true,
		},
		{
			name:          "no site match",
			alertLabels:   map[string]string{"site": "eu-west-1"},
			affectedSites: []string{"us-east-1", "us-west-2"},
			expectMatch:   false,
		},
		{
			name:          "no site label",
			alertLabels:   map[string]string{"service": "my-service"},
			affectedSites: []string{"us-east-1"},
			expectMatch:   false,
		},
		{
			name:          "case insensitive match",
			alertLabels:   map[string]string{"site": "US-EAST-1"},
			affectedSites: []string{"us-east-1"},
			expectMatch:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			alert := &routingv1.Alert{
				Id:     "alert-1",
				Labels: tc.alertLabels,
			}

			window := &routingv1.MaintenanceWindow{
				Id:            "window-1",
				AffectedSites: tc.affectedSites,
			}

			result := matcher.Match(alert, window)

			if result.Matched != tc.expectMatch {
				t.Errorf("expected matched=%v, got matched=%v", tc.expectMatch, result.Matched)
			}

			if tc.expectMatch && result.MatchType != MatchTypeSite {
				t.Errorf("expected match type %s, got %s", MatchTypeSite, result.MatchType)
			}
		})
	}
}

func TestMatcher_Match_ServiceScope(t *testing.T) {
	matcher := NewMatcher()

	tests := []struct {
		name             string
		alertServiceId   string
		alertLabels      map[string]string
		affectedServices []string
		expectMatch      bool
	}{
		{
			name:             "exact service_id match",
			alertServiceId:   "api-gateway",
			affectedServices: []string{"api-gateway"},
			expectMatch:      true,
		},
		{
			name:             "service label match",
			alertLabels:      map[string]string{"service": "database"},
			affectedServices: []string{"database"},
			expectMatch:      true,
		},
		{
			name:             "wildcard service match",
			alertServiceId:   "api-gateway-prod",
			affectedServices: []string{"api-*"},
			expectMatch:      true,
		},
		{
			name:             "no service match",
			alertServiceId:   "web-server",
			affectedServices: []string{"api-gateway", "database"},
			expectMatch:      false,
		},
		{
			name:             "service_id takes precedence over label",
			alertServiceId:   "service-a",
			alertLabels:      map[string]string{"service": "service-b"},
			affectedServices: []string{"service-a"},
			expectMatch:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			alert := &routingv1.Alert{
				Id:        "alert-1",
				ServiceId: tc.alertServiceId,
				Labels:    tc.alertLabels,
			}

			window := &routingv1.MaintenanceWindow{
				Id:               "window-1",
				AffectedServices: tc.affectedServices,
			}

			result := matcher.Match(alert, window)

			if result.Matched != tc.expectMatch {
				t.Errorf("expected matched=%v, got matched=%v", tc.expectMatch, result.Matched)
			}

			if tc.expectMatch && result.MatchType != MatchTypeService {
				t.Errorf("expected match type %s, got %s", MatchTypeService, result.MatchType)
			}
		})
	}
}

func TestMatcher_Match_LabelScope(t *testing.T) {
	matcher := NewMatcher()

	tests := []struct {
		name           string
		alertLabels    map[string]string
		affectedLabels []string
		expectMatch    bool
	}{
		{
			name:           "exact label match",
			alertLabels:    map[string]string{"env": "production"},
			affectedLabels: []string{"env=production"},
			expectMatch:    true,
		},
		{
			name:           "label not equal",
			alertLabels:    map[string]string{"env": "staging"},
			affectedLabels: []string{"env!=production"},
			expectMatch:    true,
		},
		{
			name:           "label regex match",
			alertLabels:    map[string]string{"env": "prod-us-1"},
			affectedLabels: []string{"env=~prod-.*"},
			expectMatch:    true,
		},
		{
			name:           "label regex not match",
			alertLabels:    map[string]string{"env": "staging"},
			affectedLabels: []string{"env!~prod-.*"},
			expectMatch:    true,
		},
		{
			name:           "label exists",
			alertLabels:    map[string]string{"team": "backend"},
			affectedLabels: []string{"team"},
			expectMatch:    true,
		},
		{
			name:           "label not exists",
			alertLabels:    map[string]string{"env": "production"},
			affectedLabels: []string{"!team"},
			expectMatch:    true,
		},
		{
			name:           "multiple labels all match",
			alertLabels:    map[string]string{"env": "production", "team": "backend"},
			affectedLabels: []string{"env=production", "team=backend"},
			expectMatch:    true,
		},
		{
			name:           "multiple labels partial match",
			alertLabels:    map[string]string{"env": "production", "team": "frontend"},
			affectedLabels: []string{"env=production", "team=backend"},
			expectMatch:    false,
		},
		{
			name:           "label value mismatch",
			alertLabels:    map[string]string{"env": "staging"},
			affectedLabels: []string{"env=production"},
			expectMatch:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			alert := &routingv1.Alert{
				Id:     "alert-1",
				Labels: tc.alertLabels,
			}

			window := &routingv1.MaintenanceWindow{
				Id:             "window-1",
				AffectedLabels: tc.affectedLabels,
			}

			result := matcher.Match(alert, window)

			if result.Matched != tc.expectMatch {
				t.Errorf("expected matched=%v, got matched=%v", tc.expectMatch, result.Matched)
			}

			if tc.expectMatch && result.MatchType != MatchTypeLabel {
				t.Errorf("expected match type %s, got %s", MatchTypeLabel, result.MatchType)
			}
		})
	}
}

func TestParseLabelMatchers(t *testing.T) {
	tests := []struct {
		input    string
		expected LabelMatcher
	}{
		{
			input:    "key=value",
			expected: LabelMatcher{Name: "key", Value: "value", Operator: OperatorEqual},
		},
		{
			input:    "key!=value",
			expected: LabelMatcher{Name: "key", Value: "value", Operator: OperatorNotEqual},
		},
		{
			input:    "key=~regex.*",
			expected: LabelMatcher{Name: "key", Value: "regex.*", Operator: OperatorRegex},
		},
		{
			input:    "key!~regex.*",
			expected: LabelMatcher{Name: "key", Value: "regex.*", Operator: OperatorNotRegex},
		},
		{
			input:    "key",
			expected: LabelMatcher{Name: "key", Operator: OperatorExists},
		},
		{
			input:    "!key",
			expected: LabelMatcher{Name: "key", Operator: OperatorNotExists},
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			matchers := parseLabelMatchers([]string{tc.input})

			if len(matchers) != 1 {
				t.Fatalf("expected 1 matcher, got %d", len(matchers))
			}

			m := matchers[0]

			if m.Name != tc.expected.Name {
				t.Errorf("expected name %s, got %s", tc.expected.Name, m.Name)
			}

			if m.Value != tc.expected.Value {
				t.Errorf("expected value %s, got %s", tc.expected.Value, m.Value)
			}

			if m.Operator != tc.expected.Operator {
				t.Errorf("expected operator %s, got %s", tc.expected.Operator, m.Operator)
			}
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		value    string
		pattern  string
		expected bool
	}{
		{"us-east-1", "us-east-1", true},
		{"us-east-1", "US-EAST-1", true},
		{"us-east-1", "us-*", true},
		{"api-prod", "*-prod", true},
		{"api-gateway", "api-gateway", true},
		{"us-east-1", "eu-west-1", false},
		{"staging", "prod-*", false},
		{"prod-us", "^prod-.*$", true}, // regex
	}

	for _, tc := range tests {
		t.Run(tc.value+"_"+tc.pattern, func(t *testing.T) {
			result := matchesPattern(tc.value, tc.pattern)

			if result != tc.expected {
				t.Errorf("matchesPattern(%q, %q) = %v, expected %v", tc.value, tc.pattern, result, tc.expected)
			}
		})
	}
}

func TestMatcher_MatchEquipment(t *testing.T) {
	matcher := NewMatcher()

	tests := []struct {
		name           string
		alertLabels    map[string]string
		equipmentTypes []string
		expectMatch    bool
	}{
		{
			name:           "exact equipment match",
			alertLabels:    map[string]string{"equipment_type": "router"},
			equipmentTypes: []string{"router"},
			expectMatch:    true,
		},
		{
			name:           "device_type label match",
			alertLabels:    map[string]string{"device_type": "switch"},
			equipmentTypes: []string{"switch"},
			expectMatch:    true,
		},
		{
			name:           "wildcard equipment match",
			alertLabels:    map[string]string{"equipment_type": "core-router"},
			equipmentTypes: []string{"core-*"},
			expectMatch:    true,
		},
		{
			name:           "no equipment match",
			alertLabels:    map[string]string{"equipment_type": "firewall"},
			equipmentTypes: []string{"router", "switch"},
			expectMatch:    false,
		},
		{
			name:           "no equipment label",
			alertLabels:    map[string]string{"service": "my-service"},
			equipmentTypes: []string{"router"},
			expectMatch:    false,
		},
		{
			name:           "empty equipment types",
			alertLabels:    map[string]string{"equipment_type": "router"},
			equipmentTypes: nil,
			expectMatch:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			alert := &routingv1.Alert{
				Id:     "alert-1",
				Labels: tc.alertLabels,
			}

			result := matcher.MatchEquipment(alert, tc.equipmentTypes)

			if result.Matched != tc.expectMatch {
				t.Errorf("expected matched=%v, got matched=%v", tc.expectMatch, result.Matched)
			}

			if tc.expectMatch && result.MatchType != MatchTypeEquipment {
				t.Errorf("expected match type %s, got %s", MatchTypeEquipment, result.MatchType)
			}
		})
	}
}

func TestMatcher_Match_PriorityOrder(t *testing.T) {
	matcher := NewMatcher()

	// Alert that matches both site and service
	alert := &routingv1.Alert{
		Id:        "alert-1",
		ServiceId: "api-gateway",
		Labels: map[string]string{
			"site":    "us-east-1",
			"service": "api-gateway",
		},
	}

	// Window with both site and service scope
	window := &routingv1.MaintenanceWindow{
		Id:               "window-1",
		AffectedSites:    []string{"us-east-1"},
		AffectedServices: []string{"api-gateway"},
	}

	result := matcher.Match(alert, window)

	if !result.Matched {
		t.Error("expected match")
	}

	// Site matching should take precedence
	if result.MatchType != MatchTypeSite {
		t.Errorf("expected site match to take precedence, got %s", result.MatchType)
	}
}
