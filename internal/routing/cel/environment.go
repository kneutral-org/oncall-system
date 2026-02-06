// Package cel provides CEL expression evaluation for alert routing conditions.
package cel

import (
	"fmt"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"google.golang.org/protobuf/types/known/timestamppb"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// AlertVars represents the alert variables exposed to CEL expressions.
type AlertVars struct {
	Labels       map[string]string
	Annotations  map[string]string
	Severity     string
	StartsAt     time.Time
	EndsAt       time.Time
	GeneratorURL string
	ID           string
	Summary      string
	Details      string
	ServiceID    string
	Source       string
	Fingerprint  string
}

// SiteVars represents the site variables exposed to CEL expressions.
type SiteVars struct {
	ID        string
	Name      string
	Code      string
	Type      string
	Region    string
	Country   string
	City      string
	Tier      int32
	Timezone  string
	Metadata  map[string]string
	TeamID    string
	Available bool
}

// CustomerVars represents the customer variables exposed to CEL expressions.
type CustomerVars struct {
	ID       string
	Name     string
	Tier     int32
	Metadata map[string]string
}

// EvalContext provides context data for CEL expression evaluation.
type EvalContext struct {
	Site     *routingv1.Site
	Customer *routingv1.CustomerTier
	Now      time.Time
}

// NewEnvironment creates a new CEL environment with predefined variables and functions.
func NewEnvironment() (*cel.Env, error) {
	return cel.NewEnv(
		// Alert variable declarations
		cel.Variable("alert", cel.ObjectType("AlertVars")),

		// Site variable (optional)
		cel.Variable("site", cel.ObjectType("SiteVars")),

		// Customer variable (optional)
		cel.Variable("customer", cel.ObjectType("CustomerVars")),

		// Current timestamp
		cel.Variable("now", cel.TimestampType),

		// Custom types for our domain objects
		cel.Types(&AlertVars{}, &SiteVars{}, &CustomerVars{}),
	)
}

// NewStandardEnvironment creates a CEL environment with standard alert routing variables.
func NewStandardEnvironment() (*cel.Env, error) {
	return cel.NewEnv(
		// Alert fields
		cel.Variable("alert_labels", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("alert_annotations", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("alert_severity", cel.StringType),
		cel.Variable("alert_starts_at", cel.TimestampType),
		cel.Variable("alert_ends_at", cel.TimestampType),
		cel.Variable("alert_generator_url", cel.StringType),
		cel.Variable("alert_id", cel.StringType),
		cel.Variable("alert_summary", cel.StringType),
		cel.Variable("alert_details", cel.StringType),
		cel.Variable("alert_service_id", cel.StringType),
		cel.Variable("alert_source", cel.StringType),
		cel.Variable("alert_fingerprint", cel.StringType),

		// Site fields (optional)
		cel.Variable("site_id", cel.StringType),
		cel.Variable("site_name", cel.StringType),
		cel.Variable("site_code", cel.StringType),
		cel.Variable("site_type", cel.StringType),
		cel.Variable("site_region", cel.StringType),
		cel.Variable("site_country", cel.StringType),
		cel.Variable("site_city", cel.StringType),
		cel.Variable("site_tier", cel.IntType),
		cel.Variable("site_timezone", cel.StringType),
		cel.Variable("site_metadata", cel.MapType(cel.StringType, cel.StringType)),
		cel.Variable("site_available", cel.BoolType),

		// Customer fields (optional)
		cel.Variable("customer_id", cel.StringType),
		cel.Variable("customer_name", cel.StringType),
		cel.Variable("customer_tier", cel.IntType),
		cel.Variable("customer_metadata", cel.MapType(cel.StringType, cel.StringType)),

		// Current timestamp
		cel.Variable("now", cel.TimestampType),

		// Register custom functions
		RegisterCustomFunctions(),
	)
}

// BuildActivation creates a CEL activation map from alert and context data.
func BuildActivation(alert *routingv1.Alert, ctx *EvalContext) map[string]interface{} {
	activation := make(map[string]interface{})

	// Alert fields
	if alert != nil {
		activation["alert_labels"] = convertToRefMap(alert.Labels)
		activation["alert_annotations"] = convertToRefMap(alert.Annotations)
		activation["alert_severity"] = getSeverityString(alert)
		activation["alert_id"] = alert.Id
		activation["alert_summary"] = alert.Summary
		activation["alert_details"] = alert.Details
		activation["alert_service_id"] = alert.ServiceId
		activation["alert_source"] = alert.Source.String()
		activation["alert_fingerprint"] = alert.Fingerprint

		// Timestamps
		if alert.CreatedAt != nil {
			activation["alert_starts_at"] = types.Timestamp{Time: alert.CreatedAt.AsTime()}
		} else {
			activation["alert_starts_at"] = types.Timestamp{Time: time.Time{}}
		}
		activation["alert_ends_at"] = types.Timestamp{Time: time.Time{}}
		activation["alert_generator_url"] = getGeneratorURL(alert)
	} else {
		activation["alert_labels"] = map[string]string{}
		activation["alert_annotations"] = map[string]string{}
		activation["alert_severity"] = ""
		activation["alert_id"] = ""
		activation["alert_summary"] = ""
		activation["alert_details"] = ""
		activation["alert_service_id"] = ""
		activation["alert_source"] = ""
		activation["alert_fingerprint"] = ""
		activation["alert_starts_at"] = types.Timestamp{Time: time.Time{}}
		activation["alert_ends_at"] = types.Timestamp{Time: time.Time{}}
		activation["alert_generator_url"] = ""
	}

	// Context timestamp
	now := time.Now()
	if ctx != nil && !ctx.Now.IsZero() {
		now = ctx.Now
	}
	activation["now"] = types.Timestamp{Time: now}

	// Site fields
	if ctx != nil && ctx.Site != nil {
		activation["site_id"] = ctx.Site.Id
		activation["site_name"] = ctx.Site.Name
		activation["site_code"] = ctx.Site.Code
		activation["site_type"] = ctx.Site.Type.String()
		activation["site_region"] = ctx.Site.Region
		activation["site_country"] = ctx.Site.Country
		activation["site_city"] = ctx.Site.City
		activation["site_tier"] = int64(ctx.Site.Tier)
		activation["site_timezone"] = ctx.Site.Timezone
		activation["site_metadata"] = convertToRefMap(ctx.Site.Metadata)
		activation["site_available"] = true
	} else {
		activation["site_id"] = ""
		activation["site_name"] = ""
		activation["site_code"] = ""
		activation["site_type"] = ""
		activation["site_region"] = ""
		activation["site_country"] = ""
		activation["site_city"] = ""
		activation["site_tier"] = int64(0)
		activation["site_timezone"] = ""
		activation["site_metadata"] = map[string]string{}
		activation["site_available"] = false
	}

	// Customer fields
	if ctx != nil && ctx.Customer != nil {
		activation["customer_id"] = ctx.Customer.Id
		activation["customer_name"] = ctx.Customer.Name
		activation["customer_tier"] = int64(ctx.Customer.Level)
		activation["customer_metadata"] = convertToRefMap(ctx.Customer.Metadata)
	} else {
		activation["customer_id"] = ""
		activation["customer_name"] = ""
		activation["customer_tier"] = int64(0)
		activation["customer_metadata"] = map[string]string{}
	}

	return activation
}

// convertToRefMap converts a Go map to a CEL-compatible map.
func convertToRefMap(m map[string]string) ref.Val {
	if m == nil {
		return types.NewStringStringMap(types.DefaultTypeAdapter, map[string]string{})
	}
	return types.NewStringStringMap(types.DefaultTypeAdapter, m)
}

// getSeverityString extracts the severity from an alert.
func getSeverityString(alert *routingv1.Alert) string {
	if alert == nil || alert.Labels == nil {
		return ""
	}
	if sev, ok := alert.Labels["severity"]; ok {
		return sev
	}
	return ""
}

// getGeneratorURL extracts the generator URL from an alert.
func getGeneratorURL(alert *routingv1.Alert) string {
	if alert == nil || alert.Labels == nil {
		return ""
	}
	if url, ok := alert.Labels["generatorURL"]; ok {
		return url
	}
	return ""
}

// AlertFromProto converts a routingv1.Alert to AlertVars for CEL evaluation.
func AlertFromProto(alert *routingv1.Alert) *AlertVars {
	if alert == nil {
		return &AlertVars{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		}
	}

	vars := &AlertVars{
		ID:          alert.Id,
		Summary:     alert.Summary,
		Details:     alert.Details,
		ServiceID:   alert.ServiceId,
		Source:      alert.Source.String(),
		Fingerprint: alert.Fingerprint,
		Labels:      alert.Labels,
		Annotations: alert.Annotations,
	}

	if vars.Labels == nil {
		vars.Labels = make(map[string]string)
	}
	if vars.Annotations == nil {
		vars.Annotations = make(map[string]string)
	}

	// Extract severity from labels
	if sev, ok := vars.Labels["severity"]; ok {
		vars.Severity = sev
	}

	// Extract generator URL from labels
	if url, ok := vars.Labels["generatorURL"]; ok {
		vars.GeneratorURL = url
	}

	// Extract timestamps from alert
	if alert.CreatedAt != nil {
		vars.StartsAt = alert.CreatedAt.AsTime()
	}

	return vars
}

// SiteFromProto converts a routingv1.Site to SiteVars for CEL evaluation.
func SiteFromProto(site *routingv1.Site) *SiteVars {
	if site == nil {
		return &SiteVars{
			Available: false,
			Metadata:  make(map[string]string),
		}
	}

	vars := &SiteVars{
		ID:        site.Id,
		Name:      site.Name,
		Code:      site.Code,
		Type:      site.Type.String(),
		Region:    site.Region,
		Country:   site.Country,
		City:      site.City,
		Tier:      site.Tier,
		Timezone:  site.Timezone,
		Metadata:  site.Metadata,
		TeamID:    site.PrimaryTeamId,
		Available: true,
	}

	if vars.Metadata == nil {
		vars.Metadata = make(map[string]string)
	}

	return vars
}

// CustomerFromProto converts a routingv1.CustomerTier to CustomerVars for CEL evaluation.
func CustomerFromProto(customer *routingv1.CustomerTier) *CustomerVars {
	if customer == nil {
		return &CustomerVars{
			Metadata: make(map[string]string),
		}
	}

	vars := &CustomerVars{
		ID:       customer.Id,
		Name:     customer.Name,
		Tier:     customer.Level,
		Metadata: customer.Metadata,
	}

	if vars.Metadata == nil {
		vars.Metadata = make(map[string]string)
	}

	return vars
}

// TimestampToProto converts a time.Time to a protobuf Timestamp.
func TimestampToProto(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// ValidateExpression validates a CEL expression against the standard environment.
func ValidateExpression(expression string) error {
	env, err := NewStandardEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create CEL environment: %w", err)
	}

	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("CEL compilation error: %w", issues.Err())
	}

	// Verify the expression returns a boolean
	if ast.OutputType() != cel.BoolType {
		return fmt.Errorf("CEL expression must return a boolean, got %s", ast.OutputType())
	}

	return nil
}
