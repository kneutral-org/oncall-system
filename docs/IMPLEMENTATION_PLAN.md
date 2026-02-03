# OnCall System - MVP2 Implementation Plan

## Overview

This document contains the detailed implementation plan for the OnCall System MVP2, broken down into testable, measurable tasks organized by service.

## Repositories Structure

| Repository | Service | Description |
|------------|---------|-------------|
| `oncall-system` | Monorepo root | Shared protos, docs, CI/CD |
| `alerting-service` | Project 1 | Alert ingestion, escalation, scheduling |
| `notification-service` | Project 4 | Template management, notification delivery |
| `ticket-service` | Project 3 | Extensible ticketing (Salesforce, Jira) |

## Labels Schema

| Label | Color | Description |
|-------|-------|-------------|
| `mvp2` | `#0E8A16` | MVP2 milestone |
| `service:alerting` | `#FFA500` | Alerting service tasks |
| `service:notification` | `#9B59B6` | Notification service tasks |
| `service:ticket` | `#E91E63` | Ticket service tasks |
| `service:proto` | `#607D8B` | Shared protobuf definitions |
| `service:docs` | `#00BCD4` | Documentation |
| `type:feature` | `#1D76DB` | New feature |
| `type:infra` | `#5319E7` | Infrastructure/scaffolding |
| `type:test` | `#FBCA04` | Test coverage |
| `type:integration` | `#B60205` | Service integration |

---

## Phase 1: Foundation & Proto Definitions

### 1.1 Shared Proto Definitions (oncall-system repo)

#### TASK-001: Define Alert Proto Messages
**Size:** S | **Priority:** P0
**Labels:** `mvp2`, `service:proto`, `type:feature`

**Description:**
Define the core alert-related protobuf messages that will be shared across services.

**Acceptance Criteria:**
- [ ] `Alert` message with: id, summary, details, status, source, fingerprint, created_at
- [ ] `AlertStatus` enum: TRIGGERED, ACKNOWLEDGED, RESOLVED
- [ ] `AlertSource` enum: PROMETHEUS, GRAFANA, GENERIC, MANUAL
- [ ] `labels` and `annotations` as `map<string, string>`
- [ ] Proto compiles without errors: `protoc --go_out=. --go-grpc_out=. proto/alerting/v1/*.proto`
- [ ] Generated Go code imports successfully in a test file

**Test:**
```bash
# Compile proto
protoc --go_out=. --go-grpc_out=. proto/alerting/v1/alert.proto
# Verify generated files exist
ls pkg/proto/alerting/v1/*.pb.go
# Run go vet on generated code
go vet ./pkg/proto/alerting/v1/...
```

---

#### TASK-002: Define AlertService gRPC Interface
**Size:** S | **Priority:** P0
**Labels:** `mvp2`, `service:proto`, `type:feature`

**Description:**
Define the AlertService gRPC service interface for alert CRUD and actions.

**Acceptance Criteria:**
- [ ] `CreateAlert` RPC accepting webhook payloads
- [ ] `GetAlert` RPC with alert_id parameter
- [ ] `ListAlerts` RPC with filter options (status, service_id, time range)
- [ ] `AcknowledgeAlert` RPC with user_id
- [ ] `ResolveAlert` RPC with user_id and resolution note
- [ ] `EscalateAlert` RPC to manually escalate
- [ ] All RPCs have proper request/response messages
- [ ] Proto compiles and generates valid Go server/client interfaces

**Test:**
```bash
# Compile and verify interface exists
protoc --go_out=. --go-grpc_out=. proto/alerting/v1/alert_service.proto
grep "AlertServiceServer" pkg/proto/alerting/v1/alert_service_grpc.pb.go
grep "AlertServiceClient" pkg/proto/alerting/v1/alert_service_grpc.pb.go
```

---

#### TASK-003: Define Notification Proto Messages
**Size:** S | **Priority:** P0
**Labels:** `mvp2`, `service:proto`, `type:feature`

**Description:**
Define notification-related protobuf messages for the notification service.

**Acceptance Criteria:**
- [ ] `SendNotificationRequest` with: request_id, template_id, render_context (Struct), destinations
- [ ] `Destination` message with: user_id, channel_type, channel_address
- [ ] `ChannelType` enum: SLACK, TEAMS, EMAIL, SMS, VOICE, WEBHOOK
- [ ] `NotificationPriority` enum: LOW, NORMAL, HIGH, CRITICAL
- [ ] `DeliveryStatus` message with: id, status, delivered_at, error_message
- [ ] Proto compiles without errors

**Test:**
```bash
protoc --go_out=. --go-grpc_out=. proto/notification/v1/notification.proto
go vet ./pkg/proto/notification/v1/...
```

---

#### TASK-004: Define Template Proto Messages
**Size:** S | **Priority:** P0
**Labels:** `mvp2`, `service:proto`, `type:feature`

**Description:**
Define template-related protobuf messages for template CRUD and preview.

**Acceptance Criteria:**
- [ ] `Template` message with: id, name, description, version, channels, required_variables
- [ ] `ChannelTemplate` message with: channel, content, format, metadata
- [ ] `RenderPreviewRequest` with: template_id, version, sample_data, channels
- [ ] `RenderPreviewResponse` with: previews map, validation results
- [ ] `ValidationResult` with: valid bool, errors, warnings
- [ ] Proto compiles without errors

**Test:**
```bash
protoc --go_out=. --go-grpc_out=. proto/notification/v1/template.proto
# Verify Template message exists
grep "type Template struct" pkg/proto/notification/v1/template.pb.go
```

---

#### TASK-005: Define NotificationService gRPC Interface
**Size:** S | **Priority:** P0
**Labels:** `mvp2`, `service:proto`, `type:feature`

**Description:**
Define the NotificationService and TemplateService gRPC interfaces.

**Acceptance Criteria:**
- [ ] `NotificationService.SendNotification` RPC
- [ ] `NotificationService.GetDeliveryStatus` RPC
- [ ] `TemplateService.CreateTemplate` RPC
- [ ] `TemplateService.GetTemplate` RPC
- [ ] `TemplateService.UpdateTemplate` RPC
- [ ] `TemplateService.RenderPreview` RPC
- [ ] `TemplateService.ValidateTemplate` RPC
- [ ] Both services compile and generate valid interfaces

**Test:**
```bash
protoc --go_out=. --go-grpc_out=. proto/notification/v1/service.proto
grep "NotificationServiceServer" pkg/proto/notification/v1/service_grpc.pb.go
grep "TemplateServiceServer" pkg/proto/notification/v1/service_grpc.pb.go
```

---

#### TASK-006: Define Schedule Proto Messages
**Size:** S | **Priority:** P1
**Labels:** `mvp2`, `service:proto`, `type:feature`

**Description:**
Define schedule and on-call related protobuf messages.

**Acceptance Criteria:**
- [ ] `Schedule` message with: id, name, timezone, rotations, overrides
- [ ] `Rotation` message with: id, name, type, shift_length, members
- [ ] `RotationMember` message with: user_id, position
- [ ] `Override` message with: id, user_id, start_time, end_time
- [ ] `OnCallRequest/Response` for getting current on-call users
- [ ] Proto compiles without errors

**Test:**
```bash
protoc --go_out=. --go-grpc_out=. proto/alerting/v1/schedule.proto
grep "type Schedule struct" pkg/proto/alerting/v1/schedule.pb.go
```

---

### 1.2 Alerting Service Scaffolding

#### TASK-007: Create alerting-service Repository
**Size:** XS | **Priority:** P0
**Labels:** `mvp2`, `service:alerting`, `type:infra`

**Description:**
Create and initialize the alerting-service repository with Go module and basic structure.

**Acceptance Criteria:**
- [ ] Repository created at `kneutral-org/alerting-service`
- [ ] Go module initialized: `go mod init github.com/kneutral-org/alerting-service`
- [ ] Directory structure created:
  ```
  cmd/server/main.go
  internal/alert/
  internal/escalation/
  internal/schedule/
  internal/webhook/
  internal/grpc/
  migrations/
  ```
- [ ] Basic main.go compiles: `go build ./cmd/server`
- [ ] README.md with project description

**Test:**
```bash
cd alerting-service
go mod tidy
go build ./cmd/server
```

---

#### TASK-008: Alerting Service Database Schema - Alerts Table
**Size:** S | **Priority:** P0
**Labels:** `mvp2`, `service:alerting`, `type:feature`

**Description:**
Create the alerts table migration with JSONB support for labels/annotations.

**Acceptance Criteria:**
- [ ] Migration file: `001_create_alerts_table.up.sql`
- [ ] Columns: id (UUID), summary, details, status, source, fingerprint, labels (JSONB), annotations (JSONB), created_at, acknowledged_at, acknowledged_by, resolved_at, resolved_by, external_ticket_id
- [ ] Proper indexes on: fingerprint, status, created_at, service_id
- [ ] GIN index on labels JSONB for efficient querying
- [ ] Down migration exists
- [ ] Migration applies cleanly: `migrate -path migrations -database $DATABASE_URL up`

**Test:**
```bash
# Apply migration
migrate -path migrations -database "$DATABASE_URL" up
# Verify table exists
psql $DATABASE_URL -c "\d alerts"
# Verify JSONB columns work
psql $DATABASE_URL -c "INSERT INTO alerts (id, summary, labels) VALUES (gen_random_uuid(), 'test', '{\"severity\": \"critical\"}'::jsonb) RETURNING *"
# Rollback
migrate -path migrations -database "$DATABASE_URL" down 1
```

---

#### TASK-009: Alerting Service Database Schema - Services Table
**Size:** XS | **Priority:** P0
**Labels:** `mvp2`, `service:alerting`, `type:feature`

**Description:**
Create the services table for alert categorization and integration key management.

**Acceptance Criteria:**
- [ ] Migration file: `002_create_services_table.up.sql`
- [ ] Columns: id (UUID), name, description, integration_key (unique), escalation_policy_id, created_at
- [ ] Unique constraint on integration_key
- [ ] Foreign key to alerts table (alerts.service_id)
- [ ] Down migration exists

**Test:**
```bash
migrate -path migrations -database "$DATABASE_URL" up
psql $DATABASE_URL -c "\d services"
psql $DATABASE_URL -c "INSERT INTO services (id, name, integration_key) VALUES (gen_random_uuid(), 'test-service', 'test-key-123') RETURNING *"
```

---

#### TASK-010: Alerting Service Database Schema - Escalation Policies
**Size:** S | **Priority:** P1
**Labels:** `mvp2`, `service:alerting`, `type:feature`

**Description:**
Create escalation policies and steps tables.

**Acceptance Criteria:**
- [ ] Migration: `003_create_escalation_tables.up.sql`
- [ ] `escalation_policies` table: id, name, service_id, repeat_count, created_at
- [ ] `escalation_steps` table: id, policy_id, step_number, delay_minutes, target_type, target_id
- [ ] `target_type` enum: 'user', 'schedule', 'webhook'
- [ ] Foreign keys and cascade deletes
- [ ] Index on policy_id, step_number

**Test:**
```bash
migrate -path migrations -database "$DATABASE_URL" up
psql $DATABASE_URL -c "\d escalation_policies"
psql $DATABASE_URL -c "\d escalation_steps"
# Insert test policy with steps
psql $DATABASE_URL -c "
  WITH policy AS (
    INSERT INTO escalation_policies (id, name) VALUES (gen_random_uuid(), 'test-policy') RETURNING id
  )
  INSERT INTO escalation_steps (id, policy_id, step_number, delay_minutes, target_type, target_id)
  SELECT gen_random_uuid(), policy.id, 1, 5, 'user', gen_random_uuid() FROM policy;
"
```

---

#### TASK-011: Alerting Service Database Schema - Schedules
**Size:** M | **Priority:** P1
**Labels:** `mvp2`, `service:alerting`, `type:feature`

**Description:**
Create schedules, rotations, and overrides tables.

**Acceptance Criteria:**
- [ ] Migration: `004_create_schedule_tables.up.sql`
- [ ] `schedules` table: id, name, description, timezone, created_at
- [ ] `rotations` table: id, schedule_id, name, rotation_type, shift_length_hours, start_time, handoff_time
- [ ] `rotation_members` table: rotation_id, user_id, position
- [ ] `schedule_overrides` table: id, schedule_id, user_id, start_time, end_time, override_type
- [ ] Proper indexes and constraints

**Test:**
```bash
migrate -path migrations -database "$DATABASE_URL" up
psql $DATABASE_URL -c "\d schedules"
psql $DATABASE_URL -c "\d rotations"
psql $DATABASE_URL -c "\d rotation_members"
# Create complete schedule with rotation
psql $DATABASE_URL -c "
  INSERT INTO schedules (id, name, timezone) VALUES ('11111111-1111-1111-1111-111111111111', 'Platform On-Call', 'America/New_York');
  INSERT INTO rotations (id, schedule_id, name, rotation_type, shift_length_hours)
  VALUES ('22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111', 'Weekly', 'weekly', 168);
"
```

---

#### TASK-012: Alert Store - Create and Get Operations
**Size:** M | **Priority:** P0
**Labels:** `mvp2`, `service:alerting`, `type:feature`

**Description:**
Implement the alert store with Create and Get operations using SQLC.

**Acceptance Criteria:**
- [ ] SQLC queries file: `internal/alert/queries.sql`
- [ ] `CreateAlert` function that inserts alert with JSONB labels/annotations
- [ ] `GetAlert` function by ID
- [ ] `GetAlertByFingerprint` for deduplication
- [ ] Store struct with database connection
- [ ] Unit tests with mock database
- [ ] 80%+ test coverage on store functions

**Test:**
```bash
go generate ./internal/alert/...
go test ./internal/alert/... -v -cover
# Verify coverage > 80%
go test ./internal/alert/... -coverprofile=coverage.out && go tool cover -func=coverage.out | grep total
```

---

#### TASK-013: Alert Store - List and Filter Operations
**Size:** M | **Priority:** P0
**Labels:** `mvp2`, `service:alerting`, `type:feature`

**Description:**
Implement alert listing with filters and JSONB label querying.

**Acceptance Criteria:**
- [ ] `ListAlerts` function with pagination (limit, offset, cursor)
- [ ] Filter by status (multiple statuses)
- [ ] Filter by service_id
- [ ] Filter by time range (created_at)
- [ ] Filter by label value: `labels->>'severity' = 'critical'`
- [ ] Sort options: created_at DESC (default), status
- [ ] Unit tests covering all filter combinations

**Test:**
```bash
go test ./internal/alert/... -v -run TestListAlerts
# Test specific filter scenarios
go test ./internal/alert/... -v -run "TestListAlerts/FilterByStatus"
go test ./internal/alert/... -v -run "TestListAlerts/FilterByLabels"
```

---

#### TASK-014: Alert Store - Update Operations
**Size:** S | **Priority:** P0
**Labels:** `mvp2`, `service:alerting`, `type:feature`

**Description:**
Implement alert status update operations (acknowledge, resolve).

**Acceptance Criteria:**
- [ ] `AcknowledgeAlert` function: sets status, acknowledged_at, acknowledged_by
- [ ] `ResolveAlert` function: sets status, resolved_at, resolved_by
- [ ] `UpdateAlertStatus` generic function for status transitions
- [ ] Validation: only valid transitions (TRIGGERED→ACKNOWLEDGED→RESOLVED)
- [ ] Returns updated alert
- [ ] Unit tests for valid and invalid transitions

**Test:**
```bash
go test ./internal/alert/... -v -run TestAcknowledge
go test ./internal/alert/... -v -run TestResolve
# Test invalid transition
go test ./internal/alert/... -v -run "TestUpdateStatus/InvalidTransition"
```

---

#### TASK-015: Webhook Handler - Alertmanager Format
**Size:** M | **Priority:** P0
**Labels:** `mvp2`, `service:alerting`, `type:feature`

**Description:**
Implement HTTP webhook handler for Alertmanager format.

**Acceptance Criteria:**
- [ ] HTTP handler at `POST /api/v1/webhook/alertmanager`
- [ ] Parse Alertmanager webhook payload (alerts array, commonLabels, commonAnnotations)
- [ ] Extract: status, labels, annotations, fingerprint, startsAt, endsAt
- [ ] Validate integration key from URL path or header
- [ ] Create or update alert based on fingerprint (deduplication)
- [ ] Return 200 OK with alert IDs
- [ ] Return 400 for invalid payload
- [ ] Return 401 for invalid integration key
- [ ] Integration test with sample Alertmanager payload

**Test:**
```bash
go test ./internal/webhook/... -v -run TestAlertmanagerWebhook
# Test with real payload
curl -X POST http://localhost:8080/api/v1/webhook/alertmanager/test-key \
  -H "Content-Type: application/json" \
  -d '{"alerts":[{"status":"firing","labels":{"alertname":"test","severity":"warning"},"annotations":{"summary":"Test alert"}}]}'
```

---

#### TASK-016: Webhook Handler - Grafana Format
**Size:** S | **Priority:** P1
**Labels:** `mvp2`, `service:alerting`, `type:feature`

**Description:**
Implement HTTP webhook handler for Grafana alert format.

**Acceptance Criteria:**
- [ ] HTTP handler at `POST /api/v1/webhook/grafana`
- [ ] Parse Grafana webhook payload (title, message, state, tags)
- [ ] Map Grafana state to alert status
- [ ] Extract labels from tags
- [ ] Create or update alert
- [ ] Integration test with Grafana payload sample

**Test:**
```bash
go test ./internal/webhook/... -v -run TestGrafanaWebhook
curl -X POST http://localhost:8080/api/v1/webhook/grafana/test-key \
  -H "Content-Type: application/json" \
  -d '{"title":"Test Alert","state":"alerting","tags":{"severity":"high"}}'
```

---

#### TASK-017: Webhook Handler - Generic JSON Format
**Size:** S | **Priority:** P1
**Labels:** `mvp2`, `service:alerting`, `type:feature`

**Description:**
Implement generic JSON webhook handler for custom integrations.

**Acceptance Criteria:**
- [ ] HTTP handler at `POST /api/v1/webhook/generic`
- [ ] Accept flexible JSON with required fields: summary
- [ ] Optional fields: details, labels, annotations, status
- [ ] Auto-generate fingerprint if not provided
- [ ] Create alert
- [ ] Integration test

**Test:**
```bash
go test ./internal/webhook/... -v -run TestGenericWebhook
curl -X POST http://localhost:8080/api/v1/webhook/generic/test-key \
  -H "Content-Type: application/json" \
  -d '{"summary":"Custom alert","labels":{"env":"prod"}}'
```

---

#### TASK-018: gRPC Server - AlertService Implementation
**Size:** M | **Priority:** P0
**Labels:** `mvp2`, `service:alerting`, `type:feature`

**Description:**
Implement the AlertService gRPC server.

**Acceptance Criteria:**
- [ ] Implement `AlertServiceServer` interface
- [ ] `CreateAlert` RPC connected to alert store
- [ ] `GetAlert` RPC connected to alert store
- [ ] `ListAlerts` RPC with streaming response
- [ ] `AcknowledgeAlert` RPC connected to store
- [ ] `ResolveAlert` RPC connected to store
- [ ] Proper error handling with gRPC status codes
- [ ] Unit tests for each RPC

**Test:**
```bash
go test ./internal/grpc/... -v -run TestAlertService
# Start server and test with grpcurl
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext -d '{"summary":"test"}' localhost:50051 alerting.v1.AlertService/CreateAlert
```

---

### 1.3 Notification Service Scaffolding

#### TASK-019: Create notification-service Repository
**Size:** XS | **Priority:** P0
**Labels:** `mvp2`, `service:notification`, `type:infra`

**Description:**
Create and initialize the notification-service repository.

**Acceptance Criteria:**
- [ ] Repository created at `kneutral-org/notification-service`
- [ ] Go module initialized
- [ ] Directory structure:
  ```
  cmd/server/main.go
  internal/template/
  internal/notification/
  internal/delivery/
  internal/channel/
  migrations/
  ```
- [ ] Basic main.go compiles
- [ ] README.md with project description

**Test:**
```bash
cd notification-service
go mod tidy
go build ./cmd/server
```

---

#### TASK-020: Notification Service Database Schema - Templates
**Size:** S | **Priority:** P0
**Labels:** `mvp2`, `service:notification`, `type:feature`

**Description:**
Create templates and template_versions tables.

**Acceptance Criteria:**
- [ ] Migration: `001_create_templates_table.up.sql`
- [ ] `templates` table: id, name, description, current_version, created_by_user_id, created_at, updated_at
- [ ] `template_versions` table: id, template_id, version_number, channels (JSONB), required_variables, optional_variables, created_at
- [ ] `channels` JSONB structure: `{"slack": {"content": "...", "format": "slack_blocks"}, "email": {...}}`
- [ ] Unique constraint on (template_id, version_number)
- [ ] Down migration exists

**Test:**
```bash
migrate -path migrations -database "$DATABASE_URL" up
psql $DATABASE_URL -c "\d templates"
psql $DATABASE_URL -c "\d template_versions"
# Insert test template
psql $DATABASE_URL -c "
  INSERT INTO templates (id, name) VALUES ('11111111-1111-1111-1111-111111111111', 'critical-alert');
  INSERT INTO template_versions (id, template_id, version_number, channels)
  VALUES (gen_random_uuid(), '11111111-1111-1111-1111-111111111111', 1,
    '{\"slack\": {\"content\": \"Alert: {{.alert.summary}}\", \"format\": \"text\"}}'::jsonb);
"
```

---

#### TASK-021: Notification Service Database Schema - Delivery Logs
**Size:** S | **Priority:** P1
**Labels:** `mvp2`, `service:notification`, `type:feature`

**Description:**
Create delivery_logs table for tracking notification delivery.

**Acceptance Criteria:**
- [ ] Migration: `002_create_delivery_logs_table.up.sql`
- [ ] `delivery_logs` table: id, request_id, template_id, template_version, channel, destination, status, sent_at, delivered_at, error_message, metadata (JSONB)
- [ ] `status` enum: PENDING, SENT, DELIVERED, FAILED, RETRYING
- [ ] Index on request_id (for idempotency checks)
- [ ] Index on (status, sent_at) for retry queries
- [ ] Partition by sent_at month (optional optimization)

**Test:**
```bash
migrate -path migrations -database "$DATABASE_URL" up
psql $DATABASE_URL -c "\d delivery_logs"
# Insert test delivery
psql $DATABASE_URL -c "
  INSERT INTO delivery_logs (id, request_id, template_id, template_version, channel, destination, status)
  VALUES (gen_random_uuid(), 'req-123', '11111111-1111-1111-1111-111111111111', 1, 'slack', 'U12345', 'SENT');
"
```

---

#### TASK-022: Template Store - CRUD Operations
**Size:** M | **Priority:** P0
**Labels:** `mvp2`, `service:notification`, `type:feature`

**Description:**
Implement template store with CRUD operations.

**Acceptance Criteria:**
- [ ] `CreateTemplate` function with initial version
- [ ] `GetTemplate` by ID (returns latest version)
- [ ] `GetTemplateVersion` by ID and version number
- [ ] `UpdateTemplate` creates new version (immutable versions)
- [ ] `ListTemplates` with pagination
- [ ] `ListTemplateVersions` for a template
- [ ] 80%+ test coverage

**Test:**
```bash
go test ./internal/template/... -v -cover
go test ./internal/template/... -coverprofile=coverage.out && go tool cover -func=coverage.out | grep total
```

---

#### TASK-023: Template Rendering Engine - Go Templates
**Size:** M | **Priority:** P0
**Labels:** `mvp2`, `service:notification`, `type:feature`

**Description:**
Implement template rendering engine using Go text/template.

**Acceptance Criteria:**
- [ ] `Render` function accepting template string and context map
- [ ] Support for `{{ .Labels.severity }}` syntax
- [ ] Support for `{{ .Annotations.runbook_url }}` syntax
- [ ] `default` function: `{{ .Labels.foo | default "N/A" }}`
- [ ] `upper`/`lower` functions
- [ ] `truncate` function for SMS
- [ ] Sandboxed execution with timeout (prevent infinite loops)
- [ ] Return rendered string or error with line number
- [ ] Unit tests for each function and edge cases

**Test:**
```bash
go test ./internal/template/... -v -run TestRender
# Test specific scenarios
go test ./internal/template/... -v -run "TestRender/MissingVariable"
go test ./internal/template/... -v -run "TestRender/DefaultFunction"
go test ./internal/template/... -v -run "TestRender/Timeout"
```

---

#### TASK-024: Channel Formatter - Slack Block Kit
**Size:** M | **Priority:** P0
**Labels:** `mvp2`, `service:notification`, `type:feature`

**Description:**
Implement Slack Block Kit formatter for rich message formatting.

**Acceptance Criteria:**
- [ ] `SlackFormatter` struct implementing `ChannelFormatter` interface
- [ ] Input: rendered text content
- [ ] Output: valid Slack Block Kit JSON
- [ ] Support for: header block, section block, actions block, divider
- [ ] Validate output is valid JSON
- [ ] Validate against Slack Block Kit limits (50 blocks max, text limits)
- [ ] Unit tests with sample outputs

**Test:**
```bash
go test ./internal/channel/... -v -run TestSlackFormatter
# Verify output is valid Block Kit JSON
go test ./internal/channel/... -v -run "TestSlackFormatter/ValidBlockKit"
```

---

#### TASK-025: Channel Formatter - Email HTML
**Size:** S | **Priority:** P1
**Labels:** `mvp2`, `service:notification`, `type:feature`

**Description:**
Implement email HTML formatter.

**Acceptance Criteria:**
- [ ] `EmailFormatter` struct implementing `ChannelFormatter` interface
- [ ] Input: rendered text content
- [ ] Output: HTML email with subject, body
- [ ] Support for inline CSS (email-safe)
- [ ] Generate plain text version for multipart
- [ ] Validate HTML is well-formed
- [ ] Unit tests

**Test:**
```bash
go test ./internal/channel/... -v -run TestEmailFormatter
```

---

#### TASK-026: Channel Formatter - SMS Plain Text
**Size:** XS | **Priority:** P1
**Labels:** `mvp2`, `service:notification`, `type:feature`

**Description:**
Implement SMS plain text formatter with length validation.

**Acceptance Criteria:**
- [ ] `SMSFormatter` struct implementing `ChannelFormatter` interface
- [ ] Input: rendered text content
- [ ] Output: plain text, max 160 characters
- [ ] Warning if content exceeds 160 chars
- [ ] Auto-truncate option with ellipsis
- [ ] Unit tests

**Test:**
```bash
go test ./internal/channel/... -v -run TestSMSFormatter
go test ./internal/channel/... -v -run "TestSMSFormatter/Truncation"
```

---

#### TASK-027: Template Preview RPC Implementation
**Size:** M | **Priority:** P0
**Labels:** `mvp2`, `service:notification`, `type:feature`

**Description:**
Implement the RenderPreview gRPC RPC for WYSIWYG template preview.

**Acceptance Criteria:**
- [ ] Implement `TemplateService.RenderPreview` RPC
- [ ] Accept template_id, version, sample_data (Struct), channels
- [ ] Render template with sample data
- [ ] Format for each requested channel
- [ ] Return map of channel → rendered content
- [ ] Return validation results (errors, warnings)
- [ ] Same code path as production rendering (WYSIWYG guarantee)
- [ ] Unit tests

**Test:**
```bash
go test ./internal/grpc/... -v -run TestRenderPreview
# Integration test with grpcurl
grpcurl -plaintext -d '{
  "template_id": "11111111-1111-1111-1111-111111111111",
  "sample_data": {"alert": {"summary": "Test"}, "labels": {"severity": "critical"}},
  "channels": ["SLACK", "EMAIL"]
}' localhost:50052 notification.v1.TemplateService/RenderPreview
```

---

#### TASK-028: Slack Channel Provider
**Size:** M | **Priority:** P0
**Labels:** `mvp2`, `service:notification`, `type:feature`

**Description:**
Implement Slack notification provider using Slack SDK.

**Acceptance Criteria:**
- [ ] `SlackProvider` struct implementing `ChannelProvider` interface
- [ ] `Send` method accepting destination (Slack user/channel ID) and Block Kit JSON
- [ ] Use official Slack Go SDK
- [ ] Handle rate limiting (429 responses)
- [ ] Return delivery confirmation or error
- [ ] Support for both channel and DM destinations
- [ ] Unit tests with mocked Slack API

**Test:**
```bash
go test ./internal/channel/... -v -run TestSlackProvider
# Integration test (requires SLACK_TOKEN env)
SLACK_TOKEN=xoxb-xxx go test ./internal/channel/... -v -run TestSlackProvider/Integration -tags=integration
```

---

#### TASK-029: Email Channel Provider
**Size:** S | **Priority:** P1
**Labels:** `mvp2`, `service:notification`, `type:feature`

**Description:**
Implement email notification provider using SMTP.

**Acceptance Criteria:**
- [ ] `EmailProvider` struct implementing `ChannelProvider` interface
- [ ] `Send` method accepting destination (email address) and HTML content
- [ ] SMTP configuration: host, port, username, password, from address
- [ ] Support TLS/STARTTLS
- [ ] Multipart email (HTML + plain text)
- [ ] Unit tests with mock SMTP server

**Test:**
```bash
go test ./internal/channel/... -v -run TestEmailProvider
```

---

#### TASK-030: Delivery Engine - Send with Retry
**Size:** M | **Priority:** P0
**Labels:** `mvp2`, `service:notification`, `type:feature`

**Description:**
Implement delivery engine with retry logic.

**Acceptance Criteria:**
- [ ] `DeliveryEngine` struct with channel providers registry
- [ ] `Send` method that routes to correct provider
- [ ] Exponential backoff retry (max 3 retries)
- [ ] Idempotency check using request_id
- [ ] Log delivery attempt to delivery_logs
- [ ] Update status on success/failure
- [ ] Unit tests for retry scenarios

**Test:**
```bash
go test ./internal/delivery/... -v -run TestDeliveryEngine
go test ./internal/delivery/... -v -run "TestDeliveryEngine/RetryOnFailure"
go test ./internal/delivery/... -v -run "TestDeliveryEngine/Idempotency"
```

---

#### TASK-031: gRPC Server - NotificationService Implementation
**Size:** M | **Priority:** P0
**Labels:** `mvp2`, `service:notification`, `type:feature`

**Description:**
Implement the NotificationService gRPC server.

**Acceptance Criteria:**
- [ ] Implement `NotificationServiceServer` interface
- [ ] `SendNotification` RPC:
  - Load template by ID and version
  - Render with provided context
  - Format for each destination's channel
  - Dispatch via delivery engine
  - Return request_id for tracking
- [ ] `GetDeliveryStatus` RPC
- [ ] Proper error handling
- [ ] Unit tests

**Test:**
```bash
go test ./internal/grpc/... -v -run TestNotificationService
grpcurl -plaintext -d '{
  "request_id": "req-001",
  "template_id": "11111111-1111-1111-1111-111111111111",
  "render_context": {"alert": {"summary": "Test"}},
  "destinations": [{"user_id": "user-1", "channel": "SLACK", "channel_address": "U12345"}]
}' localhost:50052 notification.v1.NotificationService/SendNotification
```

---

### 1.4 Integration Tasks

#### TASK-032: Alerting → Notification Service Integration
**Size:** M | **Priority:** P0
**Labels:** `mvp2`, `service:alerting`, `service:notification`, `type:integration`

**Description:**
Implement gRPC client in alerting-service to call notification-service.

**Acceptance Criteria:**
- [ ] gRPC client for NotificationService in alerting-service
- [ ] Connection pool with retry
- [ ] Called after alert creation/escalation
- [ ] Pass alert data as render_context (labels, annotations, etc.)
- [ ] Handle notification-service unavailability gracefully
- [ ] Integration test with both services running

**Test:**
```bash
# Start both services
docker-compose up -d alerting-service notification-service
# Create alert and verify notification sent
curl -X POST http://localhost:8080/api/v1/webhook/alertmanager/test-key \
  -H "Content-Type: application/json" \
  -d '{"alerts":[{"status":"firing","labels":{"alertname":"test"}}]}'
# Check notification service logs for send attempt
docker-compose logs notification-service | grep "SendNotification"
```

---

#### TASK-033: Alerting → kneutral-api User Lookup
**Size:** S | **Priority:** P0
**Labels:** `mvp2`, `service:alerting`, `type:integration`

**Description:**
Implement gRPC client to fetch user details from kneutral-api.

**Acceptance Criteria:**
- [ ] gRPC client for UserService (define if not exists)
- [ ] `GetUsersByIDs` method to fetch user details
- [ ] `GetUserContactMethods` method to get notification destinations
- [ ] Caching layer to reduce calls (TTL 5 minutes)
- [ ] Fallback behavior if kneutral-api unavailable
- [ ] Integration test

**Test:**
```bash
go test ./internal/user/... -v -run TestUserClient
# Integration test
go test ./internal/user/... -v -run TestUserClient/Integration -tags=integration
```

---

#### TASK-034: Notification → kneutral-api User Lookup
**Size:** S | **Priority:** P1
**Labels:** `mvp2`, `service:notification`, `type:integration`

**Description:**
Implement gRPC client in notification-service to fetch user contact methods.

**Acceptance Criteria:**
- [ ] gRPC client for UserService
- [ ] Resolve user_id to contact methods (Slack ID, email, phone)
- [ ] Caching layer
- [ ] Integration test

**Test:**
```bash
go test ./internal/user/... -v -run TestUserClient
```

---

### 1.5 Testing & Documentation

#### TASK-035: E2E Test - Alert to Notification Flow
**Size:** M | **Priority:** P0
**Labels:** `mvp2`, `type:test`, `type:integration`

**Description:**
Create end-to-end test for complete alert → notification flow.

**Acceptance Criteria:**
- [ ] Docker Compose with all services
- [ ] Test: POST Alertmanager webhook → alert created → notification sent
- [ ] Verify: Alert in database with correct status
- [ ] Verify: Delivery log created
- [ ] Verify: (Mock) Slack API called with correct payload
- [ ] Runs in CI pipeline

**Test:**
```bash
cd e2e
docker-compose up -d
go test -v -run TestAlertToNotificationFlow
docker-compose down
```

---

#### TASK-036: API Documentation - OpenAPI/gRPC
**Size:** S | **Priority:** P1
**Labels:** `mvp2`, `service:docs`, `type:feature`

**Description:**
Generate API documentation for HTTP webhooks and gRPC services.

**Acceptance Criteria:**
- [ ] OpenAPI spec for webhook endpoints
- [ ] gRPC reflection enabled on services
- [ ] Buf schema documentation generated
- [ ] Published to docs site or README
- [ ] Includes example payloads

**Test:**
```bash
# Verify OpenAPI spec is valid
swagger validate docs/openapi.yaml
# Verify gRPC reflection works
grpcurl -plaintext localhost:50051 list
```

---

## Summary

| Phase | Tasks | Priority P0 | Priority P1 |
|-------|-------|-------------|-------------|
| Proto Definitions | 6 | 5 | 1 |
| Alerting Service | 12 | 8 | 4 |
| Notification Service | 13 | 8 | 5 |
| Integration | 3 | 2 | 1 |
| Testing & Docs | 2 | 1 | 1 |
| **Total** | **36** | **24** | **12** |
