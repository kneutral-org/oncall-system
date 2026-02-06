// Package maintenance provides maintenance window management for the alerting system.
package maintenance

import (
	"fmt"
	"regexp"
	"strings"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// MatchType indicates how an alert matched a maintenance window.
type MatchType string

const (
	// MatchTypeSite indicates the alert matched by site.
	MatchTypeSite MatchType = "site"
	// MatchTypeService indicates the alert matched by service.
	MatchTypeService MatchType = "service"
	// MatchTypeLabel indicates the alert matched by label.
	MatchTypeLabel MatchType = "label"
	// MatchTypeEquipment indicates the alert matched by equipment type.
	MatchTypeEquipment MatchType = "equipment"
	// MatchTypeGlobal indicates the maintenance window applies globally (no scope).
	MatchTypeGlobal MatchType = "global"
)

// LabelOperator defines the operator for label matching.
type LabelOperator string

const (
	// OperatorEqual matches if the label value equals the expected value.
	OperatorEqual LabelOperator = "equal"
	// OperatorNotEqual matches if the label value does not equal the expected value.
	OperatorNotEqual LabelOperator = "not_equal"
	// OperatorRegex matches if the label value matches the regex pattern.
	OperatorRegex LabelOperator = "regex"
	// OperatorNotRegex matches if the label value does not match the regex pattern.
	OperatorNotRegex LabelOperator = "not_regex"
	// OperatorExists matches if the label exists.
	OperatorExists LabelOperator = "exists"
	// OperatorNotExists matches if the label does not exist.
	OperatorNotExists LabelOperator = "not_exists"
)

// LabelMatcher defines a label matching rule.
type LabelMatcher struct {
	Name     string
	Value    string
	Operator LabelOperator
}

// Matcher checks if alerts match maintenance window scopes.
type Matcher struct {
	// compiledRegexCache caches compiled regex patterns
	compiledRegexCache map[string]*regexp.Regexp
}

// NewMatcher creates a new Matcher.
func NewMatcher() *Matcher {
	return &Matcher{
		compiledRegexCache: make(map[string]*regexp.Regexp),
	}
}

// MatchResult contains the result of matching an alert against a maintenance window.
type MatchResult struct {
	Matched   bool
	MatchType MatchType
	Reason    string
	Details   map[string]string
}

// Match checks if an alert matches a maintenance window's scope.
func (m *Matcher) Match(alert *routingv1.Alert, window *routingv1.MaintenanceWindow) *MatchResult {
	// If no scope is defined, the window applies globally
	if len(window.AffectedSites) == 0 &&
		len(window.AffectedServices) == 0 &&
		len(window.AffectedLabels) == 0 {
		return &MatchResult{
			Matched:   true,
			MatchType: MatchTypeGlobal,
			Reason:    "maintenance window applies globally (no scope defined)",
		}
	}

	// Check site matching
	if result := m.matchSites(alert, window); result.Matched {
		return result
	}

	// Check service matching
	if result := m.matchServices(alert, window); result.Matched {
		return result
	}

	// Check label matching
	if result := m.matchLabels(alert, window); result.Matched {
		return result
	}

	return &MatchResult{
		Matched: false,
		Reason:  "alert does not match maintenance window scope",
	}
}

// matchSites checks if the alert's site matches any of the window's affected sites.
func (m *Matcher) matchSites(alert *routingv1.Alert, window *routingv1.MaintenanceWindow) *MatchResult {
	if len(window.AffectedSites) == 0 {
		return &MatchResult{Matched: false}
	}

	alertSite := getAlertSite(alert)
	if alertSite == "" {
		return &MatchResult{Matched: false}
	}

	for _, site := range window.AffectedSites {
		if matchesPattern(alertSite, site) {
			return &MatchResult{
				Matched:   true,
				MatchType: MatchTypeSite,
				Reason:    "alert site matches maintenance window site",
				Details: map[string]string{
					"alertSite":  alertSite,
					"windowSite": site,
				},
			}
		}
	}

	return &MatchResult{Matched: false}
}

// matchServices checks if the alert's service matches any of the window's affected services.
func (m *Matcher) matchServices(alert *routingv1.Alert, window *routingv1.MaintenanceWindow) *MatchResult {
	if len(window.AffectedServices) == 0 {
		return &MatchResult{Matched: false}
	}

	alertService := getAlertService(alert)
	if alertService == "" {
		return &MatchResult{Matched: false}
	}

	for _, service := range window.AffectedServices {
		if matchesPattern(alertService, service) {
			return &MatchResult{
				Matched:   true,
				MatchType: MatchTypeService,
				Reason:    "alert service matches maintenance window service",
				Details: map[string]string{
					"alertService":  alertService,
					"windowService": service,
				},
			}
		}
	}

	return &MatchResult{Matched: false}
}

// matchLabels checks if the alert's labels match the window's affected labels.
func (m *Matcher) matchLabels(alert *routingv1.Alert, window *routingv1.MaintenanceWindow) *MatchResult {
	if len(window.AffectedLabels) == 0 {
		return &MatchResult{Matched: false}
	}

	// Parse label matchers from the window
	matchers := parseLabelMatchers(window.AffectedLabels)

	// All matchers must match (AND logic)
	for _, matcher := range matchers {
		if !m.matchLabel(alert, matcher) {
			return &MatchResult{Matched: false}
		}
	}

	return &MatchResult{
		Matched:   true,
		MatchType: MatchTypeLabel,
		Reason:    "alert labels match maintenance window label matchers",
		Details: map[string]string{
			"matcherCount": fmt.Sprintf("%d", len(matchers)),
		},
	}
}

// matchLabel checks if a single label matcher matches the alert.
func (m *Matcher) matchLabel(alert *routingv1.Alert, matcher LabelMatcher) bool {
	labelValue, exists := alert.Labels[matcher.Name]

	switch matcher.Operator {
	case OperatorExists:
		return exists

	case OperatorNotExists:
		return !exists

	case OperatorEqual:
		return exists && labelValue == matcher.Value

	case OperatorNotEqual:
		return !exists || labelValue != matcher.Value

	case OperatorRegex:
		if !exists {
			return false
		}
		return m.matchRegex(labelValue, matcher.Value)

	case OperatorNotRegex:
		if !exists {
			return true
		}
		return !m.matchRegex(labelValue, matcher.Value)

	default:
		// Default to equality matching
		return exists && labelValue == matcher.Value
	}
}

// matchRegex matches a value against a regex pattern.
func (m *Matcher) matchRegex(value, pattern string) bool {
	// Check cache first
	if re, ok := m.compiledRegexCache[pattern]; ok {
		return re.MatchString(value)
	}

	// Compile and cache
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}

	m.compiledRegexCache[pattern] = re
	return re.MatchString(value)
}

// MatchEquipment checks if the alert's equipment type matches the maintenance window.
func (m *Matcher) MatchEquipment(alert *routingv1.Alert, equipmentTypes []string) *MatchResult {
	if len(equipmentTypes) == 0 {
		return &MatchResult{Matched: false}
	}

	alertEquipment := getAlertEquipmentType(alert)
	if alertEquipment == "" {
		return &MatchResult{Matched: false}
	}

	for _, equipType := range equipmentTypes {
		if matchesPattern(alertEquipment, equipType) {
			return &MatchResult{
				Matched:   true,
				MatchType: MatchTypeEquipment,
				Reason:    "alert equipment type matches maintenance window equipment",
				Details: map[string]string{
					"alertEquipment":  alertEquipment,
					"windowEquipment": equipType,
				},
			}
		}
	}

	return &MatchResult{Matched: false}
}

// Helper functions

// getAlertSite extracts the site identifier from an alert.
func getAlertSite(alert *routingv1.Alert) string {
	if alert.Labels == nil {
		return ""
	}

	// Try common site label names
	siteLabels := []string{"site", "datacenter", "dc", "location", "pop", "site_id"}
	for _, label := range siteLabels {
		if value, ok := alert.Labels[label]; ok && value != "" {
			return value
		}
	}

	return ""
}

// getAlertService extracts the service identifier from an alert.
func getAlertService(alert *routingv1.Alert) string {
	// First check the service_id field
	if alert.ServiceId != "" {
		return alert.ServiceId
	}

	if alert.Labels == nil {
		return ""
	}

	// Try common service label names
	serviceLabels := []string{"service", "service_name", "service_id", "app", "application"}
	for _, label := range serviceLabels {
		if value, ok := alert.Labels[label]; ok && value != "" {
			return value
		}
	}

	return ""
}

// getAlertEquipmentType extracts the equipment type from an alert.
func getAlertEquipmentType(alert *routingv1.Alert) string {
	if alert.Labels == nil {
		return ""
	}

	// Try common equipment type label names
	equipLabels := []string{"equipment_type", "device_type", "equipment", "device"}
	for _, label := range equipLabels {
		if value, ok := alert.Labels[label]; ok && value != "" {
			return value
		}
	}

	return ""
}

// matchesPattern checks if a value matches a pattern.
// Supports exact match, prefix match with *, suffix match with *, and full regex.
func matchesPattern(value, pattern string) bool {
	// Exact match
	if value == pattern {
		return true
	}

	// Case-insensitive exact match
	if strings.EqualFold(value, pattern) {
		return true
	}

	// Wildcard prefix match (e.g., "us-*" matches "us-east-1")
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}

	// Wildcard suffix match (e.g., "*-prod" matches "api-prod")
	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}

	// Full regex match (if pattern contains regex metacharacters)
	if containsRegexMeta(pattern) {
		matched, err := regexp.MatchString(pattern, value)
		return err == nil && matched
	}

	return false
}

// containsRegexMeta checks if a pattern contains regex metacharacters.
func containsRegexMeta(pattern string) bool {
	metaChars := []string{"^", "$", ".", "+", "?", "[", "]", "(", ")", "{", "}", "|", "\\"}
	for _, meta := range metaChars {
		if strings.Contains(pattern, meta) {
			return true
		}
	}
	return false
}

// parseLabelMatchers parses label matchers from affected_labels strings.
// Supports formats:
// - "key=value" (equality)
// - "key!=value" (not equal)
// - "key=~regex" (regex match)
// - "key!~regex" (regex not match)
// - "key" (exists)
// - "!key" (not exists)
func parseLabelMatchers(labels []string) []LabelMatcher {
	var matchers []LabelMatcher

	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}

		matcher := parseSingleLabelMatcher(label)
		matchers = append(matchers, matcher)
	}

	return matchers
}

// parseSingleLabelMatcher parses a single label matcher string.
func parseSingleLabelMatcher(label string) LabelMatcher {
	// Check for negation prefix (not exists)
	if strings.HasPrefix(label, "!") && !strings.Contains(label, "=") {
		return LabelMatcher{
			Name:     strings.TrimPrefix(label, "!"),
			Operator: OperatorNotExists,
		}
	}

	// Check for not-regex match (!~)
	if idx := strings.Index(label, "!~"); idx > 0 {
		return LabelMatcher{
			Name:     strings.TrimSpace(label[:idx]),
			Value:    strings.TrimSpace(label[idx+2:]),
			Operator: OperatorNotRegex,
		}
	}

	// Check for regex match (=~)
	if idx := strings.Index(label, "=~"); idx > 0 {
		return LabelMatcher{
			Name:     strings.TrimSpace(label[:idx]),
			Value:    strings.TrimSpace(label[idx+2:]),
			Operator: OperatorRegex,
		}
	}

	// Check for not-equal (!=)
	if idx := strings.Index(label, "!="); idx > 0 {
		return LabelMatcher{
			Name:     strings.TrimSpace(label[:idx]),
			Value:    strings.TrimSpace(label[idx+2:]),
			Operator: OperatorNotEqual,
		}
	}

	// Check for equal (=)
	if idx := strings.Index(label, "="); idx > 0 {
		return LabelMatcher{
			Name:     strings.TrimSpace(label[:idx]),
			Value:    strings.TrimSpace(label[idx+1:]),
			Operator: OperatorEqual,
		}
	}

	// No operator, just a key (exists)
	return LabelMatcher{
		Name:     label,
		Operator: OperatorExists,
	}
}
