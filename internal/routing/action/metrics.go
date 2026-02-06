package action

import (
	"sync"
	"time"
)

// Metrics tracks action execution metrics.
// In a production environment, these would typically integrate with
// Prometheus or another metrics system.
type Metrics struct {
	mu sync.RWMutex

	// actionTotal tracks the total number of actions executed by type and status.
	actionTotal map[string]map[string]int64

	// actionDuration tracks action execution durations by type.
	actionDuration map[string][]time.Duration
}

// NewMetrics creates a new Metrics instance.
func NewMetrics() *Metrics {
	return &Metrics{
		actionTotal:    make(map[string]map[string]int64),
		actionDuration: make(map[string][]time.Duration),
	}
}

// RecordActionExecution records the execution of an action.
func (m *Metrics) RecordActionExecution(actionType, status string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Initialize maps if needed
	if m.actionTotal[actionType] == nil {
		m.actionTotal[actionType] = make(map[string]int64)
	}

	// Increment counter
	m.actionTotal[actionType][status]++

	// Record duration
	m.actionDuration[actionType] = append(m.actionDuration[actionType], duration)
}

// GetActionTotal returns the total count for an action type and status.
func (m *Metrics) GetActionTotal(actionType, status string) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.actionTotal[actionType] == nil {
		return 0
	}
	return m.actionTotal[actionType][status]
}

// GetActionTotals returns all action totals.
func (m *Metrics) GetActionTotals() map[string]map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]map[string]int64)
	for actionType, statuses := range m.actionTotal {
		result[actionType] = make(map[string]int64)
		for status, count := range statuses {
			result[actionType][status] = count
		}
	}
	return result
}

// GetActionDurations returns the recorded durations for an action type.
func (m *Metrics) GetActionDurations(actionType string) []time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	durations := m.actionDuration[actionType]
	result := make([]time.Duration, len(durations))
	copy(result, durations)
	return result
}

// GetAverageDuration calculates the average duration for an action type.
func (m *Metrics) GetAverageDuration(actionType string) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	durations := m.actionDuration[actionType]
	if len(durations) == 0 {
		return 0
	}

	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

// Reset clears all recorded metrics.
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.actionTotal = make(map[string]map[string]int64)
	m.actionDuration = make(map[string][]time.Duration)
}

// PrometheusMetrics provides Prometheus-compatible metric names and labels.
// This is a helper for integration with Prometheus client libraries.
type PrometheusMetrics struct {
	// ActionTotalName is the metric name for the action total counter.
	// Labels: action_type, status
	ActionTotalName string

	// ActionDurationName is the metric name for the action duration histogram.
	// Labels: action_type
	ActionDurationName string

	// Buckets defines the histogram buckets for action duration.
	Buckets []float64
}

// DefaultPrometheusMetrics returns the default Prometheus metric configuration.
func DefaultPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		ActionTotalName:    "routing_action_total",
		ActionDurationName: "routing_action_duration_seconds",
		Buckets: []float64{
			0.001, // 1ms
			0.005, // 5ms
			0.01,  // 10ms
			0.025, // 25ms
			0.05,  // 50ms
			0.1,   // 100ms
			0.25,  // 250ms
			0.5,   // 500ms
			1.0,   // 1s
			2.5,   // 2.5s
			5.0,   // 5s
			10.0,  // 10s
		},
	}
}
