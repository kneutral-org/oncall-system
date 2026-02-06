// Package routing provides the routing engine for the alerting system.
package routing

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kneutral-org/alerting-system/internal/routing/cel"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// Evaluator evaluates routing conditions against alerts.
type Evaluator struct {
	// celEvaluator handles CEL expression evaluation
	celEvaluator *cel.Evaluator
}

// NewEvaluator creates a new condition evaluator.
func NewEvaluator() *Evaluator {
	celEval, _ := cel.NewEvaluator()
	return &Evaluator{
		celEvaluator: celEval,
	}
}

// NewEvaluatorWithCEL creates a new evaluator with a custom CEL evaluator.
func NewEvaluatorWithCEL(celEval *cel.Evaluator) *Evaluator {
	return &Evaluator{
		celEvaluator: celEval,
	}
}

// CELEvaluator returns the CEL evaluator instance.
func (e *Evaluator) CELEvaluator() *cel.Evaluator {
	return e.celEvaluator
}

// EvaluateResult represents the result of evaluating a single condition.
type EvaluateResult struct {
	Matched  bool
	Expected string
	Actual   string
	Reason   string
}

// EvaluateCondition evaluates a single condition against an alert.
func (e *Evaluator) EvaluateCondition(cond *routingv1.RoutingCondition, alert *routingv1.Alert) *routingv1.ConditionResult {
	result := &routingv1.ConditionResult{
		Type:     cond.Type,
		Field:    cond.Field,
		Expected: e.getExpectedValue(cond),
		Matched:  false,
	}

	switch cond.Type {
	case routingv1.ConditionType_CONDITION_TYPE_LABEL:
		result.Actual, result.Matched = e.evaluateLabelCondition(cond, alert)

	case routingv1.ConditionType_CONDITION_TYPE_ANNOTATION:
		result.Actual, result.Matched = e.evaluateAnnotationCondition(cond, alert)

	case routingv1.ConditionType_CONDITION_TYPE_SEVERITY:
		result.Actual, result.Matched = e.evaluateSeverityCondition(cond, alert)

	case routingv1.ConditionType_CONDITION_TYPE_SOURCE:
		result.Actual, result.Matched = e.evaluateSourceCondition(cond, alert)

	case routingv1.ConditionType_CONDITION_TYPE_SERVICE:
		result.Actual, result.Matched = e.evaluateServiceCondition(cond, alert)

	case routingv1.ConditionType_CONDITION_TYPE_SITE:
		result.Actual, result.Matched = e.evaluateSiteCondition(cond, alert)

	case routingv1.ConditionType_CONDITION_TYPE_POP:
		result.Actual, result.Matched = e.evaluatePOPCondition(cond, alert)

	case routingv1.ConditionType_CONDITION_TYPE_CUSTOMER_TIER:
		result.Actual, result.Matched = e.evaluateCustomerTierCondition(cond, alert)

	case routingv1.ConditionType_CONDITION_TYPE_EQUIPMENT_TYPE:
		result.Actual, result.Matched = e.evaluateEquipmentTypeCondition(cond, alert)

	case routingv1.ConditionType_CONDITION_TYPE_CARRIER:
		result.Actual, result.Matched = e.evaluateCarrierCondition(cond, alert)

	case routingv1.ConditionType_CONDITION_TYPE_CEL:
		result.Actual, result.Matched = e.evaluateCELCondition(cond, alert)

	default:
		result.Actual = "unknown condition type"
		result.Matched = false
	}

	return result
}

// EvaluateRule evaluates all conditions of a rule against an alert.
// All conditions must match (AND logic).
func (e *Evaluator) EvaluateRule(rule *routingv1.RoutingRule, alert *routingv1.Alert, evaluateAt time.Time) *routingv1.RuleEvaluation {
	eval := &routingv1.RuleEvaluation{
		RuleId:   rule.Id,
		RuleName: rule.Name,
		Priority: rule.Priority,
		Matched:  true,
		Terminal: rule.Terminal,
	}

	// Check time condition first
	if rule.TimeCondition != nil {
		eval.TimeConditionMatched, eval.TimeConditionReason = e.evaluateTimeCondition(rule.TimeCondition, evaluateAt)
		if !eval.TimeConditionMatched {
			eval.Matched = false
			return eval
		}
	} else {
		eval.TimeConditionMatched = true
		eval.TimeConditionReason = "no time condition"
	}

	// Evaluate all conditions (AND logic)
	for i, cond := range rule.Conditions {
		condResult := e.EvaluateCondition(cond, alert)
		condResult.ConditionIndex = int32(i)
		eval.ConditionResults = append(eval.ConditionResults, condResult)

		if !condResult.Matched {
			eval.Matched = false
		}
	}

	return eval
}

// EvaluateRules evaluates multiple rules against an alert and returns matching rules.
func (e *Evaluator) EvaluateRules(rules []*routingv1.RoutingRule, alert *routingv1.Alert, evaluateAt time.Time) ([]*routingv1.RuleEvaluation, []*routingv1.RoutingAction) {
	var evaluations []*routingv1.RuleEvaluation
	var matchedActions []*routingv1.RoutingAction

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		eval := e.EvaluateRule(rule, alert, evaluateAt)
		evaluations = append(evaluations, eval)

		if eval.Matched {
			matchedActions = append(matchedActions, rule.Actions...)

			// If terminal, stop evaluating more rules
			if rule.Terminal {
				break
			}
		}
	}

	return evaluations, matchedActions
}

// getExpectedValue returns a string representation of the expected value for logging.
func (e *Evaluator) getExpectedValue(cond *routingv1.RoutingCondition) string {
	switch cond.Operator {
	case routingv1.ConditionOperator_CONDITION_OPERATOR_IN,
		routingv1.ConditionOperator_CONDITION_OPERATOR_NOT_IN:
		return strings.Join(cond.StringList, ", ")
	case routingv1.ConditionOperator_CONDITION_OPERATOR_REGEX:
		return cond.RegexPattern
	case routingv1.ConditionOperator_CONDITION_OPERATOR_EXISTS,
		routingv1.ConditionOperator_CONDITION_OPERATOR_NOT_EXISTS:
		return "exists"
	default:
		return cond.StringValue
	}
}

// evaluateLabelCondition evaluates a label-based condition.
func (e *Evaluator) evaluateLabelCondition(cond *routingv1.RoutingCondition, alert *routingv1.Alert) (string, bool) {
	labelValue, exists := alert.Labels[cond.Field]
	if !exists {
		if cond.Operator == routingv1.ConditionOperator_CONDITION_OPERATOR_NOT_EXISTS {
			return "", true
		}
		if cond.Operator == routingv1.ConditionOperator_CONDITION_OPERATOR_EXISTS {
			return "", false
		}
		return "", false
	}

	return labelValue, e.compareValue(cond.Operator, labelValue, cond)
}

// evaluateAnnotationCondition evaluates an annotation-based condition.
func (e *Evaluator) evaluateAnnotationCondition(cond *routingv1.RoutingCondition, alert *routingv1.Alert) (string, bool) {
	annotationValue, exists := alert.Annotations[cond.Field]
	if !exists {
		if cond.Operator == routingv1.ConditionOperator_CONDITION_OPERATOR_NOT_EXISTS {
			return "", true
		}
		if cond.Operator == routingv1.ConditionOperator_CONDITION_OPERATOR_EXISTS {
			return "", false
		}
		return "", false
	}

	return annotationValue, e.compareValue(cond.Operator, annotationValue, cond)
}

// evaluateSeverityCondition evaluates a severity-based condition.
func (e *Evaluator) evaluateSeverityCondition(cond *routingv1.RoutingCondition, alert *routingv1.Alert) (string, bool) {
	// Severity is typically stored as a label
	severity := alert.Labels["severity"]
	if severity == "" {
		severity = "unknown"
	}

	return severity, e.compareValue(cond.Operator, severity, cond)
}

// evaluateSourceCondition evaluates a source-based condition.
func (e *Evaluator) evaluateSourceCondition(cond *routingv1.RoutingCondition, alert *routingv1.Alert) (string, bool) {
	source := alert.Source.String()
	return source, e.compareValue(cond.Operator, source, cond)
}

// evaluateServiceCondition evaluates a service-based condition.
func (e *Evaluator) evaluateServiceCondition(cond *routingv1.RoutingCondition, alert *routingv1.Alert) (string, bool) {
	return alert.ServiceId, e.compareValue(cond.Operator, alert.ServiceId, cond)
}

// evaluateSiteCondition evaluates a site-based condition.
func (e *Evaluator) evaluateSiteCondition(cond *routingv1.RoutingCondition, alert *routingv1.Alert) (string, bool) {
	site := alert.Labels["site"]
	if site == "" {
		site = alert.Labels["datacenter"]
	}
	return site, e.compareValue(cond.Operator, site, cond)
}

// evaluatePOPCondition evaluates a POP-based condition.
func (e *Evaluator) evaluatePOPCondition(cond *routingv1.RoutingCondition, alert *routingv1.Alert) (string, bool) {
	pop := alert.Labels["pop"]
	return pop, e.compareValue(cond.Operator, pop, cond)
}

// evaluateCustomerTierCondition evaluates a customer tier-based condition.
func (e *Evaluator) evaluateCustomerTierCondition(cond *routingv1.RoutingCondition, alert *routingv1.Alert) (string, bool) {
	tier := alert.Labels["customer_tier"]
	if tier == "" {
		tier = alert.Labels["tier"]
	}
	return tier, e.compareValue(cond.Operator, tier, cond)
}

// evaluateEquipmentTypeCondition evaluates an equipment type-based condition.
func (e *Evaluator) evaluateEquipmentTypeCondition(cond *routingv1.RoutingCondition, alert *routingv1.Alert) (string, bool) {
	equipType := alert.Labels["equipment_type"]
	if equipType == "" {
		equipType = alert.Labels["device_type"]
	}
	return equipType, e.compareValue(cond.Operator, equipType, cond)
}

// evaluateCarrierCondition evaluates a carrier-based condition.
func (e *Evaluator) evaluateCarrierCondition(cond *routingv1.RoutingCondition, alert *routingv1.Alert) (string, bool) {
	carrier := alert.Labels["carrier"]
	if carrier == "" {
		carrier = alert.Labels["asn"]
	}
	return carrier, e.compareValue(cond.Operator, carrier, cond)
}

// evaluateCELCondition evaluates a CEL expression condition.
func (e *Evaluator) evaluateCELCondition(cond *routingv1.RoutingCondition, alert *routingv1.Alert) (string, bool) {
	expression := cond.CelExpression
	if expression == "" {
		return "empty CEL expression", false
	}

	if e.celEvaluator == nil {
		return "CEL evaluator not initialized", false
	}

	// Create evaluation context (no site/customer context for basic evaluation)
	ctx := &cel.EvalContext{
		Now: time.Now(),
	}

	matched, err := e.celEvaluator.EvaluateExpression(expression, alert, ctx)
	if err != nil {
		return err.Error(), false
	}

	if matched {
		return "CEL expression matched", true
	}
	return "CEL expression did not match", false
}

// EvaluateCELWithContext evaluates a CEL expression with full context.
func (e *Evaluator) EvaluateCELWithContext(expression string, alert *routingv1.Alert, ctx *cel.EvalContext) (bool, error) {
	if e.celEvaluator == nil {
		return false, cel.ErrEvaluationFailed
	}
	return e.celEvaluator.EvaluateExpression(expression, alert, ctx)
}

// ValidateCELExpression validates a CEL expression without evaluating it.
func (e *Evaluator) ValidateCELExpression(expression string) error {
	if e.celEvaluator == nil {
		return cel.ErrEvaluationFailed
	}
	return e.celEvaluator.Validate(expression)
}

// compareValue compares a value using the specified operator.
func (e *Evaluator) compareValue(op routingv1.ConditionOperator, actual string, cond *routingv1.RoutingCondition) bool {
	expected := cond.StringValue

	switch op {
	case routingv1.ConditionOperator_CONDITION_OPERATOR_EQUALS:
		return actual == expected

	case routingv1.ConditionOperator_CONDITION_OPERATOR_NOT_EQUALS:
		return actual != expected

	case routingv1.ConditionOperator_CONDITION_OPERATOR_CONTAINS:
		return strings.Contains(actual, expected)

	case routingv1.ConditionOperator_CONDITION_OPERATOR_NOT_CONTAINS:
		return !strings.Contains(actual, expected)

	case routingv1.ConditionOperator_CONDITION_OPERATOR_STARTS_WITH:
		return strings.HasPrefix(actual, expected)

	case routingv1.ConditionOperator_CONDITION_OPERATOR_ENDS_WITH:
		return strings.HasSuffix(actual, expected)

	case routingv1.ConditionOperator_CONDITION_OPERATOR_REGEX:
		pattern := cond.RegexPattern
		if pattern == "" {
			pattern = expected
		}
		matched, err := regexp.MatchString(pattern, actual)
		return err == nil && matched

	case routingv1.ConditionOperator_CONDITION_OPERATOR_IN:
		for _, v := range cond.StringList {
			if actual == v {
				return true
			}
		}
		return false

	case routingv1.ConditionOperator_CONDITION_OPERATOR_NOT_IN:
		for _, v := range cond.StringList {
			if actual == v {
				return false
			}
		}
		return true

	case routingv1.ConditionOperator_CONDITION_OPERATOR_EXISTS:
		return actual != ""

	case routingv1.ConditionOperator_CONDITION_OPERATOR_NOT_EXISTS:
		return actual == ""

	case routingv1.ConditionOperator_CONDITION_OPERATOR_GREATER_THAN:
		actualInt, err1 := strconv.ParseInt(actual, 10, 64)
		expectedInt, err2 := strconv.ParseInt(expected, 10, 64)
		if err1 != nil || err2 != nil {
			return actual > expected // String comparison as fallback
		}
		return actualInt > expectedInt

	case routingv1.ConditionOperator_CONDITION_OPERATOR_LESS_THAN:
		actualInt, err1 := strconv.ParseInt(actual, 10, 64)
		expectedInt, err2 := strconv.ParseInt(expected, 10, 64)
		if err1 != nil || err2 != nil {
			return actual < expected // String comparison as fallback
		}
		return actualInt < expectedInt

	default:
		return false
	}
}

// evaluateTimeCondition evaluates time-based conditions.
func (e *Evaluator) evaluateTimeCondition(tc *routingv1.TimeCondition, evaluateAt time.Time) (bool, string) {
	if len(tc.Windows) == 0 {
		return true, "no time windows defined"
	}

	// Parse timezone
	loc, err := time.LoadLocation(tc.Timezone)
	if err != nil {
		loc = time.UTC
	}

	localTime := evaluateAt.In(loc)

	for _, window := range tc.Windows {
		matched := e.isInTimeWindow(localTime, window)
		if window.Invert {
			matched = !matched
		}
		if matched {
			return true, "matched time window"
		}
	}

	return false, "no time window matched"
}

// isInTimeWindow checks if a time falls within a time window.
func (e *Evaluator) isInTimeWindow(t time.Time, window *routingv1.TimeWindow) bool {
	// Check day of week
	dayOfWeek := int32(t.Weekday())
	dayMatches := false

	if len(window.DaysOfWeek) == 0 {
		dayMatches = true
	} else {
		for _, day := range window.DaysOfWeek {
			if day == dayOfWeek {
				dayMatches = true
				break
			}
		}
	}

	if !dayMatches {
		return false
	}

	// Parse start and end times
	startParts := strings.Split(window.StartTime, ":")
	endParts := strings.Split(window.EndTime, ":")

	if len(startParts) != 2 || len(endParts) != 2 {
		return false
	}

	startHour, _ := strconv.Atoi(startParts[0])
	startMin, _ := strconv.Atoi(startParts[1])
	endHour, _ := strconv.Atoi(endParts[0])
	endMin, _ := strconv.Atoi(endParts[1])

	currentMinutes := t.Hour()*60 + t.Minute()
	startMinutes := startHour*60 + startMin
	endMinutes := endHour*60 + endMin

	// Handle overnight windows (e.g., 22:00 - 06:00)
	if endMinutes < startMinutes {
		return currentMinutes >= startMinutes || currentMinutes < endMinutes
	}

	return currentMinutes >= startMinutes && currentMinutes < endMinutes
}

// SeverityLevel converts a severity string to a numeric level for comparison.
func SeverityLevel(severity string) int {
	switch strings.ToLower(severity) {
	case "critical", "fatal", "p1":
		return 5
	case "high", "error", "p2":
		return 4
	case "warning", "warn", "medium", "p3":
		return 3
	case "info", "low", "p4":
		return 2
	case "debug", "p5":
		return 1
	default:
		return 0
	}
}

// CompareSeverity compares two severity levels.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func CompareSeverity(a, b string) int {
	levelA := SeverityLevel(a)
	levelB := SeverityLevel(b)

	if levelA < levelB {
		return -1
	}
	if levelA > levelB {
		return 1
	}
	return 0
}
