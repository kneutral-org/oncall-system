// Package maintenance provides the maintenance window store implementation.
package maintenance

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MaintenanceAction represents the action to take during maintenance.
type MaintenanceAction string

const (
	ActionSuppress MaintenanceAction = "suppress"
	ActionSilence  MaintenanceAction = "silence"
	ActionRedirect MaintenanceAction = "redirect"
)

// MaintenanceStatus represents the status of a maintenance window.
type MaintenanceStatus string

const (
	StatusScheduled MaintenanceStatus = "scheduled"
	StatusActive    MaintenanceStatus = "active"
	StatusCompleted MaintenanceStatus = "completed"
	StatusCancelled MaintenanceStatus = "cancelled"
)

// MaintenanceWindow represents a maintenance window domain model.
type MaintenanceWindow struct {
	ID               uuid.UUID              `json:"id"`
	Name             string                 `json:"name"`
	Description      string                 `json:"description,omitempty"`
	StartTime        time.Time              `json:"startTime"`
	EndTime          time.Time              `json:"endTime"`
	AffectedSites    []string               `json:"affectedSites"`
	AffectedServices []string               `json:"affectedServices"`
	AffectedLabels   map[string]string      `json:"affectedLabels"`
	Action           MaintenanceAction      `json:"action"`
	Status           MaintenanceStatus      `json:"status"`
	ChangeTicketID   string                 `json:"changeTicketId,omitempty"`
	CreatedBy        *uuid.UUID             `json:"createdBy,omitempty"`
	CreatedAt        time.Time              `json:"createdAt"`
}

// ListParams contains parameters for listing maintenance windows.
type ListParams struct {
	StatusFilter []MaintenanceStatus
	SitesFilter  []string
	Limit        int32
	Offset       int32
}

// MaintenanceWindowStore defines the interface for maintenance window persistence.
type MaintenanceWindowStore interface {
	// CreateWindow creates a new maintenance window.
	CreateWindow(ctx context.Context, w *MaintenanceWindow) (*MaintenanceWindow, error)

	// GetWindow retrieves a maintenance window by ID.
	GetWindow(ctx context.Context, id uuid.UUID) (*MaintenanceWindow, error)

	// ListWindows retrieves maintenance windows based on filter criteria.
	ListWindows(ctx context.Context, params ListParams) ([]*MaintenanceWindow, error)

	// ListActiveWindows retrieves all currently active maintenance windows.
	ListActiveWindows(ctx context.Context) ([]*MaintenanceWindow, error)

	// UpdateWindow updates an existing maintenance window.
	UpdateWindow(ctx context.Context, w *MaintenanceWindow) (*MaintenanceWindow, error)

	// DeleteWindow deletes a maintenance window.
	DeleteWindow(ctx context.Context, id uuid.UUID) error

	// CheckAlertInMaintenance checks if an alert with given labels is in a maintenance window.
	CheckAlertInMaintenance(ctx context.Context, alertLabels map[string]string) (*MaintenanceWindow, bool)
}

// InMemoryMaintenanceWindowStore is an in-memory implementation of MaintenanceWindowStore.
type InMemoryMaintenanceWindowStore struct {
	mu      sync.RWMutex
	windows map[uuid.UUID]*MaintenanceWindow
}

// NewInMemoryMaintenanceWindowStore creates a new in-memory maintenance window store.
func NewInMemoryMaintenanceWindowStore() *InMemoryMaintenanceWindowStore {
	return &InMemoryMaintenanceWindowStore{
		windows: make(map[uuid.UUID]*MaintenanceWindow),
	}
}

// CreateWindow creates a new maintenance window.
func (s *InMemoryMaintenanceWindowStore) CreateWindow(ctx context.Context, w *MaintenanceWindow) (*MaintenanceWindow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	w.ID = uuid.New()
	w.CreatedAt = time.Now()

	if w.Action == "" {
		w.Action = ActionSuppress
	}
	if w.Status == "" {
		w.Status = StatusScheduled
	}
	if w.AffectedSites == nil {
		w.AffectedSites = []string{}
	}
	if w.AffectedServices == nil {
		w.AffectedServices = []string{}
	}
	if w.AffectedLabels == nil {
		w.AffectedLabels = make(map[string]string)
	}

	// Deep copy
	stored := deepCopyWindow(w)
	s.windows[w.ID] = stored

	return w, nil
}

// GetWindow retrieves a maintenance window by ID.
func (s *InMemoryMaintenanceWindowStore) GetWindow(ctx context.Context, id uuid.UUID) (*MaintenanceWindow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w, ok := s.windows[id]
	if !ok {
		return nil, nil
	}

	return deepCopyWindow(w), nil
}

// ListWindows retrieves maintenance windows based on filter criteria.
func (s *InMemoryMaintenanceWindowStore) ListWindows(ctx context.Context, params ListParams) ([]*MaintenanceWindow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}

	result := make([]*MaintenanceWindow, 0)
	offset := int(params.Offset)
	count := 0

	for _, w := range s.windows {
		// Apply status filter
		if len(params.StatusFilter) > 0 {
			found := false
			for _, status := range params.StatusFilter {
				if w.Status == status {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Apply sites filter
		if len(params.SitesFilter) > 0 {
			found := false
			for _, fs := range params.SitesFilter {
				for _, as := range w.AffectedSites {
					if fs == as {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				continue
			}
		}

		if count < offset {
			count++
			continue
		}

		if int32(len(result)) >= limit {
			break
		}

		result = append(result, deepCopyWindow(w))
		count++
	}

	return result, nil
}

// ListActiveWindows retrieves all currently active maintenance windows.
func (s *InMemoryMaintenanceWindowStore) ListActiveWindows(ctx context.Context) ([]*MaintenanceWindow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	result := make([]*MaintenanceWindow, 0)

	for _, w := range s.windows {
		// Check if window is active (scheduled or active status and within time range)
		if (w.Status == StatusScheduled || w.Status == StatusActive) &&
			!now.Before(w.StartTime) && now.Before(w.EndTime) {
			result = append(result, deepCopyWindow(w))
		}
	}

	return result, nil
}

// UpdateWindow updates an existing maintenance window.
func (s *InMemoryMaintenanceWindowStore) UpdateWindow(ctx context.Context, w *MaintenanceWindow) (*MaintenanceWindow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.windows[w.ID]
	if !ok {
		return nil, nil
	}

	w.CreatedAt = existing.CreatedAt
	w.CreatedBy = existing.CreatedBy

	stored := deepCopyWindow(w)
	s.windows[w.ID] = stored

	return w, nil
}

// DeleteWindow deletes a maintenance window.
func (s *InMemoryMaintenanceWindowStore) DeleteWindow(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.windows, id)
	return nil
}

// CheckAlertInMaintenance checks if an alert with given labels is in a maintenance window.
func (s *InMemoryMaintenanceWindowStore) CheckAlertInMaintenance(ctx context.Context, alertLabels map[string]string) (*MaintenanceWindow, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()

	for _, w := range s.windows {
		// Check if window is currently active
		isValidStatus := w.Status == StatusScheduled || w.Status == StatusActive
		isWithinTimeRange := !now.Before(w.StartTime) && now.Before(w.EndTime)
		if !isValidStatus || !isWithinTimeRange {
			continue
		}

		// Check if alert matches any affected site
		if len(w.AffectedSites) > 0 {
			if site, ok := alertLabels["site"]; ok {
				for _, as := range w.AffectedSites {
					if as == site {
						return deepCopyWindow(w), true
					}
				}
			}
		}

		// Check if alert matches any affected service
		if len(w.AffectedServices) > 0 {
			if service, ok := alertLabels["service"]; ok {
				for _, as := range w.AffectedServices {
					if as == service {
						return deepCopyWindow(w), true
					}
				}
			}
		}

		// Check if alert labels match affected labels
		if len(w.AffectedLabels) > 0 {
			allMatch := true
			for k, v := range w.AffectedLabels {
				if alertVal, ok := alertLabels[k]; !ok || alertVal != v {
					allMatch = false
					break
				}
			}
			if allMatch {
				return deepCopyWindow(w), true
			}
		}
	}

	return nil, false
}

// deepCopyWindow creates a deep copy of a MaintenanceWindow.
func deepCopyWindow(w *MaintenanceWindow) *MaintenanceWindow {
	copied := *w

	if w.CreatedBy != nil {
		id := *w.CreatedBy
		copied.CreatedBy = &id
	}

	copied.AffectedSites = make([]string, len(w.AffectedSites))
	copy(copied.AffectedSites, w.AffectedSites)

	copied.AffectedServices = make([]string, len(w.AffectedServices))
	copy(copied.AffectedServices, w.AffectedServices)

	if w.AffectedLabels != nil {
		copied.AffectedLabels = make(map[string]string)
		data, _ := json.Marshal(w.AffectedLabels)
		_ = json.Unmarshal(data, &copied.AffectedLabels)
	}

	return &copied
}

// Verify InMemoryMaintenanceWindowStore implements MaintenanceWindowStore interface
var _ MaintenanceWindowStore = (*InMemoryMaintenanceWindowStore)(nil)
