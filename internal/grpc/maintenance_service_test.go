package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kneutral-org/alerting-system/internal/maintenance"
	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// mockMaintenanceStore is a mock implementation of maintenance.Store for testing.
type mockMaintenanceStore struct {
	windows    []*routingv1.MaintenanceWindow
	createErr  error
	getErr     error
	listErr    error
	updateErr  error
	deleteErr  error
}

func newMockMaintenanceStore() *mockMaintenanceStore {
	return &mockMaintenanceStore{
		windows: make([]*routingv1.MaintenanceWindow, 0),
	}
}

func (m *mockMaintenanceStore) Create(ctx context.Context, window *routingv1.MaintenanceWindow) (*routingv1.MaintenanceWindow, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if window.Id == "" {
		window.Id = "generated-id"
	}
	window.CreatedAt = timestamppb.Now()
	m.windows = append(m.windows, window)
	return window, nil
}

func (m *mockMaintenanceStore) Get(ctx context.Context, id string) (*routingv1.MaintenanceWindow, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, w := range m.windows {
		if w.Id == id {
			return w, nil
		}
	}
	return nil, maintenance.ErrNotFound
}

func (m *mockMaintenanceStore) List(ctx context.Context, req *routingv1.ListMaintenanceWindowsRequest) (*routingv1.ListMaintenanceWindowsResponse, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return &routingv1.ListMaintenanceWindowsResponse{
		Windows:    m.windows,
		TotalCount: int32(len(m.windows)),
	}, nil
}

func (m *mockMaintenanceStore) Update(ctx context.Context, window *routingv1.MaintenanceWindow) (*routingv1.MaintenanceWindow, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	for i, w := range m.windows {
		if w.Id == window.Id {
			m.windows[i] = window
			return window, nil
		}
	}
	return nil, maintenance.ErrNotFound
}

func (m *mockMaintenanceStore) Delete(ctx context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	for i, w := range m.windows {
		if w.Id == id {
			m.windows = append(m.windows[:i], m.windows[i+1:]...)
			return nil
		}
	}
	return maintenance.ErrNotFound
}

func (m *mockMaintenanceStore) ListActive(ctx context.Context, siteIDs, serviceIDs []string) ([]*routingv1.MaintenanceWindow, error) {
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

func (m *mockMaintenanceStore) ListUpcoming(ctx context.Context, duration time.Duration) ([]*routingv1.MaintenanceWindow, error) {
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

func (m *mockMaintenanceStore) UpdateStatus(ctx context.Context, id string, status routingv1.MaintenanceStatus) error {
	for _, w := range m.windows {
		if w.Id == id {
			w.Status = status
			return nil
		}
	}
	return maintenance.ErrNotFound
}

func (m *mockMaintenanceStore) TransitionStatuses(ctx context.Context) error {
	return nil
}

func (m *mockMaintenanceStore) addActiveWindow(id, name string, sites, services []string) {
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
	})
}

func TestMaintenanceService_CreateMaintenanceWindow(t *testing.T) {
	store := newMockMaintenanceStore()
	logger := zerolog.Nop()
	service := NewMaintenanceService(store, logger)

	now := time.Now()
	req := &routingv1.CreateMaintenanceWindowRequest{
		Window: &routingv1.MaintenanceWindow{
			Name:        "Test Maintenance",
			Description: "Test description",
			StartTime:   timestamppb.New(now.Add(1 * time.Hour)),
			EndTime:     timestamppb.New(now.Add(2 * time.Hour)),
			Action:      routingv1.MaintenanceAction_MAINTENANCE_ACTION_SUPPRESS,
		},
	}

	window, err := service.CreateMaintenanceWindow(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if window == nil {
		t.Fatal("expected window to be created")
	}

	if window.Name != "Test Maintenance" {
		t.Errorf("expected name 'Test Maintenance', got '%s'", window.Name)
	}
}

func TestMaintenanceService_CreateMaintenanceWindow_Validation(t *testing.T) {
	store := newMockMaintenanceStore()
	logger := zerolog.Nop()
	service := NewMaintenanceService(store, logger)

	tests := []struct {
		name     string
		req      *routingv1.CreateMaintenanceWindowRequest
		wantCode codes.Code
	}{
		{
			name:     "nil window",
			req:      &routingv1.CreateMaintenanceWindowRequest{Window: nil},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "missing name",
			req: &routingv1.CreateMaintenanceWindowRequest{
				Window: &routingv1.MaintenanceWindow{
					StartTime: timestamppb.Now(),
					EndTime:   timestamppb.New(time.Now().Add(1 * time.Hour)),
				},
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "missing start_time",
			req: &routingv1.CreateMaintenanceWindowRequest{
				Window: &routingv1.MaintenanceWindow{
					Name:    "Test",
					EndTime: timestamppb.New(time.Now().Add(1 * time.Hour)),
				},
			},
			wantCode: codes.InvalidArgument,
		},
		{
			name: "missing end_time",
			req: &routingv1.CreateMaintenanceWindowRequest{
				Window: &routingv1.MaintenanceWindow{
					Name:      "Test",
					StartTime: timestamppb.Now(),
				},
			},
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.CreateMaintenanceWindow(context.Background(), tc.req)

			if err == nil {
				t.Fatal("expected error")
			}

			st, ok := status.FromError(err)
			if !ok {
				t.Fatalf("expected gRPC status error, got %v", err)
			}

			if st.Code() != tc.wantCode {
				t.Errorf("expected code %v, got %v", tc.wantCode, st.Code())
			}
		})
	}
}

func TestMaintenanceService_GetMaintenanceWindow(t *testing.T) {
	store := newMockMaintenanceStore()
	store.addActiveWindow("window-1", "Test Window", nil, nil)

	logger := zerolog.Nop()
	service := NewMaintenanceService(store, logger)

	req := &routingv1.GetMaintenanceWindowRequest{Id: "window-1"}

	window, err := service.GetMaintenanceWindow(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if window.Id != "window-1" {
		t.Errorf("expected id 'window-1', got '%s'", window.Id)
	}
}

func TestMaintenanceService_GetMaintenanceWindow_NotFound(t *testing.T) {
	store := newMockMaintenanceStore()
	logger := zerolog.Nop()
	service := NewMaintenanceService(store, logger)

	req := &routingv1.GetMaintenanceWindowRequest{Id: "non-existent"}

	_, err := service.GetMaintenanceWindow(context.Background(), req)

	if err == nil {
		t.Fatal("expected error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}

	if st.Code() != codes.NotFound {
		t.Errorf("expected code NotFound, got %v", st.Code())
	}
}

func TestMaintenanceService_GetMaintenanceWindow_MissingID(t *testing.T) {
	store := newMockMaintenanceStore()
	logger := zerolog.Nop()
	service := NewMaintenanceService(store, logger)

	req := &routingv1.GetMaintenanceWindowRequest{Id: ""}

	_, err := service.GetMaintenanceWindow(context.Background(), req)

	if err == nil {
		t.Fatal("expected error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}

	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected code InvalidArgument, got %v", st.Code())
	}
}

func TestMaintenanceService_ListMaintenanceWindows(t *testing.T) {
	store := newMockMaintenanceStore()
	store.addActiveWindow("window-1", "Window 1", nil, nil)
	store.addActiveWindow("window-2", "Window 2", nil, nil)

	logger := zerolog.Nop()
	service := NewMaintenanceService(store, logger)

	req := &routingv1.ListMaintenanceWindowsRequest{PageSize: 10}

	resp, err := service.ListMaintenanceWindows(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.TotalCount != 2 {
		t.Errorf("expected 2 windows, got %d", resp.TotalCount)
	}
}

func TestMaintenanceService_UpdateMaintenanceWindow(t *testing.T) {
	store := newMockMaintenanceStore()
	store.addActiveWindow("window-1", "Original Name", nil, nil)

	logger := zerolog.Nop()
	service := NewMaintenanceService(store, logger)

	req := &routingv1.UpdateMaintenanceWindowRequest{
		Window: &routingv1.MaintenanceWindow{
			Id:        "window-1",
			Name:      "Updated Name",
			StartTime: store.windows[0].StartTime,
			EndTime:   store.windows[0].EndTime,
			Status:    store.windows[0].Status,
			Action:    store.windows[0].Action,
		},
	}

	window, err := service.UpdateMaintenanceWindow(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if window.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got '%s'", window.Name)
	}
}

func TestMaintenanceService_UpdateMaintenanceWindow_NotFound(t *testing.T) {
	store := newMockMaintenanceStore()
	logger := zerolog.Nop()
	service := NewMaintenanceService(store, logger)

	req := &routingv1.UpdateMaintenanceWindowRequest{
		Window: &routingv1.MaintenanceWindow{
			Id:        "non-existent",
			Name:      "Test",
			StartTime: timestamppb.Now(),
			EndTime:   timestamppb.New(time.Now().Add(1 * time.Hour)),
		},
	}

	_, err := service.UpdateMaintenanceWindow(context.Background(), req)

	if err == nil {
		t.Fatal("expected error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}

	if st.Code() != codes.NotFound {
		t.Errorf("expected code NotFound, got %v", st.Code())
	}
}

func TestMaintenanceService_DeleteMaintenanceWindow(t *testing.T) {
	store := newMockMaintenanceStore()
	store.addActiveWindow("window-1", "Test Window", nil, nil)

	logger := zerolog.Nop()
	service := NewMaintenanceService(store, logger)

	req := &routingv1.DeleteMaintenanceWindowRequest{Id: "window-1"}

	resp, err := service.DeleteMaintenanceWindow(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}

	if len(store.windows) != 0 {
		t.Errorf("expected 0 windows, got %d", len(store.windows))
	}
}

func TestMaintenanceService_DeleteMaintenanceWindow_NotFound(t *testing.T) {
	store := newMockMaintenanceStore()
	logger := zerolog.Nop()
	service := NewMaintenanceService(store, logger)

	req := &routingv1.DeleteMaintenanceWindowRequest{Id: "non-existent"}

	_, err := service.DeleteMaintenanceWindow(context.Background(), req)

	if err == nil {
		t.Fatal("expected error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}

	if st.Code() != codes.NotFound {
		t.Errorf("expected code NotFound, got %v", st.Code())
	}
}

func TestMaintenanceService_ListActiveMaintenanceWindows(t *testing.T) {
	store := newMockMaintenanceStore()
	store.addActiveWindow("window-1", "Active Window", []string{"us-east-1"}, nil)

	logger := zerolog.Nop()
	service := NewMaintenanceService(store, logger)

	req := &routingv1.ListActiveMaintenanceWindowsRequest{}

	resp, err := service.ListActiveMaintenanceWindows(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.TotalCount != 1 {
		t.Errorf("expected 1 active window, got %d", resp.TotalCount)
	}
}

func TestMaintenanceService_CheckAlertMaintenance(t *testing.T) {
	store := newMockMaintenanceStore()
	store.addActiveWindow("window-1", "Site Maintenance", []string{"us-east-1"}, nil)

	logger := zerolog.Nop()
	service := NewMaintenanceService(store, logger)

	req := &routingv1.CheckAlertMaintenanceRequest{
		Alert: &routingv1.Alert{
			Id:      "alert-1",
			Summary: "Test alert",
			Labels:  map[string]string{"site": "us-east-1"},
		},
	}

	resp, err := service.CheckAlertMaintenance(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.InMaintenance {
		t.Error("expected InMaintenance to be true")
	}

	if len(resp.MatchingWindows) != 1 {
		t.Errorf("expected 1 matching window, got %d", len(resp.MatchingWindows))
	}

	if resp.RecommendedAction != routingv1.MaintenanceAction_MAINTENANCE_ACTION_SUPPRESS {
		t.Errorf("expected suppress action, got %v", resp.RecommendedAction)
	}
}

func TestMaintenanceService_CheckAlertMaintenance_NoMatch(t *testing.T) {
	store := newMockMaintenanceStore()
	store.addActiveWindow("window-1", "Site Maintenance", []string{"eu-west-1"}, nil)

	logger := zerolog.Nop()
	service := NewMaintenanceService(store, logger)

	req := &routingv1.CheckAlertMaintenanceRequest{
		Alert: &routingv1.Alert{
			Id:      "alert-1",
			Summary: "Test alert",
			Labels:  map[string]string{"site": "us-east-1"},
		},
	}

	resp, err := service.CheckAlertMaintenance(context.Background(), req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.InMaintenance {
		t.Error("expected InMaintenance to be false")
	}

	if len(resp.MatchingWindows) != 0 {
		t.Errorf("expected 0 matching windows, got %d", len(resp.MatchingWindows))
	}
}

func TestMaintenanceService_CheckAlertMaintenance_MissingAlert(t *testing.T) {
	store := newMockMaintenanceStore()
	logger := zerolog.Nop()
	service := NewMaintenanceService(store, logger)

	req := &routingv1.CheckAlertMaintenanceRequest{Alert: nil}

	_, err := service.CheckAlertMaintenance(context.Background(), req)

	if err == nil {
		t.Fatal("expected error")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}

	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected code InvalidArgument, got %v", st.Code())
	}
}

func TestMaintenanceService_CancelMaintenanceWindow(t *testing.T) {
	store := newMockMaintenanceStore()
	store.addActiveWindow("window-1", "Active Window", nil, nil)

	logger := zerolog.Nop()
	service := NewMaintenanceService(store, logger)

	err := service.CancelMaintenanceWindow(context.Background(), "window-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if store.windows[0].Status != routingv1.MaintenanceStatus_MAINTENANCE_STATUS_CANCELLED {
		t.Errorf("expected status CANCELLED, got %v", store.windows[0].Status)
	}
}
