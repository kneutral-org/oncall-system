// Package escalation provides the escalation policy store implementation.
package escalation

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EscalationPolicy represents an escalation policy domain model.
type EscalationPolicy struct {
	ID              uuid.UUID             `json:"id"`
	Name            string                `json:"name"`
	Description     string                `json:"description,omitempty"`
	Steps           []EscalationStep      `json:"steps"`
	RepeatCount     int32                 `json:"repeatCount"`
	ExhaustedAction *ExhaustedAction      `json:"exhaustedAction,omitempty"`
	CreatedAt       time.Time             `json:"createdAt"`
	UpdatedAt       time.Time             `json:"updatedAt"`
}

// EscalationStep represents a step in an escalation policy.
type EscalationStep struct {
	StepNumber       int32              `json:"stepNumber"`
	DelaySeconds     int32              `json:"delaySeconds"`
	Targets          []EscalationTarget `json:"targets"`
	SkipConditionCel string             `json:"skipConditionCel,omitempty"`
}

// EscalationTarget represents a target for escalation notifications.
type EscalationTarget struct {
	Type     string `json:"type"`
	TargetID string `json:"targetId"`
}

// ExhaustedAction defines what to do when escalation is exhausted.
type ExhaustedAction struct {
	Action   string                 `json:"action"`
	Config   map[string]interface{} `json:"config,omitempty"`
}

// ListPoliciesParams contains parameters for listing escalation policies.
type ListPoliciesParams struct {
	Limit  int32
	Offset int32
}

// PolicyStore defines the interface for escalation policy persistence.
type PolicyStore interface {
	// CreatePolicy creates a new escalation policy.
	CreatePolicy(ctx context.Context, policy *EscalationPolicy) (*EscalationPolicy, error)

	// GetPolicy retrieves an escalation policy by ID.
	GetPolicy(ctx context.Context, id uuid.UUID) (*EscalationPolicy, error)

	// ListPolicies retrieves escalation policies based on filter criteria.
	ListPolicies(ctx context.Context, params ListPoliciesParams) ([]*EscalationPolicy, error)

	// UpdatePolicy updates an existing escalation policy.
	UpdatePolicy(ctx context.Context, policy *EscalationPolicy) (*EscalationPolicy, error)

	// DeletePolicy deletes an escalation policy.
	DeletePolicy(ctx context.Context, id uuid.UUID) error
}

// ActiveEscalationStore defines the interface for active escalation tracking.
// For MVP, this is a placeholder - full implementation would track running escalations.
type ActiveEscalationStore interface {
	// StartEscalation starts a new escalation for an alert.
	StartEscalation(ctx context.Context, policyID, alertID uuid.UUID, startStep int32) (uuid.UUID, error)

	// GetEscalationStatus gets the status of an active escalation.
	GetEscalationStatus(ctx context.Context, escalationID uuid.UUID) (*ActiveEscalation, error)

	// StopEscalation stops an active escalation.
	StopEscalation(ctx context.Context, escalationID uuid.UUID, reason, stoppedBy string) error
}

// ActiveEscalation represents an active escalation instance.
type ActiveEscalation struct {
	ID          uuid.UUID `json:"id"`
	PolicyID    uuid.UUID `json:"policyId"`
	AlertID     uuid.UUID `json:"alertId"`
	CurrentStep int32     `json:"currentStep"`
	RepeatCount int32     `json:"repeatCount"`
	State       string    `json:"state"`
	StartedAt   time.Time `json:"startedAt"`
	NextStepAt  time.Time `json:"nextStepAt,omitempty"`
}


// InMemoryPolicyStore is an in-memory implementation of PolicyStore.
// For MVP, this is the primary store implementation.
type InMemoryPolicyStore struct {
	mu       sync.RWMutex
	policies map[uuid.UUID]*EscalationPolicy
}

// NewInMemoryPolicyStore creates a new in-memory policy store.
func NewInMemoryPolicyStore() *InMemoryPolicyStore {
	return &InMemoryPolicyStore{
		policies: make(map[uuid.UUID]*EscalationPolicy),
	}
}

// CreatePolicy creates a new escalation policy.
func (s *InMemoryPolicyStore) CreatePolicy(ctx context.Context, policy *EscalationPolicy) (*EscalationPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	policy.ID = uuid.New()
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	// Deep copy to avoid external mutations
	stored := *policy
	s.policies[policy.ID] = &stored

	return policy, nil
}

// GetPolicy retrieves an escalation policy by ID.
func (s *InMemoryPolicyStore) GetPolicy(ctx context.Context, id uuid.UUID) (*EscalationPolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	policy, ok := s.policies[id]
	if !ok {
		return nil, nil
	}

	// Return a copy to avoid external mutations
	result := *policy
	return &result, nil
}

// ListPolicies retrieves escalation policies.
func (s *InMemoryPolicyStore) ListPolicies(ctx context.Context, params ListPoliciesParams) ([]*EscalationPolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}

	result := make([]*EscalationPolicy, 0, len(s.policies))
	offset := int(params.Offset)
	count := 0

	for _, p := range s.policies {
		if count < offset {
			count++
			continue
		}
		if int32(len(result)) >= limit {
			break
		}
		// Return a copy to avoid external mutations
		policy := *p
		result = append(result, &policy)
		count++
	}

	return result, nil
}

// UpdatePolicy updates an existing escalation policy.
func (s *InMemoryPolicyStore) UpdatePolicy(ctx context.Context, policy *EscalationPolicy) (*EscalationPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.policies[policy.ID]; !ok {
		return nil, nil
	}

	policy.UpdatedAt = time.Now()

	// Deep copy to avoid external mutations
	stored := *policy
	s.policies[policy.ID] = &stored

	return policy, nil
}

// DeletePolicy deletes an escalation policy.
func (s *InMemoryPolicyStore) DeletePolicy(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.policies, id)
	return nil
}

// InMemoryActiveEscalationStore is an in-memory implementation of ActiveEscalationStore.
// For MVP, this is a placeholder - full implementation would be added later.
type InMemoryActiveEscalationStore struct {
	mu          sync.RWMutex
	escalations map[uuid.UUID]*ActiveEscalation
}

// NewInMemoryActiveEscalationStore creates a new in-memory active escalation store.
func NewInMemoryActiveEscalationStore() *InMemoryActiveEscalationStore {
	return &InMemoryActiveEscalationStore{
		escalations: make(map[uuid.UUID]*ActiveEscalation),
	}
}

// StartEscalation starts a new escalation for an alert.
func (s *InMemoryActiveEscalationStore) StartEscalation(ctx context.Context, policyID, alertID uuid.UUID, startStep int32) (uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	escalation := &ActiveEscalation{
		ID:          uuid.New(),
		PolicyID:    policyID,
		AlertID:     alertID,
		CurrentStep: startStep,
		RepeatCount: 0,
		State:       "ACTIVE",
		StartedAt:   time.Now(),
	}

	s.escalations[escalation.ID] = escalation
	return escalation.ID, nil
}

// GetEscalationStatus gets the status of an active escalation.
func (s *InMemoryActiveEscalationStore) GetEscalationStatus(ctx context.Context, escalationID uuid.UUID) (*ActiveEscalation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	escalation, ok := s.escalations[escalationID]
	if !ok {
		return nil, nil
	}

	result := *escalation
	return &result, nil
}

// StopEscalation stops an active escalation.
func (s *InMemoryActiveEscalationStore) StopEscalation(ctx context.Context, escalationID uuid.UUID, reason, stoppedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if escalation, ok := s.escalations[escalationID]; ok {
		escalation.State = "STOPPED"
	}

	return nil
}
