package maintenance

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/protobuf/types/known/timestamppb"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// mockStore is a mock implementation of the Store interface for testing.
type mockStore struct {
	windows         []*routingv1.MaintenanceWindow
	createCalled    bool
	getCalled       bool
	listCalled      bool
	updateCalled    bool
	deleteCalled    bool
	listActiveCalled bool
}

func newMockStore() *mockStore {
	return &mockStore{
		windows: make([]*routingv1.MaintenanceWindow, 0),
	}
}

func (m *mockStore) Create(ctx context.Context, window *routingv1.MaintenanceWindow) (*routingv1.MaintenanceWindow, error) {
	m.createCalled = true
	m.windows = append(m.windows, window)
	return window, nil
}

func (m *mockStore) Get(ctx context.Context, id string) (*routingv1.MaintenanceWindow, error) {
	m.getCalled = true
	for _, w := range m.windows {
		if w.Id == id {
			return w, nil
		}
	}
	return nil, ErrNotFound
}

func (m *mockStore) List(ctx context.Context, req *routingv1.ListMaintenanceWindowsRequest) (*routingv1.ListMaintenanceWindowsResponse, error) {
	m.listCalled = true
	return &routingv1.ListMaintenanceWindowsResponse{
		Windows:    m.windows,
		TotalCount: int32(len(m.windows)),
	}, nil
}

func (m *mockStore) Update(ctx context.Context, window *routingv1.MaintenanceWindow) (*routingv1.MaintenanceWindow, error) {
	m.updateCalled = true
	for i, w := range m.windows {
		if w.Id == window.Id {
			m.windows[i] = window
			return window, nil
		}
	}
	return nil, ErrNotFound
}

func (m *mockStore) Delete(ctx context.Context, id string) error {
	m.deleteCalled = true
	for i, w := range m.windows {
		if w.Id == id {
			m.windows = append(m.windows[:i], m.windows[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

func (m *mockStore) ListActive(ctx context.Context, siteIDs, serviceIDs []string) ([]*routingv1.MaintenanceWindow, error) {
	m.listActiveCalled = true
	var active []*routingv1.MaintenanceWindow
	now := time.Now()
	for _, w := range m.windows {
		if w.Status == routingv1.MaintenanceStatus_MAINTENANCE_STATUS_IN_PROGRESS &&
			w.StartTime.AsTime().Before(now) &&
			w.EndTime.AsTime().After(now) {
			active = append(active, w)
		}
	}
	return active, nil
}

func (m *mockStore) ListUpcoming(ctx context.Context, duration time.Duration) ([]*routingv1.MaintenanceWindow, error) {
	var upcoming []*routingv1.MaintenanceWindow
	now := time.Now()
	until := now.Add(duration)
	for _, w := range m.windows {
		if w.Status == routingv1.MaintenanceStatus_MAINTENANCE_STATUS_SCHEDULED &&
			w.StartTime.AsTime().After(now) &&
			w.StartTime.AsTime().Before(until) {
			upcoming = append(upcoming, w)
		}
	}
	return upcoming, nil
}

func (m *mockStore) UpdateStatus(ctx context.Context, id string, status routingv1.MaintenanceStatus) error {
	for _, w := range m.windows {
		if w.Id == id {
			w.Status = status
			return nil
		}
	}
	return ErrNotFound
}

func (m *mockStore) TransitionStatuses(ctx context.Context) error {
	now := time.Now()
	for _, w := range m.windows {
		// scheduled -> active
		if w.Status == routingv1.MaintenanceStatus_MAINTENANCE_STATUS_SCHEDULED &&
			w.StartTime.AsTime().Before(now) {
			w.Status = routingv1.MaintenanceStatus_MAINTENANCE_STATUS_IN_PROGRESS
		}
		// active -> completed
		if w.Status == routingv1.MaintenanceStatus_MAINTENANCE_STATUS_IN_PROGRESS &&
			w.EndTime.AsTime().Before(now) {
			w.Status = routingv1.MaintenanceStatus_MAINTENANCE_STATUS_COMPLETED
		}
	}
	return nil
}

// addActiveWindow adds an active window to the mock store.
func (m *mockStore) addActiveWindow(id, name string, sites, services, labels []string) {
	now := time.Now()
	m.windows = append(m.windows, &routingv1.MaintenanceWindow{
		Id:               id,
		Name:             name,
		StartTime:        timestamppb.New(now.Add(-1 * time.Hour)),
		EndTime:          timestamppb.New(now.Add(1 * time.Hour)),
		Status:           routingv1.MaintenanceStatus_MAINTENANCE_STATUS_IN_PROGRESS,
		Action:           routingv1.MaintenanceAction_MAINTENANCE_ACTION_SUPPRESS,
		AffectedSites:    sites,
		AffectedServices: services,
		AffectedLabels:   labels,
	})
}

func TestChecker_Check_NoActiveWindows(t *testing.T) {
	store := newMockStore()
	logger := zerolog.Nop()
	checker := NewChecker(store, logger)

	alert := &routingv1.Alert{
		Id:      "alert-1",
		Summary: "Test alert",
		Labels:  map[string]string{"site": "us-east-1"},
	}

	match, err := checker.Check(context.Background(), alert)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if match != nil {
		t.Error("expected no match when no active windows")
	}

	if !store.listActiveCalled {
		t.Error("expected ListActive to be called")
	}
}

func TestChecker_Check_MatchingSiteWindow(t *testing.T) {
	store := newMockStore()
	store.addActiveWindow("window-1", "Site Maintenance", []string{"us-east-1"}, nil, nil)

	logger := zerolog.Nop()
	checker := NewChecker(store, logger)

	alert := &routingv1.Alert{
		Id:      "alert-1",
		Summary: "Test alert",
		Labels:  map[string]string{"site": "us-east-1"},
	}

	match, err := checker.Check(context.Background(), alert)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if match == nil {
		t.Fatal("expected match")
	}

	if match.Window.Id != "window-1" {
		t.Errorf("expected window-1, got %s", match.Window.Id)
	}

	if match.Action != "suppress" {
		t.Errorf("expected action suppress, got %s", match.Action)
	}

	if match.MatchType != MatchTypeSite {
		t.Errorf("expected match type site, got %s", match.MatchType)
	}
}

func TestChecker_Check_NonMatchingWindow(t *testing.T) {
	store := newMockStore()
	store.addActiveWindow("window-1", "Site Maintenance", []string{"eu-west-1"}, nil, nil)

	logger := zerolog.Nop()
	checker := NewChecker(store, logger)

	alert := &routingv1.Alert{
		Id:      "alert-1",
		Summary: "Test alert",
		Labels:  map[string]string{"site": "us-east-1"},
	}

	match, err := checker.Check(context.Background(), alert)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if match != nil {
		t.Error("expected no match when site doesn't match")
	}
}

func TestChecker_Check_GlobalWindow(t *testing.T) {
	store := newMockStore()
	store.addActiveWindow("window-1", "Global Maintenance", nil, nil, nil)

	logger := zerolog.Nop()
	checker := NewChecker(store, logger)

	alert := &routingv1.Alert{
		Id:      "alert-1",
		Summary: "Test alert",
		Labels:  map[string]string{"site": "any-site"},
	}

	match, err := checker.Check(context.Background(), alert)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if match == nil {
		t.Fatal("expected match for global window")
	}

	if match.MatchType != MatchTypeGlobal {
		t.Errorf("expected match type global, got %s", match.MatchType)
	}
}

func TestChecker_CheckAll_MultipleMatches(t *testing.T) {
	store := newMockStore()
	store.addActiveWindow("window-1", "Site Maintenance", []string{"us-east-1"}, nil, nil)
	store.addActiveWindow("window-2", "Global Maintenance", nil, nil, nil)

	logger := zerolog.Nop()
	checker := NewChecker(store, logger)

	alert := &routingv1.Alert{
		Id:      "alert-1",
		Summary: "Test alert",
		Labels:  map[string]string{"site": "us-east-1"},
	}

	matches, err := checker.CheckAll(context.Background(), alert)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(matches))
	}
}

func TestChecker_Check_ServiceMatch(t *testing.T) {
	store := newMockStore()
	store.addActiveWindow("window-1", "Service Maintenance", nil, []string{"api-gateway"}, nil)

	logger := zerolog.Nop()
	checker := NewChecker(store, logger)

	alert := &routingv1.Alert{
		Id:        "alert-1",
		Summary:   "Test alert",
		ServiceId: "api-gateway",
	}

	match, err := checker.Check(context.Background(), alert)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if match == nil {
		t.Fatal("expected match for service window")
	}

	if match.MatchType != MatchTypeService {
		t.Errorf("expected match type service, got %s", match.MatchType)
	}
}

func TestChecker_Check_LabelMatch(t *testing.T) {
	store := newMockStore()
	store.addActiveWindow("window-1", "Label Maintenance", nil, nil, []string{"env=production"})

	logger := zerolog.Nop()
	checker := NewChecker(store, logger)

	alert := &routingv1.Alert{
		Id:      "alert-1",
		Summary: "Test alert",
		Labels:  map[string]string{"env": "production"},
	}

	match, err := checker.Check(context.Background(), alert)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if match == nil {
		t.Fatal("expected match for label window")
	}

	if match.MatchType != MatchTypeLabel {
		t.Errorf("expected match type label, got %s", match.MatchType)
	}
}

func TestChecker_Check_NilAlert(t *testing.T) {
	store := newMockStore()
	logger := zerolog.Nop()
	checker := NewChecker(store, logger)

	_, err := checker.Check(context.Background(), nil)

	if err == nil {
		t.Error("expected error for nil alert")
	}
}

func TestChecker_CheckForGRPC(t *testing.T) {
	store := newMockStore()
	store.addActiveWindow("window-1", "Site Maintenance", []string{"us-east-1"}, nil, nil)

	logger := zerolog.Nop()
	checker := NewChecker(store, logger)

	alert := &routingv1.Alert{
		Id:      "alert-1",
		Summary: "Test alert",
		Labels:  map[string]string{"site": "us-east-1"},
	}

	result, err := checker.CheckForGRPC(context.Background(), alert)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.InMaintenance {
		t.Error("expected InMaintenance to be true")
	}

	if len(result.MatchingWindows) != 1 {
		t.Errorf("expected 1 matching window, got %d", len(result.MatchingWindows))
	}

	if result.RecommendedAction != routingv1.MaintenanceAction_MAINTENANCE_ACTION_SUPPRESS {
		t.Errorf("expected suppress action, got %v", result.RecommendedAction)
	}
}

func TestChecker_ListActive(t *testing.T) {
	store := newMockStore()
	store.addActiveWindow("window-1", "Active Window", []string{"us-east-1"}, nil, nil)

	logger := zerolog.Nop()
	checker := NewChecker(store, logger)

	windows, err := checker.ListActive(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(windows) != 1 {
		t.Errorf("expected 1 active window, got %d", len(windows))
	}
}

func TestChecker_ListUpcoming(t *testing.T) {
	store := newMockStore()

	// Add a scheduled window starting in 30 minutes
	now := time.Now()
	store.windows = append(store.windows, &routingv1.MaintenanceWindow{
		Id:        "window-1",
		Name:      "Upcoming Window",
		StartTime: timestamppb.New(now.Add(30 * time.Minute)),
		EndTime:   timestamppb.New(now.Add(2 * time.Hour)),
		Status:    routingv1.MaintenanceStatus_MAINTENANCE_STATUS_SCHEDULED,
	})

	logger := zerolog.Nop()
	checker := NewChecker(store, logger)

	windows, err := checker.ListUpcoming(context.Background(), 1*time.Hour)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(windows) != 1 {
		t.Errorf("expected 1 upcoming window, got %d", len(windows))
	}
}

func TestChecker_RefreshStatuses(t *testing.T) {
	store := newMockStore()

	// Add a scheduled window that should now be active
	now := time.Now()
	store.windows = append(store.windows, &routingv1.MaintenanceWindow{
		Id:        "window-1",
		Name:      "Should Be Active",
		StartTime: timestamppb.New(now.Add(-30 * time.Minute)),
		EndTime:   timestamppb.New(now.Add(30 * time.Minute)),
		Status:    routingv1.MaintenanceStatus_MAINTENANCE_STATUS_SCHEDULED,
	})

	logger := zerolog.Nop()
	checker := NewChecker(store, logger)

	err := checker.RefreshStatuses(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if store.windows[0].Status != routingv1.MaintenanceStatus_MAINTENANCE_STATUS_IN_PROGRESS {
		t.Errorf("expected status IN_PROGRESS, got %v", store.windows[0].Status)
	}
}
