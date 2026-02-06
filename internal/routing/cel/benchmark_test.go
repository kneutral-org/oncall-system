package cel

import (
	"fmt"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

func BenchmarkEvaluator_SimpleExpression(b *testing.B) {
	eval, err := NewEvaluator()
	if err != nil {
		b.Fatal(err)
	}

	alert := &routingv1.Alert{
		Labels: map[string]string{
			"severity":    "critical",
			"environment": "production",
		},
	}

	expression := `alert_labels["severity"] == "critical"`

	// Warm up cache
	_, _ = eval.EvaluateExpression(expression, alert, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := eval.EvaluateExpression(expression, alert, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluator_ComplexExpression(b *testing.B) {
	eval, err := NewEvaluator()
	if err != nil {
		b.Fatal(err)
	}

	alert := &routingv1.Alert{
		Labels: map[string]string{
			"severity":    "critical",
			"environment": "production",
			"team":        "platform",
			"region":      "us-east-1",
		},
	}

	expression := `labelEquals(alert_labels, "severity", "critical") && hasLabel(alert_labels, "environment") && severityAtLeast(alert_labels["severity"], "high")`

	// Warm up cache
	_, _ = eval.EvaluateExpression(expression, alert, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := eval.EvaluateExpression(expression, alert, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluator_RegexExpression(b *testing.B) {
	eval, err := NewEvaluator()
	if err != nil {
		b.Fatal(err)
	}

	alert := &routingv1.Alert{
		Labels: map[string]string{
			"host": "web-prod-001.example.com",
		},
	}

	expression := `matches(alert_labels["host"], "^web-prod-[0-9]+\\.example\\.com$")`

	// Warm up cache
	_, _ = eval.EvaluateExpression(expression, alert, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := eval.EvaluateExpression(expression, alert, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluator_WithSiteContext(b *testing.B) {
	eval, err := NewEvaluator()
	if err != nil {
		b.Fatal(err)
	}

	alert := &routingv1.Alert{
		Labels: map[string]string{
			"severity": "critical",
		},
	}

	ctx := &EvalContext{
		Site: &routingv1.Site{
			Id:       "site-001",
			Name:     "East DC",
			Code:     "dc-east-1",
			Type:     routingv1.SiteType_SITE_TYPE_DATACENTER,
			Region:   "us-east",
			Tier:     1,
			Timezone: "America/New_York",
		},
		Now: time.Now(),
	}

	expression := `site_tier == 1 && labelEquals(alert_labels, "severity", "critical")`

	// Warm up cache
	_, _ = eval.EvaluateExpression(expression, alert, ctx)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := eval.EvaluateExpression(expression, alert, ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluator_CacheMiss(b *testing.B) {
	eval, err := NewEvaluator()
	if err != nil {
		b.Fatal(err)
	}

	alert := &routingv1.Alert{
		Labels: map[string]string{
			"severity": "critical",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use unique expression each time to force cache miss
		expression := fmt.Sprintf(`alert_labels["severity"] == "critical" || %d > 0`, i)
		_, err := eval.EvaluateExpression(expression, alert, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEvaluator_BatchEvaluate(b *testing.B) {
	eval, err := NewEvaluator()
	if err != nil {
		b.Fatal(err)
	}

	alert := &routingv1.Alert{
		Labels: map[string]string{
			"severity":    "critical",
			"environment": "production",
			"team":        "platform",
		},
	}

	expressions := []string{
		`alert_labels["severity"] == "critical"`,
		`hasLabel(alert_labels, "environment")`,
		`labelEquals(alert_labels, "team", "platform")`,
		`severityAtLeast(alert_labels["severity"], "high")`,
		`startsWith(alert_labels["environment"], "prod")`,
	}

	// Warm up cache
	_ = eval.BatchEvaluate(expressions, alert, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = eval.BatchEvaluate(expressions, alert, nil)
	}
}

func BenchmarkCache_GetOrCompile(b *testing.B) {
	cache, err := NewCache(1000)
	if err != nil {
		b.Fatal(err)
	}

	expression := `alert_labels["severity"] == "critical"`

	// Warm up
	_, _ = cache.GetOrCompile(expression)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cache.GetOrCompile(expression)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCache_ConcurrentGetOrCompile(b *testing.B) {
	cache, err := NewCache(1000)
	if err != nil {
		b.Fatal(err)
	}

	expressions := []string{
		`alert_labels["a"] == "1"`,
		`alert_labels["b"] == "2"`,
		`alert_labels["c"] == "3"`,
		`alert_labels["d"] == "4"`,
		`alert_labels["e"] == "5"`,
	}

	// Warm up
	for _, expr := range expressions {
		_, _ = cache.GetOrCompile(expr)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			expr := expressions[i%len(expressions)]
			_, err := cache.GetOrCompile(expr)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

func BenchmarkBuildActivation(b *testing.B) {
	alert := &routingv1.Alert{
		Id:        "alert-123",
		Summary:   "Test alert",
		Details:   "Test details",
		ServiceId: "service-001",
		Source:    routingv1.AlertSource_ALERT_SOURCE_PROMETHEUS,
		Labels: map[string]string{
			"severity":    "critical",
			"environment": "production",
			"team":        "platform",
			"region":      "us-east-1",
		},
		Annotations: map[string]string{
			"runbook": "https://runbooks.example.com",
		},
		CreatedAt: timestamppb.New(time.Now()),
	}

	ctx := &EvalContext{
		Site: &routingv1.Site{
			Id:       "site-001",
			Name:     "East DC",
			Code:     "dc-east-1",
			Type:     routingv1.SiteType_SITE_TYPE_DATACENTER,
			Region:   "us-east",
			Tier:     1,
			Timezone: "America/New_York",
		},
		Customer: &routingv1.CustomerTier{
			Id:    "tier-001",
			Name:  "Enterprise",
			Level: 1,
		},
		Now: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildActivation(alert, ctx)
	}
}
