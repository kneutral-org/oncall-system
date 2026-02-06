package action

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/protobuf/types/known/durationpb"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// MockNotificationService is a mock implementation of NotificationService.
type MockNotificationService struct {
	NotifyTeamFunc    func(ctx context.Context, teamID string, scope routingv1.TeamNotifyScope, templateID string, alert *routingv1.Alert) error
	NotifyChannelFunc func(ctx context.Context, target *routingv1.NotificationTarget, templateID string, alert *routingv1.Alert) error
	NotifyUserFunc    func(ctx context.Context, userID string, templateID string, channelOverride routingv1.ChannelType, alert *routingv1.Alert) error
	NotifyOnCallFunc  func(ctx context.Context, scheduleID string, templateID string, level routingv1.OnCallLevel, alert *routingv1.Alert) error
}

func (m *MockNotificationService) NotifyTeam(ctx context.Context, teamID string, scope routingv1.TeamNotifyScope, templateID string, alert *routingv1.Alert) error {
	if m.NotifyTeamFunc != nil {
		return m.NotifyTeamFunc(ctx, teamID, scope, templateID, alert)
	}
	return nil
}

func (m *MockNotificationService) NotifyChannel(ctx context.Context, target *routingv1.NotificationTarget, templateID string, alert *routingv1.Alert) error {
	if m.NotifyChannelFunc != nil {
		return m.NotifyChannelFunc(ctx, target, templateID, alert)
	}
	return nil
}

func (m *MockNotificationService) NotifyUser(ctx context.Context, userID string, templateID string, channelOverride routingv1.ChannelType, alert *routingv1.Alert) error {
	if m.NotifyUserFunc != nil {
		return m.NotifyUserFunc(ctx, userID, templateID, channelOverride, alert)
	}
	return nil
}

func (m *MockNotificationService) NotifyOnCall(ctx context.Context, scheduleID string, templateID string, level routingv1.OnCallLevel, alert *routingv1.Alert) error {
	if m.NotifyOnCallFunc != nil {
		return m.NotifyOnCallFunc(ctx, scheduleID, templateID, level, alert)
	}
	return nil
}

// MockAlertService is a mock implementation of AlertService.
type MockAlertService struct {
	SuppressAlertFunc  func(ctx context.Context, alertID string, reason string, duration time.Duration, logSuppression bool) error
	AggregateAlertFunc func(ctx context.Context, alert *routingv1.Alert, groupBy []string, window time.Duration, maxAlerts int32) error
	SetLabelsFunc      func(ctx context.Context, alertID string, labels map[string]string, overwrite bool) error
}

func (m *MockAlertService) SuppressAlert(ctx context.Context, alertID string, reason string, duration time.Duration, logSuppression bool) error {
	if m.SuppressAlertFunc != nil {
		return m.SuppressAlertFunc(ctx, alertID, reason, duration, logSuppression)
	}
	return nil
}

func (m *MockAlertService) AggregateAlert(ctx context.Context, alert *routingv1.Alert, groupBy []string, window time.Duration, maxAlerts int32) error {
	if m.AggregateAlertFunc != nil {
		return m.AggregateAlertFunc(ctx, alert, groupBy, window, maxAlerts)
	}
	return nil
}

func (m *MockAlertService) SetLabels(ctx context.Context, alertID string, labels map[string]string, overwrite bool) error {
	if m.SetLabelsFunc != nil {
		return m.SetLabelsFunc(ctx, alertID, labels, overwrite)
	}
	return nil
}

// MockEscalationService is a mock implementation of EscalationService.
type MockEscalationService struct {
	EscalateFunc func(ctx context.Context, alertID string, policyID string, startAtStep int32, urgent bool) error
}

func (m *MockEscalationService) Escalate(ctx context.Context, alertID string, policyID string, startAtStep int32, urgent bool) error {
	if m.EscalateFunc != nil {
		return m.EscalateFunc(ctx, alertID, policyID, startAtStep, urgent)
	}
	return nil
}

// MockTicketService is a mock implementation of TicketService.
type MockTicketService struct {
	CreateTicketFunc func(ctx context.Context, providerID, projectKey, ticketType, templateID string, fields map[string]string, alert *routingv1.Alert) (string, error)
}

func (m *MockTicketService) CreateTicket(ctx context.Context, providerID, projectKey, ticketType, templateID string, fields map[string]string, alert *routingv1.Alert) (string, error) {
	if m.CreateTicketFunc != nil {
		return m.CreateTicketFunc(ctx, providerID, projectKey, ticketType, templateID, fields, alert)
	}
	return "TICKET-123", nil
}

func TestNewNotifyTeamHandler(t *testing.T) {
	tests := []struct {
		name           string
		action         *routingv1.RoutingAction
		mockErr        error
		expectedResult bool
		expectedError  bool
	}{
		{
			name: "successful notification",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM,
				NotifyTeam: &routingv1.NotifyTeamAction{
					TeamId:     "team-123",
					Scope:      routingv1.TeamNotifyScope_TEAM_NOTIFY_SCOPE_ALL,
					TemplateId: "template-1",
				},
			},
			mockErr:        nil,
			expectedResult: true,
			expectedError:  false,
		},
		{
			name: "missing team config",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM,
			},
			mockErr:        nil,
			expectedResult: false,
			expectedError:  true,
		},
		{
			name: "empty team ID",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM,
				NotifyTeam: &routingv1.NotifyTeamAction{
					TeamId: "",
				},
			},
			mockErr:        nil,
			expectedResult: false,
			expectedError:  true,
		},
		{
			name: "notification service error",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM,
				NotifyTeam: &routingv1.NotifyTeamAction{
					TeamId: "team-123",
					Scope:  routingv1.TeamNotifyScope_TEAM_NOTIFY_SCOPE_ONCALL,
				},
			},
			mockErr:        errors.New("notification failed"),
			expectedResult: false,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := &MockNotificationService{
				NotifyTeamFunc: func(ctx context.Context, teamID string, scope routingv1.TeamNotifyScope, templateID string, alert *routingv1.Alert) error {
					return tt.mockErr
				},
			}

			handler := NewNotifyTeamHandler(mockSvc)
			alert := &routingv1.Alert{Id: "alert-1"}

			result, err := handler(context.Background(), alert, tt.action)

			if (err != nil) != tt.expectedError {
				t.Errorf("handler error = %v, expected error = %v", err, tt.expectedError)
			}

			if result.Success != tt.expectedResult {
				t.Errorf("result.Success = %v, expected %v", result.Success, tt.expectedResult)
			}
		})
	}
}

func TestNewNotifyChannelHandler(t *testing.T) {
	tests := []struct {
		name           string
		action         *routingv1.RoutingAction
		expectedResult bool
		expectedError  bool
	}{
		{
			name: "successful notification",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_CHANNEL,
				NotifyChannel: &routingv1.NotifyChannelAction{
					Target: &routingv1.NotificationTarget{
						Channel: routingv1.ChannelType_CHANNEL_TYPE_SLACK,
						Slack:   &routingv1.SlackTarget{ChannelId: "#alerts"},
					},
					TemplateId: "template-1",
				},
			},
			expectedResult: true,
			expectedError:  false,
		},
		{
			name: "missing channel config",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_CHANNEL,
			},
			expectedResult: false,
			expectedError:  true,
		},
		{
			name: "missing target",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_CHANNEL,
				NotifyChannel: &routingv1.NotifyChannelAction{
					TemplateId: "template-1",
				},
			},
			expectedResult: false,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := &MockNotificationService{}
			handler := NewNotifyChannelHandler(mockSvc)
			alert := &routingv1.Alert{Id: "alert-1"}

			result, err := handler(context.Background(), alert, tt.action)

			if (err != nil) != tt.expectedError {
				t.Errorf("handler error = %v, expected error = %v", err, tt.expectedError)
			}

			if result.Success != tt.expectedResult {
				t.Errorf("result.Success = %v, expected %v", result.Success, tt.expectedResult)
			}
		})
	}
}

func TestNewNotifyUserHandler(t *testing.T) {
	tests := []struct {
		name           string
		action         *routingv1.RoutingAction
		expectedResult bool
		expectedError  bool
	}{
		{
			name: "successful notification",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_USER,
				NotifyUser: &routingv1.NotifyUserAction{
					UserId:     "user-123",
					TemplateId: "template-1",
				},
			},
			expectedResult: true,
			expectedError:  false,
		},
		{
			name: "missing user config",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_USER,
			},
			expectedResult: false,
			expectedError:  true,
		},
		{
			name: "empty user ID",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_USER,
				NotifyUser: &routingv1.NotifyUserAction{
					UserId: "",
				},
			},
			expectedResult: false,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := &MockNotificationService{}
			handler := NewNotifyUserHandler(mockSvc)
			alert := &routingv1.Alert{Id: "alert-1"}

			result, err := handler(context.Background(), alert, tt.action)

			if (err != nil) != tt.expectedError {
				t.Errorf("handler error = %v, expected error = %v", err, tt.expectedError)
			}

			if result.Success != tt.expectedResult {
				t.Errorf("result.Success = %v, expected %v", result.Success, tt.expectedResult)
			}
		})
	}
}

func TestNewNotifyOnCallHandler(t *testing.T) {
	tests := []struct {
		name           string
		action         *routingv1.RoutingAction
		expectedResult bool
		expectedError  bool
	}{
		{
			name: "successful notification",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_ONCALL,
				NotifyOncall: &routingv1.NotifyOnCallAction{
					ScheduleId: "schedule-123",
					TemplateId: "template-1",
					Level:      routingv1.OnCallLevel_ONCALL_LEVEL_PRIMARY,
				},
			},
			expectedResult: true,
			expectedError:  false,
		},
		{
			name: "missing oncall config",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_ONCALL,
			},
			expectedResult: false,
			expectedError:  true,
		},
		{
			name: "empty schedule ID",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_NOTIFY_ONCALL,
				NotifyOncall: &routingv1.NotifyOnCallAction{
					ScheduleId: "",
				},
			},
			expectedResult: false,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := &MockNotificationService{}
			handler := NewNotifyOnCallHandler(mockSvc)
			alert := &routingv1.Alert{Id: "alert-1"}

			result, err := handler(context.Background(), alert, tt.action)

			if (err != nil) != tt.expectedError {
				t.Errorf("handler error = %v, expected error = %v", err, tt.expectedError)
			}

			if result.Success != tt.expectedResult {
				t.Errorf("result.Success = %v, expected %v", result.Success, tt.expectedResult)
			}
		})
	}
}

func TestNewSuppressHandler(t *testing.T) {
	tests := []struct {
		name           string
		action         *routingv1.RoutingAction
		mockErr        error
		expectedResult bool
		expectedError  bool
	}{
		{
			name: "successful suppression",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_SUPPRESS,
				Suppress: &routingv1.SuppressAction{
					Reason:         "maintenance",
					Duration:       durationpb.New(time.Hour),
					LogSuppression: true,
				},
			},
			mockErr:        nil,
			expectedResult: true,
			expectedError:  false,
		},
		{
			name: "missing suppress config",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_SUPPRESS,
			},
			mockErr:        nil,
			expectedResult: false,
			expectedError:  true,
		},
		{
			name: "service error",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_SUPPRESS,
				Suppress: &routingv1.SuppressAction{
					Reason: "test",
				},
			},
			mockErr:        errors.New("suppress failed"),
			expectedResult: false,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := &MockAlertService{
				SuppressAlertFunc: func(ctx context.Context, alertID string, reason string, duration time.Duration, logSuppression bool) error {
					return tt.mockErr
				},
			}

			handler := NewSuppressHandler(mockSvc)
			alert := &routingv1.Alert{Id: "alert-1"}

			result, err := handler(context.Background(), alert, tt.action)

			if (err != nil) != tt.expectedError {
				t.Errorf("handler error = %v, expected error = %v", err, tt.expectedError)
			}

			if result.Success != tt.expectedResult {
				t.Errorf("result.Success = %v, expected %v", result.Success, tt.expectedResult)
			}
		})
	}
}

func TestNewAggregateHandler(t *testing.T) {
	tests := []struct {
		name           string
		action         *routingv1.RoutingAction
		expectedResult bool
		expectedError  bool
	}{
		{
			name: "successful aggregation",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_AGGREGATE,
				Aggregate: &routingv1.AggregateAction{
					GroupBy:   []string{"service", "severity"},
					Window:    durationpb.New(5 * time.Minute),
					MaxAlerts: 10,
				},
			},
			expectedResult: true,
			expectedError:  false,
		},
		{
			name: "missing aggregate config",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_AGGREGATE,
			},
			expectedResult: false,
			expectedError:  true,
		},
		{
			name: "empty group_by",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_AGGREGATE,
				Aggregate: &routingv1.AggregateAction{
					GroupBy: []string{},
				},
			},
			expectedResult: false,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := &MockAlertService{}
			handler := NewAggregateHandler(mockSvc)
			alert := &routingv1.Alert{Id: "alert-1"}

			result, err := handler(context.Background(), alert, tt.action)

			if (err != nil) != tt.expectedError {
				t.Errorf("handler error = %v, expected error = %v", err, tt.expectedError)
			}

			if result.Success != tt.expectedResult {
				t.Errorf("result.Success = %v, expected %v", result.Success, tt.expectedResult)
			}
		})
	}
}

func TestNewEscalateHandler(t *testing.T) {
	tests := []struct {
		name           string
		action         *routingv1.RoutingAction
		expectedResult bool
		expectedError  bool
	}{
		{
			name: "successful escalation",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_ESCALATE,
				Escalate: &routingv1.EscalateAction{
					EscalationPolicyId: "policy-123",
					StartAtStep:        0,
					Urgent:             false,
				},
			},
			expectedResult: true,
			expectedError:  false,
		},
		{
			name: "urgent escalation",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_ESCALATE,
				Escalate: &routingv1.EscalateAction{
					EscalationPolicyId: "policy-123",
					StartAtStep:        2,
					Urgent:             true,
				},
			},
			expectedResult: true,
			expectedError:  false,
		},
		{
			name: "missing escalate config",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_ESCALATE,
			},
			expectedResult: false,
			expectedError:  true,
		},
		{
			name: "empty policy ID",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_ESCALATE,
				Escalate: &routingv1.EscalateAction{
					EscalationPolicyId: "",
				},
			},
			expectedResult: false,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := &MockEscalationService{}
			handler := NewEscalateHandler(mockSvc)
			alert := &routingv1.Alert{Id: "alert-1"}

			result, err := handler(context.Background(), alert, tt.action)

			if (err != nil) != tt.expectedError {
				t.Errorf("handler error = %v, expected error = %v", err, tt.expectedError)
			}

			if result.Success != tt.expectedResult {
				t.Errorf("result.Success = %v, expected %v", result.Success, tt.expectedResult)
			}
		})
	}
}

func TestNewCreateTicketHandler(t *testing.T) {
	tests := []struct {
		name           string
		action         *routingv1.RoutingAction
		expectedResult bool
		expectedError  bool
	}{
		{
			name: "successful ticket creation",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_CREATE_TICKET,
				CreateTicket: &routingv1.CreateTicketAction{
					ProviderId: "jira",
					ProjectKey: "OPS",
					TicketType: "Bug",
					TemplateId: "template-1",
					Fields: map[string]string{
						"priority": "high",
					},
				},
			},
			expectedResult: true,
			expectedError:  false,
		},
		{
			name: "missing ticket config",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_CREATE_TICKET,
			},
			expectedResult: false,
			expectedError:  true,
		},
		{
			name: "empty provider ID",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_CREATE_TICKET,
				CreateTicket: &routingv1.CreateTicketAction{
					ProviderId: "",
				},
			},
			expectedResult: false,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := &MockTicketService{}
			handler := NewCreateTicketHandler(mockSvc)
			alert := &routingv1.Alert{Id: "alert-1"}

			result, err := handler(context.Background(), alert, tt.action)

			if (err != nil) != tt.expectedError {
				t.Errorf("handler error = %v, expected error = %v", err, tt.expectedError)
			}

			if result.Success != tt.expectedResult {
				t.Errorf("result.Success = %v, expected %v", result.Success, tt.expectedResult)
			}
		})
	}
}

func TestNewSetLabelHandler(t *testing.T) {
	tests := []struct {
		name           string
		action         *routingv1.RoutingAction
		expectedResult bool
		expectedError  bool
	}{
		{
			name: "successful label set",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_SET_LABEL,
				SetLabel: &routingv1.SetLabelAction{
					Labels: map[string]string{
						"team":     "platform",
						"priority": "high",
					},
					OverwriteExisting: true,
				},
			},
			expectedResult: true,
			expectedError:  false,
		},
		{
			name: "missing set_label config",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_SET_LABEL,
			},
			expectedResult: false,
			expectedError:  true,
		},
		{
			name: "empty labels map",
			action: &routingv1.RoutingAction{
				Type: routingv1.ActionType_ACTION_TYPE_SET_LABEL,
				SetLabel: &routingv1.SetLabelAction{
					Labels: map[string]string{},
				},
			},
			expectedResult: false,
			expectedError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := &MockAlertService{}
			handler := NewSetLabelHandler(mockSvc)
			alert := &routingv1.Alert{Id: "alert-1"}

			result, err := handler(context.Background(), alert, tt.action)

			if (err != nil) != tt.expectedError {
				t.Errorf("handler error = %v, expected error = %v", err, tt.expectedError)
			}

			if result.Success != tt.expectedResult {
				t.Errorf("result.Success = %v, expected %v", result.Success, tt.expectedResult)
			}
		})
	}
}

func TestRegisterAllHandlers(t *testing.T) {
	logger := zerolog.Nop()
	metrics := NewMetrics()
	executor := NewDefaultExecutor(nil, logger, metrics)

	handlers := &ActionHandlers{
		NotificationService: &MockNotificationService{},
		AlertService:        &MockAlertService{},
		EscalationService:   &MockEscalationService{},
		TicketService:       &MockTicketService{},
	}

	RegisterAllHandlers(executor, handlers)

	registered := executor.GetRegisteredActions()

	// Should have registered: notify_team, notify_channel, notify_user, notify_oncall,
	// suppress, aggregate, set_label, escalate, create_ticket
	if len(registered) != 9 {
		t.Errorf("Expected 9 registered handlers, got %d", len(registered))
	}
}
