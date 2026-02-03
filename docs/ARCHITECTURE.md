# OnCall System - Big Picture Architecture

> A modern, gRPC-native alerting and ticketing platform replacing GoAlert with simplified architecture, template management, and extensible integrations.

## System Overview

```mermaid
flowchart TB
    subgraph External["External Sources"]
        PM[Prometheus/Alertmanager]
        GR[Grafana]
        WH[Generic Webhooks]
        MN[Manual Triggers]
    end

    subgraph P1["Project 1: alerting-system"]
        WR[Webhook Receiver]
        AE[Alert Engine]
        EM[Escalation Manager]
        SM[Schedule Manager]
        RR[CEL Routing Rules]
        DB1[(PostgreSQL)]

        WR --> AE
        AE --> EM
        EM --> SM
        AE --> RR
        AE --> DB1
        EM --> DB1
        SM --> DB1
    end

    subgraph P2["Project 2: kneutral-api (Existing)"]
        UM[User Management]
        AU[Authentication]
        UI[Alert Dashboard]
        TE[Template Editor UI]
        REST[REST API]
        DB2[(PostgreSQL)]

        UM --> DB2
        UI --> REST
        TE --> REST
    end

    subgraph P4["Project 4: notification-service"]
        NS[Notification Service]
        TS[Template Service]
        RE[Rendering Engine]
        DE[Delivery Engine]
        DB4[(PostgreSQL)]

        subgraph Channels["Channel Providers"]
            SL[Slack]
            TM[MS Teams]
            EM2[Email]
            SMS[SMS/Voice]
            PH[Push]
            WHK[Webhooks]
        end

        TS --> RE
        NS --> RE
        NS --> DE
        DE --> Channels
        TS --> DB4
        NS --> DB4
    end

    subgraph P3["Project 3: ticket-system (Future)"]
        PR[Provider Registry]
        SF[Salesforce]
        JR[Jira]
        SN[ServiceNow]
        SE[Sync Engine]
        DB3[(PostgreSQL)]

        PR --> SF
        PR --> JR
        PR --> SN
        SE --> DB3
    end

    PM --> WR
    GR --> WR
    WH --> WR
    MN --> WR

    RR -->|"gRPC: SendNotification"| NS
    EM -->|"gRPC: GetUsers"| UM
    NS -->|"gRPC: GetUser"| UM
    REST -->|"gRPC: RenderPreview"| TS
    NS -->|"gRPC: CreateTicket"| PR

    classDef external fill:#e1f5fe,stroke:#01579b
    classDef project1 fill:#fff3e0,stroke:#e65100
    classDef project2 fill:#e8f5e9,stroke:#1b5e20
    classDef project3 fill:#fce4ec,stroke:#880e4f
    classDef project4 fill:#f3e5f5,stroke:#4a148c

    class PM,GR,WH,MN external
    class WR,AE,EM,SM,RR,DB1 project1
    class UM,AU,UI,TE,REST,DB2 project2
    class PR,SF,JR,SN,SE,DB3 project3
    class NS,TS,RE,DE,DB4,SL,TM,EM2,SMS,PH,WHK project4
```

---

## Service Communication

```mermaid
sequenceDiagram
    autonumber
    participant AM as Alertmanager
    participant AS as alerting-system
    participant KA as kneutral-api
    participant NS as notification-service
    participant SL as Slack

    AM->>AS: POST /webhook/alertmanager<br/>(labels, annotations)
    AS->>AS: Deduplicate & Store Alert
    AS->>AS: CEL: Evaluate routing rules
    AS->>KA: gRPC: GetOnCallUsers(schedule_id)
    KA-->>AS: [user_id_1, user_id_2]
    AS->>NS: gRPC: SendNotification<br/>(template_id, render_context, destinations)
    NS->>NS: Load template, render with context
    NS->>KA: gRPC: GetUserContactMethods(user_id)
    KA-->>NS: {slack_id, email, phone}
    NS->>SL: Send Slack Block Kit message
    SL-->>NS: Delivery confirmation
    NS-->>AS: Delivery status
```

---

## Project Responsibilities

```mermaid
mindmap
  root((OnCall System))
    alerting-system
      Webhook Ingestion
        Alertmanager
        Grafana
        Generic JSON
      Alert Management
        Deduplication
        Status Tracking
        JSONB Labels
      Escalation
        Policies
        Steps & Delays
        On-Call Calculation
      Routing
        CEL Rules
        Template Selection
    kneutral-api
      User Management
        OIDC/SAML Auth
        RBAC
        Contact Methods
      UI Layer
        Alert Dashboard
        Template Editor
        Schedule Management
      API Gateway
        REST Endpoints
        gRPC Clients
    notification-service
      Template Management
        CRUD
        Versioning
        Preview/WYSIWYG
      Rendering
        Go Templates
        Channel Formatters
        Validation
      Delivery
        Rate Limiting
        Retries
        Status Tracking
      Channels
        Slack
        Teams
        Email
        SMS/Voice
    ticket-system
      Provider Registry
        Plugin Interface
        Salesforce
        Jira
        ServiceNow
      Sync Engine
        Bidirectional
        Field Mapping
        CEL Transform
```

---

## Data Flow: Alert Lifecycle

```mermaid
stateDiagram-v2
    [*] --> Received: Webhook arrives
    Received --> Deduplicated: Check fingerprint
    Deduplicated --> Stored: New alert
    Deduplicated --> Updated: Existing alert
    Stored --> Triggered: Initial state
    Updated --> Triggered: Still firing
    Triggered --> Routing: CEL rules evaluation
    Routing --> Notifying: Template + destinations selected
    Notifying --> Acknowledged: User acks
    Notifying --> Escalated: Timeout
    Escalated --> Notifying: Next step
    Acknowledged --> Resolved: User resolves
    Resolved --> [*]: Closed

    note right of Routing
        CEL evaluates:
        - labels.severity
        - labels.team
        - annotations.*
    end note

    note right of Notifying
        Sends to:
        - Slack
        - Email
        - SMS
        - Creates ticket
    end note
```

---

## Template System Flow

```mermaid
flowchart LR
    subgraph UI["kneutral-api UI"]
        TE[Template Editor]
        PV[Preview Panel]
        SD[Sample Data]
    end

    subgraph NS["notification-service"]
        TS[TemplateService]
        RE[Rendering Engine]
        CF[Channel Formatters]
        VL[Validator]
    end

    subgraph Output["Rendered Outputs"]
        SK[Slack Blocks JSON]
        TM[Teams Adaptive Card]
        EM[Email HTML]
        SM[SMS Text]
    end

    TE -->|"Edit template"| TS
    SD -->|"Sample alert data"| TS
    TS --> RE
    RE --> CF
    CF --> SK
    CF --> TM
    CF --> EM
    CF --> SM
    SK --> PV
    TM --> PV
    EM --> PV
    SM --> PV
    VL -->|"Warnings/Errors"| PV
```

---

## User Reference Pattern

```mermaid
erDiagram
    KNEUTRAL_API_USERS ||--o{ ALERTING_SCHEDULES : "referenced by"
    KNEUTRAL_API_USERS ||--o{ ALERTING_ALERTS : "acknowledged by"
    KNEUTRAL_API_USERS ||--o{ NOTIFICATION_DESTINATIONS : "receives"

    KNEUTRAL_API_USERS {
        uuid id PK
        string email
        string name
        json contact_methods
        string role
    }

    ALERTING_SCHEDULES {
        uuid id PK
        uuid user_id FK "refs kneutral user"
        string schedule_name
        int position
    }

    ALERTING_ALERTS {
        uuid id PK
        uuid acknowledged_by FK "refs kneutral user"
        uuid resolved_by FK "refs kneutral user"
        jsonb labels
        jsonb annotations
    }

    NOTIFICATION_DESTINATIONS {
        uuid id PK
        uuid user_id FK "refs kneutral user"
        string channel_type
        string channel_address
    }
```

> **Key Principle:** alerting-system and notification-service **never store user data**. They only store `user_id` references and call kneutral-api via gRPC for user details.

---

## Technology Stack

```mermaid
flowchart TB
    subgraph Languages["Languages & Frameworks"]
        GO[Go 1.22+]
        GRPC[gRPC + Protobuf]
        GIN[Gin REST Framework]
    end

    subgraph Data["Data Layer"]
        PG[(PostgreSQL)]
        RD[(Redis)]
        JSONB[JSONB for Labels]
    end

    subgraph Infra["Infrastructure"]
        K8S[Kubernetes]
        HELM[Helm Charts]
        ARGO[ArgoCD]
    end

    subgraph Observability["Observability"]
        OTEL[OpenTelemetry]
        PROM[Prometheus]
        GRAF[Grafana]
        LOKI[Loki]
    end

    subgraph Integrations["External Integrations"]
        SLACK[Slack SDK]
        MSGRAPH[MS Graph API]
        TWILIO[Twilio SMS/Voice]
        SMTP[SMTP Email]
        SF[Salesforce API]
    end
```

---

## Ticket System Plugin Architecture

```mermaid
classDiagram
    class TicketProvider {
        <<interface>>
        +ID() string
        +Name() string
        +Capabilities() []Capability
        +CreateTicket(ctx, alert, cfg) Ticket
        +UpdateTicket(ctx, ticket, update) error
        +SyncStatus(ctx) chan StatusUpdate
    }

    class SalesforceProvider {
        -clientID string
        -instanceURL string
        -pubsubClient PubSubClient
        +CreateTicket() Case
        +SyncStatus() chan StatusUpdate
    }

    class JiraProvider {
        -apiToken string
        -baseURL string
        +CreateTicket() Issue
        +SyncStatus() chan StatusUpdate
    }

    class ServiceNowProvider {
        -instanceURL string
        -credentials Credentials
        +CreateTicket() Incident
        +SyncStatus() chan StatusUpdate
    }

    class WebhookProvider {
        -webhookURL string
        -headers map
        +CreateTicket() WebhookResponse
    }

    class ProviderRegistry {
        -providers map~string,TicketProvider~
        +Register(provider TicketProvider)
        +Get(id string) TicketProvider
        +List() []ProviderInfo
    }

    TicketProvider <|.. SalesforceProvider
    TicketProvider <|.. JiraProvider
    TicketProvider <|.. ServiceNowProvider
    TicketProvider <|.. WebhookProvider
    ProviderRegistry o-- TicketProvider
```

---

## Implementation Phases

```mermaid
gantt
    title OnCall System Implementation Roadmap
    dateFormat  YYYY-MM-DD

    section Phase 1: Foundation
    alerting-system scaffolding    :p1a, 2026-02-10, 1w
    PostgreSQL schema              :p1b, after p1a, 1w
    gRPC service definitions       :p1c, after p1a, 1w
    Webhook receiver               :p1d, after p1b, 1w

    section Phase 2: Alert Engine
    Escalation policy engine       :p2a, after p1d, 2w
    Schedule management            :p2b, after p1d, 2w
    On-call calculation            :p2c, after p2a, 1w

    section Phase 3: Notification
    notification-service scaffold  :p3a, after p2c, 1w
    Template CRUD & storage        :p3b, after p3a, 1w
    Rendering engine               :p3c, after p3b, 2w
    Delivery engine                :p3d, after p3c, 2w

    section Phase 4: Integration
    kneutral-api gRPC clients      :p4a, after p3d, 1w
    REST endpoints                 :p4b, after p4a, 1w
    Template Editor UI             :p4c, after p4b, 2w

    section Phase 5: Channels
    Slack & Email providers        :p5a, after p3d, 2w
    Teams & SMS providers          :p5b, after p5a, 2w

    section Phase 6: Ticketing
    ticket-system foundation       :p6a, after p4c, 2w
    Salesforce integration         :p6b, after p6a, 3w
```

---

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| **Services Count** | 4 services | Template Manager inside Notification Service for WYSIWYG guarantee |
| **Inter-service Comm** | gRPC | Efficient binary protocol, strong typing, streaming |
| **User Storage** | kneutral-api only | Single source of truth, avoids sync complexity |
| **Labels/Annotations** | JSONB in PostgreSQL | Flexible schema, queryable, no migrations for new labels |
| **Routing Rules** | CEL (Common Expression Language) | Auditable, no code deployment for rule changes |
| **Template Rendering** | Server-side | Security (prevent injection), consistency (WYSIWYG) |
| **Ticketing** | Plugin registry pattern | Extensible without core changes |

---

## AI Consensus (Gemini 3 Pro Preview + GPT 5.2)

### Agreement Points
- Notification Service must be separate from alerting-system
- Routing rules ("which template") stay in alerting-system
- Server-side rendering mandatory
- Use `google.protobuf.Struct` for dynamic Prometheus labels
- Preview API must use same renderer as production

### Final Decision
**Option B: 4 Services** (GPT 5.2 recommendation)
- Template Manager as module inside Notification Service
- WYSIWYG guarantee (same code path for preview and send)
- Simpler operations, no cross-service render drift
- Pattern: PagerDuty, Opsgenie

---

## Repository Structure

```
kneutral-org/oncall-system/
├── docs/
│   ├── ARCHITECTURE.md          # This document
│   ├── API.md                   # API specifications
│   └── DEPLOYMENT.md            # Deployment guide
├── proto/
│   ├── alerting/v1/             # Alert service protos
│   ├── notification/v1/         # Notification service protos
│   └── ticketing/v1/            # Ticket service protos
├── alerting-system/             # Project 1
├── notification-service/        # Project 4
├── ticket-system/               # Project 3 (future)
└── README.md
```

---

## Next Steps

1. **Create GitHub repo:** `kneutral-org/oncall-system`
2. **Initialize monorepo structure**
3. **Define gRPC proto files**
4. **Begin Phase 1 implementation**

---

*Document generated with AI assistance from Gemini 3 Pro Preview and GPT 5.2 via PAL MCP consensus workflow.*
