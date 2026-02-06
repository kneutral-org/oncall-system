package action

import (
	"testing"
	"time"
)

func TestMetrics_RecordActionExecution(t *testing.T) {
	m := NewMetrics()

	// Record some executions
	m.RecordActionExecution("ACTION_TYPE_NOTIFY_TEAM", "success", 10*time.Millisecond)
	m.RecordActionExecution("ACTION_TYPE_NOTIFY_TEAM", "success", 20*time.Millisecond)
	m.RecordActionExecution("ACTION_TYPE_NOTIFY_TEAM", "failure", 5*time.Millisecond)
	m.RecordActionExecution("ACTION_TYPE_SUPPRESS", "success", 15*time.Millisecond)

	// Check totals
	if got := m.GetActionTotal("ACTION_TYPE_NOTIFY_TEAM", "success"); got != 2 {
		t.Errorf("GetActionTotal(NOTIFY_TEAM, success) = %d, want 2", got)
	}

	if got := m.GetActionTotal("ACTION_TYPE_NOTIFY_TEAM", "failure"); got != 1 {
		t.Errorf("GetActionTotal(NOTIFY_TEAM, failure) = %d, want 1", got)
	}

	if got := m.GetActionTotal("ACTION_TYPE_SUPPRESS", "success"); got != 1 {
		t.Errorf("GetActionTotal(SUPPRESS, success) = %d, want 1", got)
	}

	// Check non-existent
	if got := m.GetActionTotal("ACTION_TYPE_ESCALATE", "success"); got != 0 {
		t.Errorf("GetActionTotal(ESCALATE, success) = %d, want 0", got)
	}
}

func TestMetrics_GetActionDurations(t *testing.T) {
	m := NewMetrics()

	m.RecordActionExecution("ACTION_TYPE_NOTIFY_TEAM", "success", 10*time.Millisecond)
	m.RecordActionExecution("ACTION_TYPE_NOTIFY_TEAM", "success", 20*time.Millisecond)
	m.RecordActionExecution("ACTION_TYPE_NOTIFY_TEAM", "failure", 5*time.Millisecond)

	durations := m.GetActionDurations("ACTION_TYPE_NOTIFY_TEAM")
	if len(durations) != 3 {
		t.Errorf("GetActionDurations() returned %d durations, want 3", len(durations))
	}

	// Check non-existent action type
	emptyDurations := m.GetActionDurations("ACTION_TYPE_UNKNOWN")
	if len(emptyDurations) != 0 {
		t.Errorf("GetActionDurations(UNKNOWN) returned %d durations, want 0", len(emptyDurations))
	}
}

func TestMetrics_GetAverageDuration(t *testing.T) {
	m := NewMetrics()

	m.RecordActionExecution("ACTION_TYPE_NOTIFY_TEAM", "success", 10*time.Millisecond)
	m.RecordActionExecution("ACTION_TYPE_NOTIFY_TEAM", "success", 20*time.Millisecond)
	m.RecordActionExecution("ACTION_TYPE_NOTIFY_TEAM", "failure", 30*time.Millisecond)

	avg := m.GetAverageDuration("ACTION_TYPE_NOTIFY_TEAM")
	expected := 20 * time.Millisecond
	if avg != expected {
		t.Errorf("GetAverageDuration() = %v, want %v", avg, expected)
	}

	// Check non-existent action type
	zeroAvg := m.GetAverageDuration("ACTION_TYPE_UNKNOWN")
	if zeroAvg != 0 {
		t.Errorf("GetAverageDuration(UNKNOWN) = %v, want 0", zeroAvg)
	}
}

func TestMetrics_GetActionTotals(t *testing.T) {
	m := NewMetrics()

	m.RecordActionExecution("ACTION_TYPE_NOTIFY_TEAM", "success", 10*time.Millisecond)
	m.RecordActionExecution("ACTION_TYPE_NOTIFY_TEAM", "failure", 5*time.Millisecond)
	m.RecordActionExecution("ACTION_TYPE_SUPPRESS", "success", 15*time.Millisecond)

	totals := m.GetActionTotals()

	if len(totals) != 2 {
		t.Errorf("GetActionTotals() returned %d action types, want 2", len(totals))
	}

	if totals["ACTION_TYPE_NOTIFY_TEAM"]["success"] != 1 {
		t.Errorf("totals[NOTIFY_TEAM][success] = %d, want 1", totals["ACTION_TYPE_NOTIFY_TEAM"]["success"])
	}

	if totals["ACTION_TYPE_NOTIFY_TEAM"]["failure"] != 1 {
		t.Errorf("totals[NOTIFY_TEAM][failure] = %d, want 1", totals["ACTION_TYPE_NOTIFY_TEAM"]["failure"])
	}
}

func TestMetrics_Reset(t *testing.T) {
	m := NewMetrics()

	m.RecordActionExecution("ACTION_TYPE_NOTIFY_TEAM", "success", 10*time.Millisecond)
	m.RecordActionExecution("ACTION_TYPE_SUPPRESS", "success", 15*time.Millisecond)

	m.Reset()

	if got := m.GetActionTotal("ACTION_TYPE_NOTIFY_TEAM", "success"); got != 0 {
		t.Errorf("After Reset(), GetActionTotal() = %d, want 0", got)
	}

	totals := m.GetActionTotals()
	if len(totals) != 0 {
		t.Errorf("After Reset(), GetActionTotals() returned %d action types, want 0", len(totals))
	}
}

func TestMetrics_Concurrency(t *testing.T) {
	m := NewMetrics()

	// Test concurrent access
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				m.RecordActionExecution("ACTION_TYPE_NOTIFY_TEAM", "success", time.Millisecond)
				m.GetActionTotal("ACTION_TYPE_NOTIFY_TEAM", "success")
				m.GetActionDurations("ACTION_TYPE_NOTIFY_TEAM")
				m.GetAverageDuration("ACTION_TYPE_NOTIFY_TEAM")
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final count
	total := m.GetActionTotal("ACTION_TYPE_NOTIFY_TEAM", "success")
	if total != 1000 {
		t.Errorf("After concurrent access, total = %d, want 1000", total)
	}
}

func TestDefaultPrometheusMetrics(t *testing.T) {
	pm := DefaultPrometheusMetrics()

	if pm.ActionTotalName != "routing_action_total" {
		t.Errorf("ActionTotalName = %s, want routing_action_total", pm.ActionTotalName)
	}

	if pm.ActionDurationName != "routing_action_duration_seconds" {
		t.Errorf("ActionDurationName = %s, want routing_action_duration_seconds", pm.ActionDurationName)
	}

	if len(pm.Buckets) != 12 {
		t.Errorf("len(Buckets) = %d, want 12", len(pm.Buckets))
	}

	// Verify bucket values are increasing
	for i := 1; i < len(pm.Buckets); i++ {
		if pm.Buckets[i] <= pm.Buckets[i-1] {
			t.Errorf("Bucket %d (%v) should be greater than bucket %d (%v)",
				i, pm.Buckets[i], i-1, pm.Buckets[i-1])
		}
	}
}
