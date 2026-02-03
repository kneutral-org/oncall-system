# Alerting & Ticketing System Architecture Plan

## Executive Summary

**4-5 service architecture** replacing GoAlert with a simplified, gRPC-native alerting system that integrates with kneutral-api for user management, a dedicated Notification Service with Template Manager, and an extensible ticketing system for future Salesforce integration.

---

## AI Consensus Summary (Gemini 3 Pro Preview + GPT 5.2)

### Points of Agreement (Both Models)
| Topic | Consensus |
|-------|-----------|
| Notification Service | Must be separate from alerting-system ✓ |
| Routing Rules | Stay in alerting-system (business logic/policy domain) |
| Rendering | Server-side mandatory (security + consistency) |
| Dynamic Labels | Use `google.protobuf.Struct` or `map<string,string>` |
| Preview API | Essential - must use same renderer as production |
| Domain Ownership | alerting-system = "who/when", notification = "how" |

### Key Divergence: Template Manager Location

| Model | Recommendation | Confidence | Industry Pattern |
|-------|----------------|------------|------------------|
| **Gemini 3 Pro** | Separate service (Project 5) | 9/10 | Twilio Notify, Courier.com, Novu |
| **GPT 5.2** | Module inside Notification Service | 8/10 | PagerDuty, Opsgenie |

### Trade-offs

**Option A: 5 Services (Template Manager Separate)**
- ✅ Independent deployment lifecycles
- ✅ CMS-like content management for non-developers
- ✅ Template updates don't require notification service redeploy
- ❌ Requires aggressive caching to avoid latency
- ❌ Risk of render drift if formatters diverge
- ❌ More operational overhead (5 services)

**Option B: 4 Services (Template Manager Inside Notification Service)** ✅ SELECTED
- ✅ WYSIWYG guarantee - same renderer for preview and production
- ✅ Simpler architecture (4 services)
- ✅ No cross-service render drift
- ✅ Lower operational overhead
- ❌ Coupled deployment (template changes require notification redeploy)
- ❌ Harder to scale template management independently

### Decision: Option B Selected
The system will use **4 services** with Template Manager as a module inside the Notification Service, following the PagerDuty/Opsgenie pattern for guaranteed WYSIWYG preview-to-production consistency.

---

## Architecture Options

### Option A: 5 Services (Gemini Recommendation)

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                                    EXTERNAL SOURCES                                   │
│         Prometheus/Alertmanager  │  Grafana  │  Webhooks  │  Manual Triggers         │
└───────────────────────────────────────────┬──────────────────────────────────────────┘
                                            │ HTTP Webhooks
                                            ▼
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                              PROJECT 1: ALERTING-SYSTEM                              │
│  ┌────────────────────────────────────────────────────────────────────────────────┐  │
│  │  Webhook Receiver → Alert Engine → Escalation → On-Call Calculation            │  │
│  │  • Dynamic labels/annotations (JSONB)                                          │  │
│  │  • CEL-based routing rules ("which template for which alert")                  │  │
│  │  • Deduplication & status management                                           │  │
│  └───────────────────────────────┬────────────────────────────────────────────────┘  │
└──────────────────────────────────┼───────────────────────────────────────────────────┘
                                   │ gRPC: SendNotification(template_id, context, dest)
          ┌────────────────────────┴────────────────────────┐
          │                                                 │
          ▼                                                 ▼
┌───────────────────────────────┐      ┌───────────────────────────────────────────────┐
│  PROJECT 2: KNEUTRAL-API      │      │          PROJECT 4: NOTIFICATION-SERVICE      │
│  (Existing)                   │      │                                               │
│  ┌─────────────────────────┐  │      │  ┌─────────────────────────────────────────┐  │
│  │ User Management         │  │      │  │  Channel Providers                      │  │
│  │ • Authentication        │  │      │  │  • Slack (Block Kit)                    │  │
│  │ • Authorization (RBAC)  │  │      │  │  • MS Teams (Adaptive Cards)            │  │
│  │ • User profiles         │  │      │  │  • Email (HTML/Text)                    │  │
│  │ • Contact methods       │◄─┼──────│  │  • SMS/Voice (Twilio)                   │  │
│  └─────────────────────────┘  │ gRPC │  │  • Push Notifications                   │  │
│                               │      │  │  • Webhooks                             │  │
│  ┌─────────────────────────┐  │      │  └─────────────────────────────────────────┘  │
│  │ Alert UI / Dashboard    │  │      │                                               │
│  │ • Template Editor UI    │  │      │  ┌─────────────────────────────────────────┐  │
│  │ • Preview rendering     │  │      │  │  Delivery Engine                        │  │
│  │ • Alert actions         │  │      │  │  • Rate limiting                        │  │
│  └─────────────────────────┘  │      │  │  • Retries with backoff                 │  │
│                               │      │  │  • Idempotency keys                     │  │
│  REST API for Frontend        │      │  │  • Delivery status tracking             │  │
└───────────────────────────────┘      │  │  • Provider fallback (SMS→Voice)        │  │
                                       │  └─────────────────────────────────────────┘  │
                                       │                      │                        │
                                       │                      │ gRPC: GetTemplate      │
                                       │                      ▼                        │
                                       │  ┌─────────────────────────────────────────┐  │
                                       │  │  Template Cache (Redis)                 │  │
                                       │  │  • Aggressive caching                   │  │
                                       │  │  • Cache invalidation on update         │  │
                                       │  └──────────────────┬──────────────────────┘  │
                                       └──────────────────────┼────────────────────────┘
                                                              │ gRPC
                                                              ▼
                                       ┌───────────────────────────────────────────────┐
                                       │          PROJECT 5: TEMPLATE-MANAGER          │
                                       │                                               │
                                       │  ┌─────────────────────────────────────────┐  │
                                       │  │  Template Storage                       │  │
                                       │  │  • CRUD operations                      │  │
                                       │  │  • Versioning (immutable versions)      │  │
                                       │  │  • Inheritance/composition              │  │
                                       │  └─────────────────────────────────────────┘  │
                                       │                                               │
                                       │  ┌─────────────────────────────────────────┐  │
                                       │  │  Preview Engine                         │  │
                                       │  │  • RenderPreview RPC                    │  │
                                       │  │  • Channel-specific formatters          │  │
                                       │  │  • Validation (length, required vars)   │  │
                                       │  └─────────────────────────────────────────┘  │
                                       │                                               │
                                       │  ┌─────────────────────────────────────────┐  │
                                       │  │  Variable System                        │  │
                                       │  │  • {{ .Labels.severity }}               │  │
                                       │  │  • {{ .Annotations.runbook_url }}       │  │
                                       │  │  • Safe accessors with defaults         │  │
                                       │  │  • "Blessed" keys validation            │  │
                                       │  └─────────────────────────────────────────┘  │
                                       └───────────────────────────────────────────────┘
                                                              │
          ┌───────────────────────────────────────────────────┘
          │ gRPC
          ▼
┌───────────────────────────────────────────────────────────────────────────────────────┐
│                              PROJECT 3: TICKET-SYSTEM                                 │
│  ┌─────────────────────────────────────────────────────────────────────────────────┐  │
│  │  Provider Registry (Plugin Architecture)                                        │  │
│  │  • Salesforce Provider (REST + Pub/Sub API)                                     │  │
│  │  • Jira Provider                                                                │  │
│  │  • ServiceNow Provider                                                          │  │
│  │  • Generic Webhook Provider                                                     │  │
│  └─────────────────────────────────────────────────────────────────────────────────┘  │
│  ┌─────────────────────────────────────────────────────────────────────────────────┐  │
│  │  Bidirectional Sync Engine                                                      │  │
│  │  • Alert ↔ Ticket status synchronization                                        │  │
│  │  • Field transformation (CEL-based)                                             │  │
│  └─────────────────────────────────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────────────────────────────────┘
```

---

### Option B: 4 Services (GPT 5.2 Recommendation)

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                                    EXTERNAL SOURCES                                   │
│         Prometheus/Alertmanager  │  Grafana  │  Webhooks  │  Manual Triggers         │
└───────────────────────────────────────────┬──────────────────────────────────────────┘
                                            │ HTTP Webhooks
                                            ▼
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                              PROJECT 1: ALERTING-SYSTEM                              │
│  ┌────────────────────────────────────────────────────────────────────────────────┐  │
│  │  Webhook Receiver → Alert Engine → Escalation → On-Call Calculation            │  │
│  │  • Dynamic labels/annotations (JSONB)                                          │  │
│  │  • CEL-based routing rules ("which template for which alert")                  │  │
│  │  • Deduplication & status management                                           │  │
│  └───────────────────────────────┬────────────────────────────────────────────────┘  │
└──────────────────────────────────┼───────────────────────────────────────────────────┘
                                   │ gRPC: SendNotification(template_id, context, dest)
          ┌────────────────────────┴────────────────────────┐
          │                                                 │
          ▼                                                 ▼
┌───────────────────────────────┐      ┌───────────────────────────────────────────────┐
│  PROJECT 2: KNEUTRAL-API      │      │   PROJECT 4: NOTIFICATION-SERVICE             │
│  (Existing)                   │      │   (WITH INTEGRATED TEMPLATE MANAGER)          │
│  ┌─────────────────────────┐  │      │                                               │
│  │ User Management         │  │      │  ┌─────────────────────────────────────────┐  │
│  │ • Authentication        │  │      │  │  gRPC Services                          │  │
│  │ • Authorization (RBAC)  │  │      │  │  ├── NotificationService                │  │
│  │ • User profiles         │  │      │  │  │   • SendNotification                 │  │
│  │ • Contact methods       │◄─┼──────│  │  │   • GetDeliveryStatus                │  │
│  └─────────────────────────┘  │ gRPC │  │  │   • RegisterDestination              │  │
│                               │      │  │  │                                      │  │
│  ┌─────────────────────────┐  │      │  │  └── TemplateService (same process)    │  │
│  │ Alert UI / Dashboard    │  │      │  │      • CreateTemplate                   │  │
│  │ • Template Editor UI    │──┼──────│  │      • UpdateTemplate                   │  │
│  │ • Preview rendering     │  │ gRPC │  │      • GetTemplate                      │  │
│  │ • Alert actions         │  │      │  │      • RenderPreview ◄── WYSIWYG        │  │
│  └─────────────────────────┘  │      │  │      • ValidateTemplate                 │  │
│                               │      │  └─────────────────────────────────────────┘  │
│  REST API for Frontend        │      │                                               │
└───────────────────────────────┘      │  ┌─────────────────────────────────────────┐  │
                                       │  │  SHARED RENDERING ENGINE                │  │
                                       │  │  (Same code path for preview & send)    │  │
                                       │  │                                         │  │
                                       │  │  ┌───────────────────────────────────┐  │  │
                                       │  │  │ Channel Formatters                │  │  │
                                       │  │  │ • SlackBlockKitFormatter          │  │  │
                                       │  │  │ • TeamsAdaptiveCardFormatter      │  │  │
                                       │  │  │ • EmailHTMLFormatter              │  │  │
                                       │  │  │ • SMSPlainTextFormatter           │  │  │
                                       │  │  └───────────────────────────────────┘  │  │
                                       │  │                                         │  │
                                       │  │  ┌───────────────────────────────────┐  │  │
                                       │  │  │ Template Engine (Go templates)   │  │  │
                                       │  │  │ • {{ .Labels.severity }}          │  │  │
                                       │  │  │ • Safe accessors with defaults    │  │  │
                                       │  │  │ • Sandboxed execution             │  │  │
                                       │  │  └───────────────────────────────────┘  │  │
                                       │  └─────────────────────────────────────────┘  │
                                       │                                               │
                                       │  ┌─────────────────────────────────────────┐  │
                                       │  │  Delivery Engine                        │  │
                                       │  │  • Rate limiting (Redis token bucket)   │  │
                                       │  │  • Retries with exponential backoff     │  │
                                       │  │  • Idempotency keys                     │  │
                                       │  │  • Delivery status tracking             │  │
                                       │  │  • Provider fallback (SMS→Voice)        │  │
                                       │  └─────────────────────────────────────────┘  │
                                       │                                               │
                                       │  ┌─────────────────────────────────────────┐  │
                                       │  │  Channel Providers                      │  │
                                       │  │  • Slack, MS Teams, Email, SMS/Voice    │  │
                                       │  │  • Push Notifications, Webhooks         │  │
                                       │  └─────────────────────────────────────────┘  │
                                       └───────────────────────────┬───────────────────┘
                                                                   │ gRPC
                                                                   ▼
┌───────────────────────────────────────────────────────────────────────────────────────┐
│                              PROJECT 3: TICKET-SYSTEM                                 │
│  ┌─────────────────────────────────────────────────────────────────────────────────┐  │
│  │  Provider Registry (Plugin Architecture)                                        │  │
│  │  • Salesforce Provider (REST + Pub/Sub API)                                     │  │
│  │  • Jira Provider, ServiceNow Provider, Generic Webhook                          │  │
│  └─────────────────────────────────────────────────────────────────────────────────┘  │
│  ┌─────────────────────────────────────────────────────────────────────────────────┐  │
│  │  Bidirectional Sync Engine (Alert ↔ Ticket status sync, CEL field transform)   │  │
│  └─────────────────────────────────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────────────────────────────────┘
```

---

## System Overview

```
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                                    EXTERNAL SOURCES                                   │
├─────────────────┬─────────────────┬──────────────────┬─────────────────────────────────┤
│   Prometheus    │     Grafana     │   Alertmanager   │       Other Webhooks           │
│   Alertmanager  │   Alert Rules   │    (Direct)      │    (Site24x7, Mailgun)         │
└────────┬────────┴────────┬────────┴────────┬─────────┴──────────────┬────────────────┘
         │                 │                 │                        │
         └─────────────────┴─────────────────┴────────────────────────┘
                                      │
                               Webhooks/HTTP
                                      │
                                      ▼
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                                                                                       │
│                            ┌──────────────────────────────────────┐                   │
│                            │      ALERTING-SYSTEM (Project 1)     │                   │
│                            │                                      │                   │
│                            │  ┌─────────────────────────────────┐ │                   │
│                            │  │   Webhook Receiver Layer        │ │                   │
│                            │  │  - Alertmanager format          │ │                   │
│                            │  │  - Grafana format               │ │                   │
│                            │  │  - Generic JSON                 │ │                   │
│                            │  │  - Dynamic labels/annotations   │ │                   │
│                            │  └──────────────┬──────────────────┘ │                   │
│                            │                 │                    │                   │
│                            │                 ▼                    │                   │
│                            │  ┌─────────────────────────────────┐ │                   │
│                            │  │        Alert Engine             │ │                   │
│                            │  │  - Deduplication                │ │                   │
│                            │  │  - Status management            │ │                   │
│                            │  │  - Escalation policies          │ │                   │
│                            │  │  - On-call routing              │ │                   │
│                            │  │  - Label/annotation storage     │ │                   │
│                            │  └──────────────┬──────────────────┘ │                   │
│                            │                 │                    │                   │
│                            │        ┌────────┴────────┐           │                   │
│                            │        │    PostgreSQL   │           │                   │
│                            │        │   (Alerts DB)   │           │                   │
│                            │        └─────────────────┘           │                   │
│                            │                                      │                   │
│                            │  ┌─────────────────────────────────┐ │                   │
│                            │  │       gRPC Service Layer        │ │                   │
│                            │  │  - AlertService                 │ │                   │
│                            │  │  - EscalationService            │ │                   │
│                            │  │  - ScheduleService              │ │                   │
│                            │  │  - TicketingService             │ │                   │
│                            │  └─────────┬────────────┬──────────┘ │                   │
│                            │            │            │            │                   │
│                            └────────────┼────────────┼────────────┘                   │
│                                         │            │                                │
│           ┌─────────────────────────────┘            └─────────────────────┐          │
│           │ gRPC                                                     gRPC │          │
│           ▼                                                               ▼          │
│  ┌─────────────────────────────────┐             ┌─────────────────────────────────┐ │
│  │    KNEUTRAL-API (Project 2)     │             │   TICKET-SYSTEM (Project 3)     │ │
│  │                                 │             │       (Future - Extensible)     │ │
│  │  ┌───────────────────────────┐  │             │                                 │ │
│  │  │   User Management         │  │             │  ┌───────────────────────────┐  │ │
│  │  │   - Authentication        │  │             │  │   Provider Registry       │  │ │
│  │  │   - Authorization (RBAC)  │  │             │  │   - Salesforce            │  │ │
│  │  │   - User profiles         │  │             │  │   - Jira                  │  │ │
│  │  │   - Teams & roles         │  │             │  │   - ServiceNow            │  │ │
│  │  └───────────────────────────┘  │             │  │   - Custom webhooks       │  │ │
│  │                                 │             │  └───────────────────────────┘  │ │
│  │  ┌───────────────────────────┐  │             │                                 │ │
│  │  │   Alert UI Layer          │  │             │  ┌───────────────────────────┐  │ │
│  │  │   - Dashboard             │  │             │  │   Ticket Engine           │  │ │
│  │  │   - Alert actions         │  │             │  │   - Bidirectional sync    │  │ │
│  │  │   - Schedule management   │  │             │  │   - Status mapping        │  │ │
│  │  │   - On-call views         │  │             │  │   - Field transformation  │  │ │
│  │  └───────────────────────────┘  │             │  └───────────────────────────┘  │ │
│  │                                 │             │                                 │ │
│  │  ┌───────────────────────────┐  │             │  ┌───────────────────────────┐  │ │
│  │  │   REST API (Existing)     │  │             │  │   gRPC Service            │  │ │
│  │  │   /api/v1/alerts          │  │             │  │   - CreateTicket          │  │ │
│  │  │   /api/v1/schedules       │  │             │  │   - SyncStatus            │  │ │
│  │  │   /api/v1/oncall          │  │             │  │   - ListProviders         │  │ │
│  │  └───────────────────────────┘  │             │  └───────────────────────────┘  │ │
│  │                                 │             │                                 │ │
│  └─────────────────────────────────┘             └─────────────────────────────────┘ │
│                                                                                       │
└──────────────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
┌──────────────────────────────────────────────────────────────────────────────────────┐
│                              NOTIFICATION DESTINATIONS                                │
├───────────────┬───────────────┬───────────────┬───────────────┬─────────────────────┤
│     Slack     │   MS Teams    │  SMS/Voice    │     Email     │   Salesforce Case   │
│               │               │   (Twilio)    │    (SMTP)     │   Jira Ticket       │
└───────────────┴───────────────┴───────────────┴───────────────┴─────────────────────┘
```

---

## Component Architecture

### Project 1: alerting-system (This Project)

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                               ALERTING-SYSTEM                                        │
│                           (Simplified GoAlert Fork)                                  │
├─────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│  ┌──────────────────────────────────────────────────────────────────────────────┐   │
│  │                          INGRESS LAYER (HTTP)                                │   │
│  │                                                                              │   │
│  │  POST /api/v1/webhook/alertmanager  ────► AlertmanagerHandler               │   │
│  │  POST /api/v1/webhook/grafana       ────► GrafanaHandler                    │   │
│  │  POST /api/v1/webhook/generic       ────► GenericHandler                    │   │
│  │  POST /api/v1/heartbeat/:id         ────► HeartbeatHandler                  │   │
│  │                                                                              │   │
│  │  Common: Label/Annotation extraction, Integration Key validation            │   │
│  └──────────────────────────────────────────────────────────────────────────────┘   │
│                                          │                                          │
│                                          ▼                                          │
│  ┌──────────────────────────────────────────────────────────────────────────────┐   │
│  │                          ALERT ENGINE (Core)                                 │   │
│  │                                                                              │   │
│  │   ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐             │   │
│  │   │ Alert Store     │  │  Deduplicator   │  │ Metadata Store  │             │   │
│  │   │ - CRUD ops      │  │  - Fingerprint  │  │ - Labels        │             │   │
│  │   │ - Status mgmt   │  │  - Grouping     │  │ - Annotations   │             │   │
│  │   │ - History       │  │  - Inhibition   │  │ - Custom fields │             │   │
│  │   └─────────────────┘  └─────────────────┘  └─────────────────┘             │   │
│  │                                                                              │   │
│  │   ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐             │   │
│  │   │ Escalation Mgr  │  │  Schedule Mgr   │  │  On-Call Calc   │             │   │
│  │   │ - Policies      │  │  - Rotations    │  │  - User lookup  │             │   │
│  │   │ - Steps/delays  │  │  - Overrides    │  │  - Fallback     │             │   │
│  │   │ - Targets       │  │  - Time rules   │  │  - Coverage     │             │   │
│  │   └─────────────────┘  └─────────────────┘  └─────────────────┘             │   │
│  │                                                                              │   │
│  │   NOTE: Users NOT stored here - reference by kneutral_user_id only          │   │
│  └──────────────────────────────────────────────────────────────────────────────┘   │
│                                          │                                          │
│                                          ▼                                          │
│  ┌──────────────────────────────────────────────────────────────────────────────┐   │
│  │                          gRPC SERVICE LAYER                                  │   │
│  │                                                                              │   │
│  │   service AlertService {                                                     │   │
│  │     rpc CreateAlert(CreateAlertRequest) returns (Alert);                     │   │
│  │     rpc UpdateAlert(UpdateAlertRequest) returns (Alert);                     │   │
│  │     rpc GetAlert(GetAlertRequest) returns (Alert);                           │   │
│  │     rpc ListAlerts(ListAlertsRequest) returns (stream Alert);                │   │
│  │     rpc AcknowledgeAlert(AckRequest) returns (AckResponse);                  │   │
│  │     rpc ResolveAlert(ResolveRequest) returns (ResolveResponse);              │   │
│  │     rpc EscalateAlert(EscalateRequest) returns (EscalateResponse);           │   │
│  │   }                                                                          │   │
│  │                                                                              │   │
│  │   service ScheduleService {                                                  │   │
│  │     rpc GetOnCallUsers(OnCallRequest) returns (OnCallResponse);              │   │
│  │     rpc CreateSchedule(CreateScheduleRequest) returns (Schedule);            │   │
│  │     rpc UpdateRotation(UpdateRotationRequest) returns (Rotation);            │   │
│  │   }                                                                          │   │
│  │                                                                              │   │
│  │   service NotificationService {                                              │   │
│  │     rpc SendNotification(NotificationRequest) returns (NotificationResp);    │   │
│  │     rpc RegisterDestination(DestinationRequest) returns (DestinationResp);   │   │
│  │   }                                                                          │   │
│  │                                                                              │   │
│  │   service TicketIntegrationService {                                         │   │
│  │     rpc CreateTicketFromAlert(TicketRequest) returns (TicketResponse);       │   │
│  │     rpc SyncTicketStatus(stream StatusUpdate) returns (stream SyncAck);      │   │
│  │   }                                                                          │   │
│  └──────────────────────────────────────────────────────────────────────────────┘   │
│                                                                                      │
│  ┌──────────────────────────────────────────────────────────────────────────────┐   │
│  │                         NOTIFICATION ENGINE                                   │   │
│  │                                                                              │   │
│  │   ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │   │
│  │   │   Slack     │  │  MS Teams   │  │   Twilio    │  │   Email     │        │   │
│  │   │   Sender    │  │   Sender    │  │   Sender    │  │   Sender    │        │   │
│  │   └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘        │   │
│  │                                                                              │   │
│  │   ┌─────────────┐  ┌─────────────────────────────────────────────┐          │   │
│  │   │   Webhook   │  │   Ticket Provider (via gRPC to Project 3)   │          │   │
│  │   │   Sender    │  │   - Routes to external ticketing systems    │          │   │
│  │   └─────────────┘  └─────────────────────────────────────────────┘          │   │
│  └──────────────────────────────────────────────────────────────────────────────┘   │
│                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

---

### Project 2: kneutral-api Integration

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                                 KNEUTRAL-API                                         │
│                        (Frontend Layer & User Management)                            │
├─────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│  EXISTING (Keep)                           NEW (Add for alerting-system)            │
│  ───────────────                           ─────────────────────────────            │
│                                                                                      │
│  ┌─────────────────────────┐              ┌─────────────────────────┐               │
│  │   User Management       │              │   gRPC Client Layer     │               │
│  │   - OIDC/SAML Auth      │              │                         │               │
│  │   - Roles & Permissions │              │   ┌─────────────────┐   │               │
│  │   - Session Management  │              │   │ AlertClient     │   │               │
│  │   - User Sync Status    │◄─────────────│   │ - List alerts   │   │               │
│  └─────────────────────────┘              │   │ - Ack/Resolve   │   │               │
│                                           │   │ - Get details   │   │               │
│  ┌─────────────────────────┐              │   └─────────────────┘   │               │
│  │   REST API Layer        │              │                         │               │
│  │   (Gin Framework)       │              │   ┌─────────────────┐   │               │
│  │                         │              │   │ ScheduleClient  │   │               │
│  │   /api/v1/users         │              │   │ - On-call       │   │               │
│  │   /api/v1/auth          │              │   │ - Rotations     │   │               │
│  │   /api/v1/teams         │              │   │ - Overrides     │   │               │
│  └─────────────────────────┘              │   └─────────────────┘   │               │
│            │                              │                         │               │
│            │                              │   ┌─────────────────┐   │               │
│            ▼                              │   │ TicketClient    │   │               │
│  ┌─────────────────────────┐              │   │ - Create ticket │   │               │
│  │   NEW: Alert Handlers   │◄─────────────│   │ - Sync status   │   │               │
│  │                         │              │   └─────────────────┘   │               │
│  │   /api/v1/alerts        │              │                         │               │
│  │   /api/v1/schedules     │              └─────────────────────────┘               │
│  │   /api/v1/oncall        │                        │                               │
│  │   /api/v1/tickets       │                        │ gRPC                          │
│  └─────────────────────────┘                        ▼                               │
│                                           ┌─────────────────────────┐               │
│                                           │   alerting-system       │               │
│                                           │   (Project 1)           │               │
│                                           └─────────────────────────┘               │
│                                                                                      │
│  ┌──────────────────────────────────────────────────────────────────────────────┐   │
│  │                         USER REFERENCE PATTERN                               │   │
│  │                                                                              │   │
│  │   kneutral-api                         alerting-system                       │   │
│  │   ┌──────────────────┐                 ┌──────────────────┐                  │   │
│  │   │ users            │                 │ schedules        │                  │   │
│  │   │ ─────────────────│                 │ ─────────────────│                  │   │
│  │   │ id: uuid         │◄────────────────│ user_id: uuid    │                  │   │
│  │   │ email: string    │   (reference)   │ (references      │                  │   │
│  │   │ name: string     │                 │  kneutral user)  │                  │   │
│  │   │ role: enum       │                 │                  │                  │   │
│  │   └──────────────────┘                 └──────────────────┘                  │   │
│  │                                                                              │   │
│  │   alerting-system NEVER creates users - only stores user_id references      │   │
│  │   User lookup happens via gRPC call to kneutral-api when needed            │   │
│  └──────────────────────────────────────────────────────────────────────────────┘   │
│                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

---

### Project 3: ticket-system (Future)

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              TICKET-SYSTEM                                           │
│                      (Extensible Ticketing Platform)                                 │
├─────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│  ┌──────────────────────────────────────────────────────────────────────────────┐   │
│  │                         gRPC SERVICE INTERFACE                               │   │
│  │                                                                              │   │
│  │   service TicketingService {                                                 │   │
│  │     rpc CreateTicket(CreateTicketRequest) returns (Ticket);                  │   │
│  │     rpc UpdateTicket(UpdateTicketRequest) returns (Ticket);                  │   │
│  │     rpc GetTicket(GetTicketRequest) returns (Ticket);                        │   │
│  │     rpc LinkAlertToTicket(LinkRequest) returns (LinkResponse);               │   │
│  │     rpc SyncStatus(stream ExternalStatus) returns (stream InternalStatus);   │   │
│  │     rpc ListProviders(ListProvidersRequest) returns (ProviderList);          │   │
│  │     rpc TestProvider(TestProviderRequest) returns (TestResult);              │   │
│  │   }                                                                          │   │
│  └──────────────────────────────────────────────────────────────────────────────┘   │
│                                          │                                          │
│                                          ▼                                          │
│  ┌──────────────────────────────────────────────────────────────────────────────┐   │
│  │                         PROVIDER REGISTRY (Plugin Architecture)              │   │
│  │                                                                              │   │
│  │   type TicketProvider interface {                                            │   │
│  │     ID() string                                                              │   │
│  │     Name() string                                                            │   │
│  │     Capabilities() []Capability  // CREATE, UPDATE, SYNC, WEBHOOK           │   │
│  │     ValidateConfig(cfg Config) error                                         │   │
│  │     CreateTicket(ctx, alert, cfg) (*Ticket, error)                           │   │
│  │     UpdateTicket(ctx, ticket, update) error                                  │   │
│  │     SyncStatus(ctx) (<-chan StatusUpdate, error)                             │   │
│  │   }                                                                          │   │
│  │                                                                              │   │
│  │   ┌─────────────────────────────────────────────────────────────────────┐   │   │
│  │   │                      PROVIDER IMPLEMENTATIONS                       │   │   │
│  │   │                                                                     │   │   │
│  │   │   ┌────────────────┐  ┌────────────────┐  ┌────────────────┐       │   │   │
│  │   │   │  SALESFORCE    │  │     JIRA       │  │  SERVICENOW    │       │   │   │
│  │   │   │  Provider      │  │   Provider     │  │   Provider     │       │   │   │
│  │   │   │                │  │                │  │                │       │   │   │
│  │   │   │ - REST API     │  │ - REST API     │  │ - REST API     │       │   │   │
│  │   │   │ - Pub/Sub API  │  │ - Webhooks     │  │ - Webhooks     │       │   │   │
│  │   │   │ - OAuth2       │  │ - OAuth2/API   │  │ - OAuth2       │       │   │   │
│  │   │   │ - Platform     │  │   Token        │  │                │       │   │   │
│  │   │   │   Events       │  │                │  │                │       │   │   │
│  │   │   └────────────────┘  └────────────────┘  └────────────────┘       │   │   │
│  │   │                                                                     │   │   │
│  │   │   ┌────────────────┐  ┌────────────────┐                           │   │   │
│  │   │   │    WEBHOOK     │  │    CUSTOM      │                           │   │   │
│  │   │   │   Provider     │  │   Provider     │                           │   │   │
│  │   │   │                │  │  (Template)    │                           │   │   │
│  │   │   │ - POST/PUT     │  │                │                           │   │   │
│  │   │   │ - Configurable │  │ - Implement    │                           │   │   │
│  │   │   │   payload      │  │   interface    │                           │   │   │
│  │   │   └────────────────┘  └────────────────┘                           │   │   │
│  │   └─────────────────────────────────────────────────────────────────────┘   │   │
│  └──────────────────────────────────────────────────────────────────────────────┘   │
│                                                                                      │
│  ┌──────────────────────────────────────────────────────────────────────────────┐   │
│  │                         FIELD TRANSFORMATION ENGINE                          │   │
│  │                                                                              │   │
│  │   Alert Labels/Annotations ──► Ticket Fields Mapping                         │   │
│  │                                                                              │   │
│  │   Example Salesforce mapping:                                                │   │
│  │   ┌─────────────────────────────────────────────────────────────────────┐   │   │
│  │   │  alert.labels.severity      →  case.Priority                        │   │   │
│  │   │  alert.annotations.summary  →  case.Subject                         │   │   │
│  │   │  alert.annotations.runbook  →  case.Description (append)            │   │   │
│  │   │  alert.labels.team          →  case.OwnerId (lookup)                │   │   │
│  │   │  alert.labels.service       →  case.Service__c (custom field)       │   │   │
│  │   │  alert.fingerprint          →  case.AlertFingerprint__c             │   │   │
│  │   └─────────────────────────────────────────────────────────────────────┘   │   │
│  │                                                                              │   │
│  │   Rule-based transformation using CEL (Common Expression Language):          │   │
│  │   - Conditional field population                                            │   │
│  │   - Value transformations                                                   │   │
│  │   - Default fallbacks                                                       │   │
│  └──────────────────────────────────────────────────────────────────────────────┘   │
│                                                                                      │
│  ┌──────────────────────────────────────────────────────────────────────────────┐   │
│  │                         BIDIRECTIONAL SYNC ENGINE                            │   │
│  │                                                                              │   │
│  │    alerting-system                ticket-system              Salesforce      │   │
│  │                                                                              │   │
│  │    Alert Created ───────────────► CreateTicket() ──────────► Case Created   │   │
│  │         │                              │                          │          │   │
│  │         │                              ▼                          │          │   │
│  │         │                      Store ticket_id,                   │          │   │
│  │         │                      external_id mapping                │          │   │
│  │         │                              │                          │          │   │
│  │         │                              │                          ▼          │   │
│  │         │                              │                   Case Updated      │   │
│  │         │                              │                   (Status: Closed)  │   │
│  │         │                              │                          │          │   │
│  │         │                              ▼                          │          │   │
│  │         │◄──────────────────── SyncStatus() ◄─────────────────────┘          │   │
│  │         │                     (Pub/Sub API)                                  │   │
│  │         ▼                                                                    │   │
│  │    Alert Resolved                                                            │   │
│  │    (auto-sync)                                                               │   │
│  │                                                                              │   │
│  └──────────────────────────────────────────────────────────────────────────────┘   │
│                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

---

## Data Flow Diagrams

### Alert Lifecycle Flow

```
┌────────────────────────────────────────────────────────────────────────────────────┐
│                              ALERT LIFECYCLE FLOW                                   │
└────────────────────────────────────────────────────────────────────────────────────┘

   ┌─────────────┐
   │ Prometheus  │
   │ Alertmanager│
   └──────┬──────┘
          │
          │ POST /api/v1/webhook/alertmanager
          │ {
          │   "status": "firing",
          │   "alerts": [{
          │     "labels": {
          │       "alertname": "HighCPU",
          │       "severity": "critical",
          │       "service": "api-gateway",
          │       "team": "platform"
          │     },
          │     "annotations": {
          │       "summary": "CPU > 90% for 5m",
          │       "runbook": "https://..."
          │     }
          │   }]
          │ }
          ▼
   ┌──────────────────────────────────────────────────────────────────┐
   │                    alerting-system                                │
   │  ┌────────────────────────────────────────────────────────────┐  │
   │  │ 1. PARSE & VALIDATE                                        │  │
   │  │    - Extract labels/annotations                            │  │
   │  │    - Validate integration key                              │  │
   │  │    - Generate fingerprint for dedup                        │  │
   │  └─────────────────────────┬──────────────────────────────────┘  │
   │                            ▼                                     │
   │  ┌────────────────────────────────────────────────────────────┐  │
   │  │ 2. STORE ALERT                                             │  │
   │  │    - Check for existing alert (dedup)                      │  │
   │  │    - Store in PostgreSQL with all labels/annotations       │  │
   │  │    - Status: TRIGGERED                                     │  │
   │  └─────────────────────────┬──────────────────────────────────┘  │
   │                            ▼                                     │
   │  ┌────────────────────────────────────────────────────────────┐  │
   │  │ 3. ESCALATION ENGINE                                       │  │
   │  │    - Look up escalation policy for service                 │  │
   │  │    - Determine on-call user(s) via schedule                │  │
   │  │    - User lookup: gRPC call to kneutral-api                │  │
   │  └─────────────────────────┬──────────────────────────────────┘  │
   │                            ▼                                     │
   │  ┌────────────────────────────────────────────────────────────┐  │
   │  │ 4. NOTIFICATION DISPATCH                                   │  │
   │  │    - Send to Slack/Teams/SMS based on user preferences     │  │
   │  │    - Optionally create ticket (via ticket-system gRPC)     │  │
   │  └────────────────────────────────────────────────────────────┘  │
   └──────────────────────────────────────────────────────────────────┘
          │
          │ User receives notification
          ▼
   ┌─────────────┐
   │    User     │
   │  (On-Call)  │
   └──────┬──────┘
          │
          │ User clicks "Acknowledge" in kneutral-api UI
          ▼
   ┌──────────────────────────────────────────────────────────────────┐
   │                      kneutral-api                                 │
   │  ┌────────────────────────────────────────────────────────────┐  │
   │  │ REST: POST /api/v1/alerts/{id}/acknowledge                 │  │
   │  │ - Validate user session/permissions                        │  │
   │  │ - Call alerting-system via gRPC                            │  │
   │  └─────────────────────────┬──────────────────────────────────┘  │
   └────────────────────────────┼──────────────────────────────────────┘
                                │ gRPC: AcknowledgeAlert
                                ▼
   ┌──────────────────────────────────────────────────────────────────┐
   │                    alerting-system                                │
   │  ┌────────────────────────────────────────────────────────────┐  │
   │  │ 5. UPDATE ALERT STATUS                                     │  │
   │  │    - Status: TRIGGERED → ACKNOWLEDGED                      │  │
   │  │    - Record user_id, timestamp                             │  │
   │  │    - Pause escalation timer                                │  │
   │  └─────────────────────────┬──────────────────────────────────┘  │
   │                            ▼                                     │
   │  ┌────────────────────────────────────────────────────────────┐  │
   │  │ 6. SYNC TO TICKET SYSTEM (if ticket exists)                │  │
   │  │    - gRPC: UpdateTicket(status: "In Progress")             │  │
   │  └────────────────────────────────────────────────────────────┘  │
   └──────────────────────────────────────────────────────────────────┘
          │
          │ Issue resolved
          ▼
   ┌──────────────────────────────────────────────────────────────────┐
   │  7. RESOLVE ALERT                                                │
   │     - Status: ACKNOWLEDGED → RESOLVED                            │
   │     - Sync to ticket-system                                      │
   │     - Ticket closed in Salesforce                                │
   └──────────────────────────────────────────────────────────────────┘
```

---

### User Reference Flow

```
┌────────────────────────────────────────────────────────────────────────────────────┐
│                           USER REFERENCE FLOW                                       │
│                     (alerting-system does NOT store users)                          │
└────────────────────────────────────────────────────────────────────────────────────┘

   ┌─────────────────────────────────────────────────────────────────────────────────┐
   │                           SCHEDULE CREATION                                      │
   │                                                                                  │
   │   kneutral-api                                        alerting-system            │
   │   ┌────────────────────┐                             ┌────────────────────┐     │
   │   │ POST /api/v1/      │                             │                    │     │
   │   │   schedules        │  ─────────────────────────► │ CreateSchedule     │     │
   │   │                    │  gRPC: {                    │ (user_ids only,    │     │
   │   │ Body: {            │    "name": "Platform",      │  no user details)  │     │
   │   │   "name": "...",   │    "users": [               │                    │     │
   │   │   "users": [       │      "uuid-1",              │                    │     │
   │   │     "uuid-1",      │      "uuid-2"               │                    │     │
   │   │     "uuid-2"       │    ]                        │                    │     │
   │   │   ]                │  }                          │                    │     │
   │   │ }                  │                             │                    │     │
   │   └────────────────────┘                             └────────────────────┘     │
   │                                                                                  │
   └─────────────────────────────────────────────────────────────────────────────────┘

   ┌─────────────────────────────────────────────────────────────────────────────────┐
   │                           NOTIFICATION FLOW                                      │
   │                                                                                  │
   │   alerting-system                                     kneutral-api               │
   │   ┌────────────────────┐                             ┌────────────────────┐     │
   │   │ Alert triggered    │                             │                    │     │
   │   │ Need to notify     │  ─────────────────────────► │ GetUsers           │     │
   │   │ on-call user       │  gRPC: GetUsersByIDs({      │                    │     │
   │   │                    │    ids: ["uuid-1"]          │ Returns:           │     │
   │   │ Schedule says:     │  })                         │ {                  │     │
   │   │ user_id: "uuid-1"  │                             │   "users": [{      │     │
   │   │                    │                             │     "id": "...",   │     │
   │   │                    │  ◄───────────────────────── │     "email": "..", │     │
   │   │ Now I know:        │                             │     "slack": "..", │     │
   │   │ - email            │                             │     "phone": ".."  │     │
   │   │ - slack ID         │                             │   }]               │     │
   │   │ - phone            │                             │ }                  │     │
   │   └────────────────────┘                             └────────────────────┘     │
   │                                                                                  │
   └─────────────────────────────────────────────────────────────────────────────────┘

   ┌─────────────────────────────────────────────────────────────────────────────────┐
   │                           USER SERVICE (kneutral-api)                           │
   │                                                                                  │
   │   service UserService {                                                          │
   │     rpc GetUsersByIDs(GetUsersRequest) returns (UsersResponse);                 │
   │     rpc GetUserContactMethods(UserID) returns (ContactMethods);                 │
   │     rpc ValidateUserExists(UserID) returns (ValidationResponse);                │
   │   }                                                                              │
   │                                                                                  │
   │   message ContactMethods {                                                       │
   │     string email = 1;                                                           │
   │     string slack_id = 2;                                                        │
   │     string phone = 3;                                                           │
   │     string teams_id = 4;                                                        │
   │   }                                                                              │
   │                                                                                  │
   └─────────────────────────────────────────────────────────────────────────────────┘
```

---

### Label/Annotation Dynamic Handling

```
┌────────────────────────────────────────────────────────────────────────────────────┐
│                      DYNAMIC LABELS & ANNOTATIONS FLOW                              │
└────────────────────────────────────────────────────────────────────────────────────┘

   INCOMING ALERTMANAGER PAYLOAD:
   ┌─────────────────────────────────────────────────────────────────────────────────┐
   │ {                                                                                │
   │   "labels": {                                                                    │
   │     "alertname": "HighMemory",                                                   │
   │     "severity": "warning",                  ◄── Standard Prometheus labels      │
   │     "namespace": "production",                                                   │
   │     "pod": "api-gateway-7b9f8d6c5-xk2m4",                                       │
   │     "team": "platform",                     ◄── Custom routing labels           │
   │     "ticket_priority": "P2",                                                     │
   │     "sla_tier": "gold"                                                          │
   │   },                                                                             │
   │   "annotations": {                                                               │
   │     "summary": "Memory usage above 80%",    ◄── Standard annotations            │
   │     "description": "Pod memory at 85%",                                         │
   │     "runbook_url": "https://...",                                               │
   │     "dashboard_url": "https://grafana/...", ◄── Custom annotations              │
   │     "escalation_hint": "Check memory leaks in auth service"                     │
   │   }                                                                              │
   │ }                                                                                │
   └─────────────────────────────────────────────────────────────────────────────────┘
                                          │
                                          ▼
   ┌─────────────────────────────────────────────────────────────────────────────────┐
   │                        alerting-system PROCESSING                                │
   │                                                                                  │
   │   ┌──────────────────────────────────────────────────────────────────────────┐  │
   │   │ 1. STORE ALL LABELS/ANNOTATIONS AS-IS                                    │  │
   │   │                                                                          │  │
   │   │    alerts table:                                                         │  │
   │   │    ┌───────────┬─────────────────────────────────────────────────────┐  │  │
   │   │    │ Column    │ Value                                               │  │  │
   │   │    ├───────────┼─────────────────────────────────────────────────────┤  │  │
   │   │    │ id        │ uuid                                                │  │  │
   │   │    │ summary   │ "Memory usage above 80%"                            │  │  │
   │   │    │ status    │ TRIGGERED                                           │  │  │
   │   │    │ labels    │ JSONB: {"alertname": "HighMemory", ...}             │  │  │
   │   │    │ annotations│ JSONB: {"summary": "...", "runbook_url": "..."}    │  │  │
   │   │    └───────────┴─────────────────────────────────────────────────────┘  │  │
   │   └──────────────────────────────────────────────────────────────────────────┘  │
   │                                                                                  │
   │   ┌──────────────────────────────────────────────────────────────────────────┐  │
   │   │ 2. ROUTING RULES (CEL-based)                                             │  │
   │   │                                                                          │  │
   │   │    Rule: "labels.severity == 'critical' && labels.team == 'platform'"    │  │
   │   │    Action: Route to Platform team escalation policy                      │  │
   │   │                                                                          │  │
   │   │    Rule: "has(labels.ticket_priority)"                                   │  │
   │   │    Action: Create ticket with priority = labels.ticket_priority          │  │
   │   └──────────────────────────────────────────────────────────────────────────┘  │
   │                                                                                  │
   │   ┌──────────────────────────────────────────────────────────────────────────┐  │
   │   │ 3. PASS TO TICKET SYSTEM                                                 │  │
   │   │                                                                          │  │
   │   │    gRPC CreateTicket {                                                   │  │
   │   │      alert_id: "...",                                                    │  │
   │   │      labels: { full labels map },                                        │  │
   │   │      annotations: { full annotations map },                              │  │
   │   │      provider_config: {                                                  │  │
   │   │        "type": "salesforce",                                             │  │
   │   │        "priority_field": "labels.ticket_priority",                       │  │
   │   │        "subject_template": "{{annotations.summary}} - {{labels.pod}}"    │  │
   │   │      }                                                                   │  │
   │   │    }                                                                     │  │
   │   └──────────────────────────────────────────────────────────────────────────┘  │
   │                                                                                  │
   └─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Database Schema Overview

### alerting-system Database

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                        alerting-system PostgreSQL Schema                             │
└─────────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────┐       ┌─────────────────────────────┐
│         services            │       │      escalation_policies    │
├─────────────────────────────┤       ├─────────────────────────────┤
│ id: UUID (PK)               │◄──────│ id: UUID (PK)               │
│ name: VARCHAR               │       │ service_id: UUID (FK)       │
│ description: TEXT           │       │ name: VARCHAR               │
│ integration_key: VARCHAR    │       │ repeat_count: INT           │
│ created_at: TIMESTAMP       │       │ created_at: TIMESTAMP       │
└─────────────────────────────┘       └─────────────────────────────┘
              │                                     │
              │                                     │
              ▼                                     ▼
┌─────────────────────────────┐       ┌─────────────────────────────┐
│          alerts             │       │     escalation_steps        │
├─────────────────────────────┤       ├─────────────────────────────┤
│ id: UUID (PK)               │       │ id: UUID (PK)               │
│ service_id: UUID (FK)       │       │ policy_id: UUID (FK)        │
│ summary: TEXT               │       │ step_number: INT            │
│ details: TEXT               │       │ delay_minutes: INT          │
│ status: ENUM                │       │ target_type: ENUM           │
│ source: ENUM                │       │ target_id: UUID             │
│ fingerprint: VARCHAR        │       │   (schedule_id or user_id)  │
│ labels: JSONB        ◄──────┼───────│                             │
│ annotations: JSONB   ◄──────┼───────│                             │
│ created_at: TIMESTAMP       │       └─────────────────────────────┘
│ acknowledged_at: TIMESTAMP  │
│ acknowledged_by: UUID ──────┼───────► (kneutral-api user_id)
│ resolved_at: TIMESTAMP      │
│ resolved_by: UUID    ───────┼───────► (kneutral-api user_id)
│ external_ticket_id: VARCHAR │
└─────────────────────────────┘

┌─────────────────────────────┐       ┌─────────────────────────────┐
│         schedules           │       │        rotations            │
├─────────────────────────────┤       ├─────────────────────────────┤
│ id: UUID (PK)               │◄──────│ id: UUID (PK)               │
│ name: VARCHAR               │       │ schedule_id: UUID (FK)      │
│ description: TEXT           │       │ name: VARCHAR               │
│ timezone: VARCHAR           │       │ rotation_type: ENUM         │
│ created_at: TIMESTAMP       │       │ shift_length: INTERVAL      │
└─────────────────────────────┘       │ start_time: TIMESTAMP       │
              │                       │ handoff_time: TIME          │
              │                       └─────────────────────────────┘
              ▼                                     │
┌─────────────────────────────┐                    │
│     schedule_overrides      │                    ▼
├─────────────────────────────┤       ┌─────────────────────────────┐
│ id: UUID (PK)               │       │     rotation_members        │
│ schedule_id: UUID (FK)      │       ├─────────────────────────────┤
│ user_id: UUID ──────────────┼───────│ rotation_id: UUID (FK)      │
│ start_time: TIMESTAMP       │       │ user_id: UUID ──────────────┼───► (kneutral user)
│ end_time: TIMESTAMP         │       │ position: INT               │
│ override_type: ENUM         │       └─────────────────────────────┘
└─────────────────────────────┘

┌─────────────────────────────┐       ┌─────────────────────────────┐
│       alert_logs            │       │      routing_rules          │
├─────────────────────────────┤       ├─────────────────────────────┤
│ id: UUID (PK)               │       │ id: UUID (PK)               │
│ alert_id: UUID (FK)         │       │ service_id: UUID (FK)       │
│ event_type: ENUM            │       │ name: VARCHAR               │
│ user_id: UUID               │       │ condition_expr: TEXT (CEL)  │
│ message: TEXT               │       │ action_type: ENUM           │
│ created_at: TIMESTAMP       │       │ action_config: JSONB        │
└─────────────────────────────┘       │ priority: INT               │
                                      └─────────────────────────────┘

NOTE: NO users table - all user_id fields reference kneutral-api users via gRPC
```

---

## Notification Service Architecture (Detailed)

### gRPC Service Definitions

```protobuf
// proto/notification/v1/notification.proto

syntax = "proto3";
package notification.v1;

import "google/protobuf/struct.proto";
import "google/protobuf/timestamp.proto";

// ============================================================================
// NOTIFICATION SERVICE
// ============================================================================

service NotificationService {
  // Send a notification using a template
  rpc SendNotification(SendNotificationRequest) returns (SendNotificationResponse);

  // Get delivery status
  rpc GetDeliveryStatus(GetDeliveryStatusRequest) returns (DeliveryStatus);

  // Stream delivery status updates
  rpc StreamDeliveryStatus(StreamDeliveryStatusRequest) returns (stream DeliveryStatus);

  // Register a new destination (user contact method)
  rpc RegisterDestination(RegisterDestinationRequest) returns (Destination);
}

message SendNotificationRequest {
  string request_id = 1;                    // Idempotency key
  string template_id = 2;                   // Template to use
  int32 template_version = 3;               // Specific version (0 = latest)
  google.protobuf.Struct render_context = 4; // Dynamic data (labels, annotations, alert)
  repeated Destination destinations = 5;    // Where to send
  NotificationPriority priority = 6;        // Affects rate limiting
}

message Destination {
  string user_id = 1;                       // kneutral-api user reference
  ChannelType channel = 2;                  // slack, email, sms, teams, etc.
  string channel_address = 3;               // email address, phone number, slack ID
}

enum ChannelType {
  CHANNEL_UNSPECIFIED = 0;
  CHANNEL_SLACK = 1;
  CHANNEL_TEAMS = 2;
  CHANNEL_EMAIL = 3;
  CHANNEL_SMS = 4;
  CHANNEL_VOICE = 5;
  CHANNEL_PUSH = 6;
  CHANNEL_WEBHOOK = 7;
}

enum NotificationPriority {
  PRIORITY_UNSPECIFIED = 0;
  PRIORITY_LOW = 1;       // Batch, may be delayed
  PRIORITY_NORMAL = 2;    // Standard delivery
  PRIORITY_HIGH = 3;      // Skip batching, immediate
  PRIORITY_CRITICAL = 4;  // Bypass rate limits
}

// ============================================================================
// TEMPLATE SERVICE
// ============================================================================

service TemplateService {
  // CRUD
  rpc CreateTemplate(CreateTemplateRequest) returns (Template);
  rpc GetTemplate(GetTemplateRequest) returns (Template);
  rpc UpdateTemplate(UpdateTemplateRequest) returns (Template);
  rpc DeleteTemplate(DeleteTemplateRequest) returns (DeleteTemplateResponse);
  rpc ListTemplates(ListTemplatesRequest) returns (stream Template);

  // Versioning
  rpc GetTemplateVersion(GetTemplateVersionRequest) returns (Template);
  rpc ListTemplateVersions(ListTemplateVersionsRequest) returns (stream TemplateVersion);

  // Preview & Validation (CRITICAL for WYSIWYG)
  rpc RenderPreview(RenderPreviewRequest) returns (RenderPreviewResponse);
  rpc ValidateTemplate(ValidateTemplateRequest) returns (ValidationResult);
}

message Template {
  string id = 1;
  string name = 2;
  string description = 3;
  int32 current_version = 4;
  map<string, ChannelTemplate> channels = 5;  // Per-channel content
  repeated string required_variables = 6;      // Variables that MUST be present
  repeated string optional_variables = 7;      // Variables with defaults
  google.protobuf.Timestamp created_at = 8;
  google.protobuf.Timestamp updated_at = 9;
  string created_by_user_id = 10;
}

message ChannelTemplate {
  ChannelType channel = 1;
  string content = 2;           // Template content (Go template syntax)
  string format = 3;            // "text", "html", "slack_blocks", "teams_adaptive"
  map<string, string> metadata = 4;  // Channel-specific settings
}

message RenderPreviewRequest {
  string template_id = 1;
  int32 template_version = 2;           // 0 = draft/latest
  google.protobuf.Struct sample_data = 3;  // Sample render context
  repeated ChannelType channels = 4;       // Which channels to preview
}

message RenderPreviewResponse {
  map<string, RenderedPreview> previews = 1;  // channel -> rendered content
  ValidationResult validation = 2;
}

message RenderedPreview {
  ChannelType channel = 1;
  string rendered_content = 2;     // Final output (HTML, JSON, text)
  string preview_url = 3;          // Optional: URL to visual preview
  repeated string warnings = 4;     // Non-blocking issues
}

message ValidationResult {
  bool valid = 1;
  repeated ValidationError errors = 2;
  repeated ValidationWarning warnings = 3;
}

message ValidationError {
  string field = 1;
  string message = 2;
  string code = 3;  // "MISSING_VARIABLE", "INVALID_SYNTAX", "LENGTH_EXCEEDED"
}
```

---

## Template System Details

### Template Variable System

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                           RENDER CONTEXT CONTRACT                                    │
├─────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│  The render context passed to templates follows this structure:                      │
│                                                                                      │
│  {                                                                                   │
│    "alert": {                                                                        │
│      "id": "abc-123",                                                               │
│      "summary": "High CPU on api-gateway",                                          │
│      "details": "CPU usage exceeded 90% for 5 minutes",                             │
│      "status": "TRIGGERED",                                                         │
│      "source": "prometheus",                                                        │
│      "created_at": "2026-02-03T10:30:00Z"                                           │
│    },                                                                                │
│    "labels": {                        ◄── Dynamic from Prometheus                   │
│      "alertname": "HighCPU",                                                        │
│      "severity": "critical",                                                        │
│      "namespace": "production",                                                     │
│      "pod": "api-gateway-7b9f8d6c5-xk2m4",                                         │
│      "team": "platform",                                                            │
│      "cluster": "us-east-1"                                                         │
│    },                                                                                │
│    "annotations": {                   ◄── Dynamic from Prometheus                   │
│      "summary": "CPU > 90% for 5m",                                                 │
│      "description": "Pod memory at 85%",                                            │
│      "runbook_url": "https://runbooks.example.com/high-cpu",                        │
│      "dashboard_url": "https://grafana.example.com/d/abc123"                        │
│    },                                                                                │
│    "user": {                          ◄── From kneutral-api                         │
│      "id": "user-uuid",                                                             │
│      "name": "John Doe",                                                            │
│      "email": "john@example.com"                                                    │
│    },                                                                                │
│    "escalation": {                                                                  │
│      "step": 1,                                                                     │
│      "policy_name": "Platform On-Call"                                              │
│    }                                                                                 │
│  }                                                                                   │
│                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

### Template Syntax Examples

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              SLACK BLOCK KIT TEMPLATE                                │
├─────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│  {                                                                                   │
│    "blocks": [                                                                       │
│      {                                                                               │
│        "type": "header",                                                            │
│        "text": {                                                                     │
│          "type": "plain_text",                                                       │
│          "text": "🚨 {{ .alert.summary }}"                                          │
│        }                                                                             │
│      },                                                                              │
│      {                                                                               │
│        "type": "section",                                                            │
│        "fields": [                                                                   │
│          {                                                                           │
│            "type": "mrkdwn",                                                         │
│            "text": "*Severity:*\n{{ .labels.severity | default \"unknown\" }}"      │
│          },                                                                          │
│          {                                                                           │
│            "type": "mrkdwn",                                                         │
│            "text": "*Namespace:*\n{{ .labels.namespace | default \"N/A\" }}"        │
│          }                                                                           │
│        ]                                                                             │
│      },                                                                              │
│      {{ if .annotations.runbook_url }}                                              │
│      {                                                                               │
│        "type": "actions",                                                            │
│        "elements": [                                                                 │
│          {                                                                           │
│            "type": "button",                                                         │
│            "text": { "type": "plain_text", "text": "📖 Runbook" },                  │
│            "url": "{{ .annotations.runbook_url }}"                                  │
│          }                                                                           │
│        ]                                                                             │
│      }                                                                               │
│      {{ end }}                                                                       │
│    ]                                                                                 │
│  }                                                                                   │
│                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              EMAIL HTML TEMPLATE                                     │
├─────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│  Subject: [{{ .labels.severity | upper }}] {{ .alert.summary }}                     │
│                                                                                      │
│  <html>                                                                              │
│  <body style="font-family: sans-serif;">                                            │
│    <div style="background: {{ if eq .labels.severity "critical" }}#dc3545           │
│                            {{ else if eq .labels.severity "warning" }}#ffc107       │
│                            {{ else }}#17a2b8{{ end }};                              │
│         color: white; padding: 20px;">                                              │
│      <h1>{{ .alert.summary }}</h1>                                                   │
│    </div>                                                                            │
│    <div style="padding: 20px;">                                                      │
│      <p><strong>Details:</strong> {{ .alert.details }}</p>                          │
│      <p><strong>Pod:</strong> {{ .labels.pod | default "N/A" }}</p>                 │
│      <p><strong>Namespace:</strong> {{ .labels.namespace }}</p>                     │
│      {{ if .annotations.runbook_url }}                                              │
│      <p><a href="{{ .annotations.runbook_url }}">View Runbook</a></p>               │
│      {{ end }}                                                                       │
│    </div>                                                                            │
│  </body>                                                                             │
│  </html>                                                                             │
│                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              SMS PLAIN TEXT TEMPLATE                                 │
├─────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│  [{{ .labels.severity | upper }}] {{ .alert.summary }}                              │
│  Pod: {{ .labels.pod | truncate 20 }}                                               │
│  {{ if .annotations.runbook_url }}Runbook: {{ .annotations.runbook_url }}{{ end }}  │
│                                                                                      │
│  (Max 160 chars - validation enforced)                                              │
│                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

### Live Preview Flow

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              LIVE PREVIEW ARCHITECTURE                               │
└─────────────────────────────────────────────────────────────────────────────────────┘

   User in kneutral-api UI
   ┌────────────────────────────────────────────────────────────────────────────────┐
   │  Template Editor                                                                │
   │  ┌──────────────────────────────────┬──────────────────────────────────────┐   │
   │  │  EDIT PANEL                      │  PREVIEW PANEL                       │   │
   │  │                                  │                                      │   │
   │  │  Template Name: [Critical Alert] │  ┌────────────────────────────────┐  │   │
   │  │                                  │  │  SLACK PREVIEW                 │  │   │
   │  │  Channel: [Slack ▼]              │  │  ┌────────────────────────────┐│  │   │
   │  │                                  │  │  │ 🚨 High CPU on api-gateway ││  │   │
   │  │  ┌────────────────────────────┐  │  │  │                            ││  │   │
   │  │  │ {                          │  │  │  │ Severity: critical         ││  │   │
   │  │  │   "blocks": [              │  │  │  │ Namespace: production      ││  │   │
   │  │  │     {                      │  │  │  │                            ││  │   │
   │  │  │       "type": "header",    │  │  │  │ [📖 Runbook]               ││  │   │
   │  │  │       "text": {            │  │  │  └────────────────────────────┘│  │   │
   │  │  │         "text": "🚨 {{     │  │  └────────────────────────────────┘  │   │
   │  │  │           .alert.summary   │  │                                      │   │
   │  │  │         }}"                │  │  Sample Data:                        │   │
   │  │  │       }                    │  │  ┌────────────────────────────────┐  │   │
   │  │  │     }                      │  │  │ { "alert": {...},             │  │   │
   │  │  │   ]                        │  │  │   "labels": {...} }           │  │   │
   │  │  │ }                          │  │  └────────────────────────────────┘  │   │
   │  │  └────────────────────────────┘  │                                      │   │
   │  │                                  │  [Use Real Alert ▼] [Refresh]        │   │
   │  │  [Save Draft] [Publish v2]       │                                      │   │
   │  └──────────────────────────────────┴──────────────────────────────────────┘   │
   └────────────────────────────────────────────────────────────────────────────────┘
                │                                          │
                │ Edit template                            │ On each edit (debounced)
                ▼                                          ▼
   ┌────────────────────────────────────────────────────────────────────────────────┐
   │  kneutral-api REST                                                              │
   │  POST /api/v1/templates/{id}/preview                                           │
   │  Body: { "sample_data": {...}, "channels": ["slack", "email"] }                │
   └────────────────────────────────────────────────────────────────────────────────┘
                │
                │ gRPC: RenderPreview
                ▼
   ┌────────────────────────────────────────────────────────────────────────────────┐
   │  Notification Service (or Template Manager if separate)                        │
   │                                                                                 │
   │  1. Load template content                                                       │
   │  2. Parse Go template                                                           │
   │  3. Execute with sample_data                                                    │
   │  4. Format for each channel (Slack Block Kit, HTML, etc.)                      │
   │  5. Validate output (length limits, required fields)                           │
   │  6. Return rendered previews + validation results                              │
   └────────────────────────────────────────────────────────────────────────────────┘
                │
                │ Returns
                ▼
   ┌────────────────────────────────────────────────────────────────────────────────┐
   │  {                                                                              │
   │    "previews": {                                                                │
   │      "slack": {                                                                 │
   │        "rendered_content": "{\"blocks\": [...]}",                              │
   │        "preview_url": null                                                      │
   │      },                                                                         │
   │      "email": {                                                                 │
   │        "rendered_content": "<html>...</html>",                                 │
   │        "preview_url": null                                                      │
   │      }                                                                          │
   │    },                                                                           │
   │    "validation": {                                                              │
   │      "valid": true,                                                             │
   │      "warnings": ["SMS exceeds 160 chars, will be truncated"]                  │
   │    }                                                                            │
   │  }                                                                              │
   └────────────────────────────────────────────────────────────────────────────────┘
```

---

## Technology Stack

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              TECHNOLOGY STACK                                        │
└─────────────────────────────────────────────────────────────────────────────────────┘

┌────────────────────────┐  ┌────────────────────────┐  ┌────────────────────────┐
│    alerting-system     │  │     kneutral-api       │  │    ticket-system       │
│      (Project 1)       │  │      (Project 2)       │  │     (Project 3)        │
├────────────────────────┤  ├────────────────────────┤  ├────────────────────────┤
│                        │  │                        │  │                        │
│  Language: Go          │  │  Language: Go          │  │  Language: Go          │
│                        │  │                        │  │                        │
│  API:                  │  │  API:                  │  │  API:                  │
│  - gRPC (primary)      │  │  - REST (existing)     │  │  - gRPC (primary)      │
│  - HTTP (webhooks)     │  │  - gRPC client         │  │                        │
│                        │  │                        │  │                        │
│  Database:             │  │  Database:             │  │  Database:             │
│  - PostgreSQL          │  │  - PostgreSQL          │  │  - PostgreSQL          │
│  - JSONB for labels    │  │  - (existing)          │  │                        │
│                        │  │                        │  │                        │
│  Queue:                │  │  Cache:                │  │  External:             │
│  - River (pg-backed)   │  │  - Redis (sessions)    │  │  - Salesforce API      │
│                        │  │                        │  │  - Jira API            │
│  Notifications:        │  │  Auth:                 │  │  - ServiceNow API      │
│  - Slack SDK           │  │  - OIDC/SAML           │  │                        │
│  - MS Graph            │  │  - JWT                 │  │  Sync:                 │
│  - Twilio              │  │  - Casbin RBAC         │  │  - Pub/Sub API         │
│  - SMTP                │  │                        │  │  - Webhooks            │
│                        │  │                        │  │                        │
└────────────────────────┘  └────────────────────────┘  └────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              SHARED INFRASTRUCTURE                                   │
├─────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                      │
│  ┌─────────────────────┐  ┌─────────────────────┐  ┌─────────────────────┐         │
│  │   Observability     │  │    Service Mesh     │  │    Deployment       │         │
│  ├─────────────────────┤  ├─────────────────────┤  ├─────────────────────┤         │
│  │ - OpenTelemetry     │  │ - Istio/Envoy       │  │ - Kubernetes        │         │
│  │ - Prometheus        │  │ - mTLS              │  │ - Helm charts       │         │
│  │ - Grafana           │  │ - Load balancing    │  │ - ArgoCD            │         │
│  │ - Loki              │  │                     │  │                     │         │
│  └─────────────────────┘  └─────────────────────┘  └─────────────────────┘         │
│                                                                                      │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

---

## gRPC Service Definitions

```protobuf
// proto/alerting/v1/alerting.proto

syntax = "proto3";
package alerting.v1;

import "google/protobuf/timestamp.proto";

// ============================================================================
// ALERT SERVICE
// ============================================================================

service AlertService {
  // Alert CRUD
  rpc CreateAlert(CreateAlertRequest) returns (Alert);
  rpc GetAlert(GetAlertRequest) returns (Alert);
  rpc ListAlerts(ListAlertsRequest) returns (stream Alert);
  rpc UpdateAlertStatus(UpdateStatusRequest) returns (Alert);

  // Alert Actions
  rpc AcknowledgeAlert(AcknowledgeRequest) returns (AcknowledgeResponse);
  rpc ResolveAlert(ResolveRequest) returns (ResolveResponse);
  rpc EscalateAlert(EscalateRequest) returns (EscalateResponse);
  rpc SnoozeAlert(SnoozeRequest) returns (SnoozeResponse);
}

message Alert {
  string id = 1;
  string service_id = 2;
  string summary = 3;
  string details = 4;
  AlertStatus status = 5;
  AlertSource source = 6;
  string fingerprint = 7;
  map<string, string> labels = 8;       // Dynamic labels from Alertmanager
  map<string, string> annotations = 9;   // Dynamic annotations
  google.protobuf.Timestamp created_at = 10;
  google.protobuf.Timestamp acknowledged_at = 11;
  string acknowledged_by_user_id = 12;   // Reference to kneutral-api user
  google.protobuf.Timestamp resolved_at = 13;
  string resolved_by_user_id = 14;
  string external_ticket_id = 15;        // Link to ticket-system
}

enum AlertStatus {
  ALERT_STATUS_UNSPECIFIED = 0;
  ALERT_STATUS_TRIGGERED = 1;
  ALERT_STATUS_ACKNOWLEDGED = 2;
  ALERT_STATUS_RESOLVED = 3;
}

// ============================================================================
// SCHEDULE SERVICE
// ============================================================================

service ScheduleService {
  rpc CreateSchedule(CreateScheduleRequest) returns (Schedule);
  rpc GetSchedule(GetScheduleRequest) returns (Schedule);
  rpc UpdateSchedule(UpdateScheduleRequest) returns (Schedule);
  rpc DeleteSchedule(DeleteScheduleRequest) returns (DeleteScheduleResponse);

  // On-Call Operations
  rpc GetOnCallUsers(OnCallRequest) returns (OnCallResponse);
  rpc CreateOverride(CreateOverrideRequest) returns (Override);

  // Rotations
  rpc AddRotation(AddRotationRequest) returns (Rotation);
  rpc UpdateRotation(UpdateRotationRequest) returns (Rotation);
}

message Schedule {
  string id = 1;
  string name = 2;
  string description = 3;
  string timezone = 4;
  repeated Rotation rotations = 5;
  repeated Override active_overrides = 6;
}

message OnCallResponse {
  repeated string user_ids = 1;  // kneutral-api user IDs
  string schedule_id = 2;
  google.protobuf.Timestamp valid_until = 3;
}

// ============================================================================
// TICKET INTEGRATION SERVICE
// ============================================================================

service TicketIntegrationService {
  rpc CreateTicketFromAlert(CreateTicketRequest) returns (CreateTicketResponse);
  rpc GetTicketStatus(GetTicketStatusRequest) returns (TicketStatus);
  rpc SyncTicketStatus(stream TicketStatusUpdate) returns (stream SyncAckResponse);
  rpc LinkAlertToTicket(LinkRequest) returns (LinkResponse);
}

message CreateTicketRequest {
  string alert_id = 1;
  string provider_type = 2;              // "salesforce", "jira", etc.
  map<string, string> provider_config = 3;
  map<string, string> field_mappings = 4; // Alert label -> Ticket field
}

message TicketStatusUpdate {
  string ticket_id = 1;
  string external_ticket_id = 2;
  string status = 3;
  map<string, string> metadata = 4;
  google.protobuf.Timestamp updated_at = 5;
}
```

---

## Implementation Phases

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                           IMPLEMENTATION ROADMAP                                     │
└─────────────────────────────────────────────────────────────────────────────────────┘

PHASE 1: FOUNDATION (Weeks 1-3)
═══════════════════════════════════════════════════════════════════════════════════════

┌─────────────────────────────────────────────────────────────────────────────────────┐
│  alerting-system Core                                                                │
│  ─────────────────────                                                              │
│  □ Project scaffolding (Go modules, Makefile)                                       │
│  □ PostgreSQL schema and migrations                                                 │
│  □ gRPC service definitions (protobuf)                                              │
│  □ Alert Store (CRUD with JSONB labels/annotations)                                 │
│  □ Basic webhook receiver (Alertmanager format)                                     │
│  □ Deduplication logic (fingerprint-based)                                          │
└─────────────────────────────────────────────────────────────────────────────────────┘

PHASE 2: CORE ENGINE (Weeks 4-6)
═══════════════════════════════════════════════════════════════════════════════════════

┌─────────────────────────────────────────────────────────────────────────────────────┐
│  Alert Engine                                                                        │
│  ────────────                                                                        │
│  □ Escalation policy engine                                                         │
│  □ Schedule management (rotations, overrides)                                       │
│  □ On-call calculation                                                              │
│  □ gRPC client for kneutral-api user lookup                                         │
│  □ Notification dispatch (Slack, Email)                                             │
└─────────────────────────────────────────────────────────────────────────────────────┘

PHASE 3: KNEUTRAL-API INTEGRATION (Weeks 7-8)
═══════════════════════════════════════════════════════════════════════════════════════

┌─────────────────────────────────────────────────────────────────────────────────────┐
│  kneutral-api Additions                                                              │
│  ─────────────────────                                                              │
│  □ gRPC client for alerting-system                                                  │
│  □ New REST endpoints: /api/v1/alerts, /api/v1/schedules, /api/v1/oncall           │
│  □ Alert dashboard UI integration                                                   │
│  □ User contact methods management                                                  │
│  □ gRPC server for user lookup service                                              │
└─────────────────────────────────────────────────────────────────────────────────────┘

PHASE 4: ADVANCED FEATURES (Weeks 9-11)
═══════════════════════════════════════════════════════════════════════════════════════

┌─────────────────────────────────────────────────────────────────────────────────────┐
│  Advanced Alerting                                                                   │
│  ─────────────────                                                                   │
│  □ CEL-based routing rules                                                          │
│  □ Dynamic label/annotation routing                                                 │
│  □ Additional notification channels (MS Teams, Twilio)                              │
│  □ Heartbeat monitoring                                                             │
│  □ Alert bundling and throttling                                                    │
└─────────────────────────────────────────────────────────────────────────────────────┘

PHASE 5: TICKET SYSTEM FOUNDATION (Weeks 12-14)
═══════════════════════════════════════════════════════════════════════════════════════

┌─────────────────────────────────────────────────────────────────────────────────────┐
│  ticket-system (Project 3)                                                          │
│  ─────────────────────────                                                          │
│  □ Project scaffolding                                                              │
│  □ Provider registry (plugin architecture)                                          │
│  □ gRPC service implementation                                                      │
│  □ Webhook provider (generic HTTP POST)                                             │
│  □ Integration with alerting-system                                                 │
└─────────────────────────────────────────────────────────────────────────────────────┘

PHASE 6: SALESFORCE INTEGRATION (Weeks 15-18)
═══════════════════════════════════════════════════════════════════════════════════════

┌─────────────────────────────────────────────────────────────────────────────────────┐
│  Salesforce Provider                                                                 │
│  ──────────────────                                                                 │
│  □ OAuth2 authentication flow                                                       │
│  □ Case creation via REST API                                                       │
│  □ Pub/Sub API integration for status sync                                          │
│  □ Field transformation engine                                                      │
│  □ Bidirectional status sync                                                        │
└─────────────────────────────────────────────────────────────────────────────────────┘
```

---

## Key Decisions & Rationale

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Inter-service communication | gRPC | Efficient binary protocol, strong typing, streaming support |
| User storage | kneutral-api only | Single source of truth, avoids sync complexity |
| Label/annotation storage | JSONB in PostgreSQL | Flexible schema, queryable, no migrations needed |
| Routing rules | CEL (Common Expression Language) | Auditable, no code deployment for rule changes |
| Ticketing architecture | Plugin registry | Extensible without core changes |
| Salesforce sync | Pub/Sub API | Real-time, official API, gRPC-based |

---

## Files to Create/Modify

### alerting-system (New Project)
```
alerting-system/
├── cmd/
│   └── server/main.go
├── internal/
│   ├── alert/           # Alert store and engine
│   ├── escalation/      # Escalation policy engine
│   ├── schedule/        # Schedule and on-call management
│   ├── notification/    # Notification dispatch
│   ├── webhook/         # HTTP webhook handlers
│   └── grpc/            # gRPC service implementations
├── proto/
│   └── alerting/v1/     # Protocol buffer definitions
├── migrations/          # SQL migrations
├── Makefile
└── go.mod
```

### kneutral-api (Modifications)
```
kneutral-api/
├── pkg/
│   └── alerting/        # NEW: gRPC client for alerting-system
├── handler/
│   ├── alert_handler.go # NEW: REST handlers for alerts
│   └── schedule_handler.go # NEW: REST handlers for schedules
├── router/v1/
│   ├── alerts.go        # NEW: Alert routes
│   └── schedules.go     # NEW: Schedule routes
└── service/
    └── user_service.go  # MODIFY: Add gRPC server for user lookup
```

---

## Verification Plan

1. **Unit Tests**: Each component tested in isolation
2. **Integration Tests**: gRPC service communication
3. **E2E Tests**: Full alert flow from webhook to notification
4. **Load Tests**: Alertmanager burst simulation
5. **Chaos Tests**: Service failure and recovery
