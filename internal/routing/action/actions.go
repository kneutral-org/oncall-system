package action

import (
	"context"
	"fmt"
	"time"

	routingv1 "github.com/kneutral-org/alerting-system/pkg/proto/alerting/routing/v1"
)

// NotificationService defines the interface for sending notifications.
type NotificationService interface {
	// NotifyTeam sends a notification to a team.
	NotifyTeam(ctx context.Context, teamID string, scope routingv1.TeamNotifyScope, templateID string, alert *routingv1.Alert) error
	// NotifyChannel sends a notification to a channel.
	NotifyChannel(ctx context.Context, target *routingv1.NotificationTarget, templateID string, alert *routingv1.Alert) error
	// NotifyUser sends a notification to a specific user.
	NotifyUser(ctx context.Context, userID string, templateID string, channelOverride routingv1.ChannelType, alert *routingv1.Alert) error
	// NotifyOnCall sends a notification to on-call personnel.
	NotifyOnCall(ctx context.Context, scheduleID string, templateID string, level routingv1.OnCallLevel, alert *routingv1.Alert) error
}

// AlertService defines the interface for modifying alerts.
type AlertService interface {
	// SuppressAlert marks an alert as suppressed.
	SuppressAlert(ctx context.Context, alertID string, reason string, duration time.Duration, logSuppression bool) error
	// AggregateAlert adds an alert to an aggregation group.
	AggregateAlert(ctx context.Context, alert *routingv1.Alert, groupBy []string, window time.Duration, maxAlerts int32) error
	// SetLabels updates the labels on an alert.
	SetLabels(ctx context.Context, alertID string, labels map[string]string, overwrite bool) error
}

// EscalationService defines the interface for escalation operations.
type EscalationService interface {
	// Escalate triggers an escalation policy for an alert.
	Escalate(ctx context.Context, alertID string, policyID string, startAtStep int32, urgent bool) error
}

// TicketService defines the interface for ticket creation.
type TicketService interface {
	// CreateTicket creates a ticket in an external system.
	CreateTicket(ctx context.Context, providerID, projectKey, ticketType, templateID string, fields map[string]string, alert *routingv1.Alert) (string, error)
}

// ActionHandlers holds the service dependencies for action handlers.
type ActionHandlers struct {
	NotificationService NotificationService
	AlertService        AlertService
	EscalationService   EscalationService
	TicketService       TicketService
}

// RegisterAllHandlers registers all action handlers with the executor.
func RegisterAllHandlers(executor *DefaultExecutor, handlers *ActionHandlers) {
	if handlers.NotificationService != nil {
		executor.RegisterAction(routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM, NewNotifyTeamHandler(handlers.NotificationService))
		executor.RegisterAction(routingv1.ActionType_ACTION_TYPE_NOTIFY_CHANNEL, NewNotifyChannelHandler(handlers.NotificationService))
		executor.RegisterAction(routingv1.ActionType_ACTION_TYPE_NOTIFY_USER, NewNotifyUserHandler(handlers.NotificationService))
		executor.RegisterAction(routingv1.ActionType_ACTION_TYPE_NOTIFY_ONCALL, NewNotifyOnCallHandler(handlers.NotificationService))
	}

	if handlers.AlertService != nil {
		executor.RegisterAction(routingv1.ActionType_ACTION_TYPE_SUPPRESS, NewSuppressHandler(handlers.AlertService))
		executor.RegisterAction(routingv1.ActionType_ACTION_TYPE_AGGREGATE, NewAggregateHandler(handlers.AlertService))
		executor.RegisterAction(routingv1.ActionType_ACTION_TYPE_SET_LABEL, NewSetLabelHandler(handlers.AlertService))
	}

	if handlers.EscalationService != nil {
		executor.RegisterAction(routingv1.ActionType_ACTION_TYPE_ESCALATE, NewEscalateHandler(handlers.EscalationService))
	}

	if handlers.TicketService != nil {
		executor.RegisterAction(routingv1.ActionType_ACTION_TYPE_CREATE_TICKET, NewCreateTicketHandler(handlers.TicketService))
	}
}

// NewNotifyTeamHandler creates a handler for notify_team actions.
func NewNotifyTeamHandler(svc NotificationService) ActionHandler {
	return func(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction) (*Result, error) {
		startTime := time.Now()
		config := action.GetNotifyTeam()

		if config == nil {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM.String(),
				Success:    false,
				Message:    "notify_team configuration is missing",
				Error:      ErrInvalidAction,
				Retryable:  false,
				Duration:   time.Since(startTime),
			}, ErrInvalidAction
		}

		if config.TeamId == "" {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM.String(),
				Success:    false,
				Message:    "team_id is required",
				Error:      ErrInvalidAction,
				Retryable:  false,
				Duration:   time.Since(startTime),
			}, ErrInvalidAction
		}

		err := svc.NotifyTeam(ctx, config.TeamId, config.Scope, config.TemplateId, alert)
		duration := time.Since(startTime)

		if err != nil {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM.String(),
				Success:    false,
				Message:    fmt.Sprintf("failed to notify team %s: %v", config.TeamId, err),
				Error:      err,
				Retryable:  true,
				Duration:   duration,
			}, err
		}

		return &Result{
			ActionType: routingv1.ActionType_ACTION_TYPE_NOTIFY_TEAM.String(),
			Success:    true,
			Message:    fmt.Sprintf("notified team %s with scope %s", config.TeamId, config.Scope.String()),
			Duration:   duration,
		}, nil
	}
}

// NewNotifyChannelHandler creates a handler for notify_channel actions.
func NewNotifyChannelHandler(svc NotificationService) ActionHandler {
	return func(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction) (*Result, error) {
		startTime := time.Now()
		config := action.GetNotifyChannel()

		if config == nil {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_NOTIFY_CHANNEL.String(),
				Success:    false,
				Message:    "notify_channel configuration is missing",
				Error:      ErrInvalidAction,
				Retryable:  false,
				Duration:   time.Since(startTime),
			}, ErrInvalidAction
		}

		if config.Target == nil {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_NOTIFY_CHANNEL.String(),
				Success:    false,
				Message:    "target is required",
				Error:      ErrInvalidAction,
				Retryable:  false,
				Duration:   time.Since(startTime),
			}, ErrInvalidAction
		}

		err := svc.NotifyChannel(ctx, config.Target, config.TemplateId, alert)
		duration := time.Since(startTime)

		if err != nil {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_NOTIFY_CHANNEL.String(),
				Success:    false,
				Message:    fmt.Sprintf("failed to notify channel: %v", err),
				Error:      err,
				Retryable:  true,
				Duration:   duration,
			}, err
		}

		return &Result{
			ActionType: routingv1.ActionType_ACTION_TYPE_NOTIFY_CHANNEL.String(),
			Success:    true,
			Message:    fmt.Sprintf("notified channel %s", config.Target.Channel.String()),
			Duration:   duration,
		}, nil
	}
}

// NewNotifyUserHandler creates a handler for notify_user actions.
func NewNotifyUserHandler(svc NotificationService) ActionHandler {
	return func(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction) (*Result, error) {
		startTime := time.Now()
		config := action.GetNotifyUser()

		if config == nil {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_NOTIFY_USER.String(),
				Success:    false,
				Message:    "notify_user configuration is missing",
				Error:      ErrInvalidAction,
				Retryable:  false,
				Duration:   time.Since(startTime),
			}, ErrInvalidAction
		}

		if config.UserId == "" {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_NOTIFY_USER.String(),
				Success:    false,
				Message:    "user_id is required",
				Error:      ErrInvalidAction,
				Retryable:  false,
				Duration:   time.Since(startTime),
			}, ErrInvalidAction
		}

		err := svc.NotifyUser(ctx, config.UserId, config.TemplateId, config.ChannelOverride, alert)
		duration := time.Since(startTime)

		if err != nil {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_NOTIFY_USER.String(),
				Success:    false,
				Message:    fmt.Sprintf("failed to notify user %s: %v", config.UserId, err),
				Error:      err,
				Retryable:  true,
				Duration:   duration,
			}, err
		}

		return &Result{
			ActionType: routingv1.ActionType_ACTION_TYPE_NOTIFY_USER.String(),
			Success:    true,
			Message:    fmt.Sprintf("notified user %s", config.UserId),
			Duration:   duration,
		}, nil
	}
}

// NewNotifyOnCallHandler creates a handler for notify_oncall actions.
func NewNotifyOnCallHandler(svc NotificationService) ActionHandler {
	return func(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction) (*Result, error) {
		startTime := time.Now()
		config := action.GetNotifyOncall()

		if config == nil {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_NOTIFY_ONCALL.String(),
				Success:    false,
				Message:    "notify_oncall configuration is missing",
				Error:      ErrInvalidAction,
				Retryable:  false,
				Duration:   time.Since(startTime),
			}, ErrInvalidAction
		}

		if config.ScheduleId == "" {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_NOTIFY_ONCALL.String(),
				Success:    false,
				Message:    "schedule_id is required",
				Error:      ErrInvalidAction,
				Retryable:  false,
				Duration:   time.Since(startTime),
			}, ErrInvalidAction
		}

		err := svc.NotifyOnCall(ctx, config.ScheduleId, config.TemplateId, config.Level, alert)
		duration := time.Since(startTime)

		if err != nil {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_NOTIFY_ONCALL.String(),
				Success:    false,
				Message:    fmt.Sprintf("failed to notify on-call for schedule %s: %v", config.ScheduleId, err),
				Error:      err,
				Retryable:  true,
				Duration:   duration,
			}, err
		}

		return &Result{
			ActionType: routingv1.ActionType_ACTION_TYPE_NOTIFY_ONCALL.String(),
			Success:    true,
			Message:    fmt.Sprintf("notified on-call for schedule %s at level %s", config.ScheduleId, config.Level.String()),
			Duration:   duration,
		}, nil
	}
}

// NewSuppressHandler creates a handler for suppress actions.
func NewSuppressHandler(svc AlertService) ActionHandler {
	return func(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction) (*Result, error) {
		startTime := time.Now()
		config := action.GetSuppress()

		if config == nil {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_SUPPRESS.String(),
				Success:    false,
				Message:    "suppress configuration is missing",
				Error:      ErrInvalidAction,
				Retryable:  false,
				Duration:   time.Since(startTime),
			}, ErrInvalidAction
		}

		duration := time.Duration(0)
		if config.Duration != nil {
			duration = config.Duration.AsDuration()
		}

		err := svc.SuppressAlert(ctx, alert.Id, config.Reason, duration, config.LogSuppression)
		executionDuration := time.Since(startTime)

		if err != nil {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_SUPPRESS.String(),
				Success:    false,
				Message:    fmt.Sprintf("failed to suppress alert: %v", err),
				Error:      err,
				Retryable:  false, // Suppression should not be retried automatically
				Duration:   executionDuration,
			}, err
		}

		return &Result{
			ActionType: routingv1.ActionType_ACTION_TYPE_SUPPRESS.String(),
			Success:    true,
			Message:    fmt.Sprintf("alert suppressed for %v: %s", duration, config.Reason),
			Duration:   executionDuration,
		}, nil
	}
}

// NewAggregateHandler creates a handler for aggregate actions.
func NewAggregateHandler(svc AlertService) ActionHandler {
	return func(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction) (*Result, error) {
		startTime := time.Now()
		config := action.GetAggregate()

		if config == nil {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_AGGREGATE.String(),
				Success:    false,
				Message:    "aggregate configuration is missing",
				Error:      ErrInvalidAction,
				Retryable:  false,
				Duration:   time.Since(startTime),
			}, ErrInvalidAction
		}

		if len(config.GroupBy) == 0 {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_AGGREGATE.String(),
				Success:    false,
				Message:    "group_by is required",
				Error:      ErrInvalidAction,
				Retryable:  false,
				Duration:   time.Since(startTime),
			}, ErrInvalidAction
		}

		window := time.Duration(0)
		if config.Window != nil {
			window = config.Window.AsDuration()
		}

		err := svc.AggregateAlert(ctx, alert, config.GroupBy, window, config.MaxAlerts)
		duration := time.Since(startTime)

		if err != nil {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_AGGREGATE.String(),
				Success:    false,
				Message:    fmt.Sprintf("failed to aggregate alert: %v", err),
				Error:      err,
				Retryable:  true,
				Duration:   duration,
			}, err
		}

		return &Result{
			ActionType: routingv1.ActionType_ACTION_TYPE_AGGREGATE.String(),
			Success:    true,
			Message:    fmt.Sprintf("alert added to aggregation group by %v with window %v", config.GroupBy, window),
			Duration:   duration,
		}, nil
	}
}

// NewEscalateHandler creates a handler for escalate actions.
func NewEscalateHandler(svc EscalationService) ActionHandler {
	return func(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction) (*Result, error) {
		startTime := time.Now()
		config := action.GetEscalate()

		if config == nil {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_ESCALATE.String(),
				Success:    false,
				Message:    "escalate configuration is missing",
				Error:      ErrInvalidAction,
				Retryable:  false,
				Duration:   time.Since(startTime),
			}, ErrInvalidAction
		}

		if config.EscalationPolicyId == "" {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_ESCALATE.String(),
				Success:    false,
				Message:    "escalation_policy_id is required",
				Error:      ErrInvalidAction,
				Retryable:  false,
				Duration:   time.Since(startTime),
			}, ErrInvalidAction
		}

		err := svc.Escalate(ctx, alert.Id, config.EscalationPolicyId, config.StartAtStep, config.Urgent)
		duration := time.Since(startTime)

		if err != nil {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_ESCALATE.String(),
				Success:    false,
				Message:    fmt.Sprintf("failed to escalate alert: %v", err),
				Error:      err,
				Retryable:  true,
				Duration:   duration,
			}, err
		}

		message := fmt.Sprintf("escalated alert using policy %s", config.EscalationPolicyId)
		if config.Urgent {
			message += " (urgent)"
		}
		if config.StartAtStep > 0 {
			message += fmt.Sprintf(" starting at step %d", config.StartAtStep)
		}

		return &Result{
			ActionType: routingv1.ActionType_ACTION_TYPE_ESCALATE.String(),
			Success:    true,
			Message:    message,
			Duration:   duration,
		}, nil
	}
}

// NewCreateTicketHandler creates a handler for create_ticket actions.
func NewCreateTicketHandler(svc TicketService) ActionHandler {
	return func(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction) (*Result, error) {
		startTime := time.Now()
		config := action.GetCreateTicket()

		if config == nil {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_CREATE_TICKET.String(),
				Success:    false,
				Message:    "create_ticket configuration is missing",
				Error:      ErrInvalidAction,
				Retryable:  false,
				Duration:   time.Since(startTime),
			}, ErrInvalidAction
		}

		if config.ProviderId == "" {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_CREATE_TICKET.String(),
				Success:    false,
				Message:    "provider_id is required",
				Error:      ErrInvalidAction,
				Retryable:  false,
				Duration:   time.Since(startTime),
			}, ErrInvalidAction
		}

		ticketID, err := svc.CreateTicket(ctx, config.ProviderId, config.ProjectKey, config.TicketType, config.TemplateId, config.Fields, alert)
		duration := time.Since(startTime)

		if err != nil {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_CREATE_TICKET.String(),
				Success:    false,
				Message:    fmt.Sprintf("failed to create ticket: %v", err),
				Error:      err,
				Retryable:  true,
				Duration:   duration,
			}, err
		}

		return &Result{
			ActionType: routingv1.ActionType_ACTION_TYPE_CREATE_TICKET.String(),
			Success:    true,
			Message:    fmt.Sprintf("created ticket %s in %s/%s", ticketID, config.ProviderId, config.ProjectKey),
			Duration:   duration,
		}, nil
	}
}

// NewSetLabelHandler creates a handler for set_label actions.
func NewSetLabelHandler(svc AlertService) ActionHandler {
	return func(ctx context.Context, alert *routingv1.Alert, action *routingv1.RoutingAction) (*Result, error) {
		startTime := time.Now()
		config := action.GetSetLabel()

		if config == nil {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_SET_LABEL.String(),
				Success:    false,
				Message:    "set_label configuration is missing",
				Error:      ErrInvalidAction,
				Retryable:  false,
				Duration:   time.Since(startTime),
			}, ErrInvalidAction
		}

		if len(config.Labels) == 0 {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_SET_LABEL.String(),
				Success:    false,
				Message:    "labels map is required and must not be empty",
				Error:      ErrInvalidAction,
				Retryable:  false,
				Duration:   time.Since(startTime),
			}, ErrInvalidAction
		}

		err := svc.SetLabels(ctx, alert.Id, config.Labels, config.OverwriteExisting)
		duration := time.Since(startTime)

		if err != nil {
			return &Result{
				ActionType: routingv1.ActionType_ACTION_TYPE_SET_LABEL.String(),
				Success:    false,
				Message:    fmt.Sprintf("failed to set labels: %v", err),
				Error:      err,
				Retryable:  false, // Label operations should not be retried automatically
				Duration:   duration,
			}, err
		}

		return &Result{
			ActionType: routingv1.ActionType_ACTION_TYPE_SET_LABEL.String(),
			Success:    true,
			Message:    fmt.Sprintf("set %d labels on alert", len(config.Labels)),
			Duration:   duration,
		}, nil
	}
}
