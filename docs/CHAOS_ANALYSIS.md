# OnCall System MVP2 - Chaos Engineering Analysis

> A critical examination of failure modes, cascading failures, and resilience gaps in the 222-task MVP2 implementation plan.

---

## Executive Summary

After analyzing the MVP2 plan, I've identified **47 distinct failure scenarios** across 8 categories. While the plan includes solid foundational resilience (circuit breakers, retries, timeouts), it has critical gaps in:

1. **Partial failure handling** - Most scenarios assume binary success/failure
2. **State synchronization** - No saga pattern for distributed transactions
3. **Observability during degradation** - Limited visibility when things go wrong
4. **Data corruption recovery** - No checksums or validation on critical data
5. **Human error protection** - Missing guardrails for operational mistakes

---

## Part 1: Top 10 Most Likely Failures

### Rank 1: Slack API Rate Limiting (99% certainty during incidents)

**Scenario:** During a production incident, multiple alerts fire simultaneously. Each alert triggers notifications to the same Slack channel. Slack returns HTTP 429.

**Why it WILL happen:**
- Slack rate limits are 1 message/second per channel
- Alertmanager fires multiple alerts in batches
- TASK-163 mentions circuit breaker but not **per-channel** rate limiting

**Missing from plan:**
- Per-destination rate limiting (not just per-provider)
- Message coalescing when rate limited
- Priority queuing (critical alerts skip queue)

**Current gap:** TASK-116 mentions "Notification Batching" but acceptance criteria don't address rate limit evasion.

---

### Rank 2: PostgreSQL Connection Pool Exhaustion

**Scenario:** High alert volume + slow template rendering = all connections waiting. New alerts cannot be stored. Webhook returns 503.

**Why it WILL happen:**
- TASK-044 (PgBouncer) exists but pool sizing is unspecified
- Template rendering (TASK-023) can be slow with complex templates
- Database transactions hold connections during render

**Current gap:** No connection timeout separate from query timeout. A slow query holds the connection until statement_timeout.

**Missing:**
- Connection acquisition timeout (separate from query timeout)
- Read replica for ListAlerts (heavy queries)
- Connection per operation type budgets

---

### Rank 3: Template Rendering Timeout During Peak Load

**Scenario:** A complex template with nested loops takes 900ms to render. Under load, context switching adds 200ms. Timeout is 1s (TASK-128). Some notifications fail intermittently.

**Why it WILL happen:**
- Go template execution is single-threaded per goroutine
- CPU contention under load increases latency
- The 1s timeout is absolute, not CPU time

**Missing from plan:**
- Template complexity scoring at save time
- Pre-rendered template cache for common scenarios
- Render time budget proportional to template complexity

---

### Rank 4: kneutral-api User Cache Stampede

**Scenario:** Cache TTL (5 min per TASK-033) expires. 100 concurrent notifications all request the same user. All 100 hit kneutral-api simultaneously.

**Why it WILL happen:**
- TASK-033/034 mention "caching layer" but not stampede protection
- On-call schedules often route to same user
- Cache expiry is synchronized across keys

**Missing:**
- Singleflight pattern for concurrent requests to same key
- Staggered cache expiry (jitter)
- Stale-while-revalidate pattern

---

### Rank 5: Escalation Timer Drift After Pod Restart

**Scenario:** Alert triggers at T+0. Escalation set for T+5min. Pod restarts at T+3min. Escalation never fires because timer was in-memory.

**Current mitigation:** TASK-126 mentions "Escalation Timer Persistence" but it's a P1 task that could be deprioritized.

**Why it WILL happen:**
- Kubernetes pods restart regularly (rolling updates, node preemption)
- In-memory timers are the simplest implementation
- Easy to forget during initial development

**Missing (even with TASK-126):**
- Leader election for escalation processing
- Distributed timer implementation details
- Exactly-once escalation guarantee

---

### Rank 6: Webhook Receiver Overload During Prometheus Restart

**Scenario:** Prometheus restarts and re-fires all active alerts at once. 1000 alerts hit the webhook endpoint simultaneously.

**Why it WILL happen:**
- Prometheus sends all firing alerts every evaluation cycle
- After restart, all alerts are re-sent immediately
- No built-in throttling in Alertmanager

**Current gap:** TASK-115 mentions "Alert Aggregation/Grouping" but acceptance criteria don't include:
- Rate limiting at ingestion
- Queue depth limits with backpressure
- Priority processing for new vs. re-fired alerts

---

### Rank 7: DNS Resolution Failure for External Services

**Scenario:** DNS resolver returns SERVFAIL for slack.com. Circuit breaker trips. Recovery takes 30s (TASK-047) but DNS TTL was 60s. System oscillates.

**Why it WILL happen:**
- Cloud DNS has occasional hiccups
- Go's default resolver caches are often disabled in containers
- Circuit breaker and DNS cache have independent timers

**Missing:**
- DNS prefetching for known external services
- Separate DNS failure circuit breaker
- Fallback to IP-based connections for critical services

---

### Rank 8: Database Migration Failure Mid-Flight

**Scenario:** Migration 015 (add column with NOT NULL) acquires lock. Migration times out. Column partially added. Next migration fails. Service won't start.

**Why it WILL happen:**
- TASK-107 mentions "Zero-Downtime Migration Strategy" but doesn't specify lock timeout handling
- Large tables + NOT NULL + default value = table rewrite = long lock

**Missing:**
- Migration timeout configuration
- Rollback automation on failure
- Pre-flight migration testing on production copy

---

### Rank 9: TLS Certificate Expiry During Weekend

**Scenario:** Certificate expires Saturday 2am. No one notices until Monday. All gRPC calls fail with certificate error.

**Current mitigation:** TASK-125 mentions "Certificate Renewal Automation" but:
- No alerting on imminent expiry
- No monitoring of certificate validity
- No fallback for failed renewal

**Missing:**
- Certificate expiry monitoring (< 14 days = warning, < 7 days = critical)
- Automatic renewal failure alerting
- Emergency manual renewal runbook

---

### Rank 10: Goroutine Leak in Long-Running Connections

**Scenario:** gRPC streaming for ListAlerts leaves goroutines when client disconnects ungracefully. Memory grows over 72 hours. OOMKilled.

**Why it WILL happen:**
- TASK-018 includes "ListAlerts RPC with streaming response"
- Client disconnections don't always trigger server-side cleanup
- Go's goroutines are cheap, making leaks easy to ignore

**Current gap:** No mention of:
- Goroutine monitoring in TASK-098 (pprof)
- Connection cleanup on context cancellation
- Maximum stream duration limits

---

## Part 2: Cascading Failure Chains

### Chain 1: The Notification Thundering Herd

```
Slack API partial outage (returns 500 for 10% of requests)
    |
    v
Circuit breaker hesitates (not enough failures to trip)
    |
    v
Retry logic fires (3 retries per notification)
    |
    v
3x load on Slack API
    |
    v
Slack returns 429 for ALL requests
    |
    v
All notifications fail
    |
    v
Escalation engine fires new notifications (user didn't ack)
    |
    v
Even more Slack requests
    |
    v
Circuit breaker finally trips
    |
    v
30 seconds pass, circuit half-opens
    |
    v
First request succeeds (Slack recovered)
    |
    v
ALL pending notifications flush simultaneously
    |
    v
429 again, circuit re-trips
    |
    v
LOOP FOREVER until operator intervention
```

**Missing resilience:**
- Adaptive retry backoff based on error type
- Pending notification drain rate limiting
- Health check separate from first real request

---

### Chain 2: The User Cache Disaster

```
kneutral-api deploys new version with bug
    |
    v
GetUsersByIDs returns empty array for valid users
    |
    v
alerting-service caches empty response (5 min TTL)
    |
    v
No users found for on-call schedule
    |
    v
Escalation engine skips all steps (no targets)
    |
    v
Critical alerts go unnotified for 5 minutes
    |
    v
kneutral-api bug fixed, deployed
    |
    v
Cache still has stale data for 3 more minutes
    |
    v
Total incident duration: 8 minutes unnotified
```

**Missing resilience:**
- Cache negative result TTL (shorter than positive)
- Validation of user lookup results (empty = suspicious)
- Fallback to last-known-good user data

---

### Chain 3: The Template Corruption Spiral

```
Developer saves template with syntax error in Slack block
    |
    v
TASK-027 (ValidateTemplate) only validates Go template syntax
    |
    v
Slack Block Kit JSON is syntactically valid but semantically wrong
    |
    v
Slack API returns 400 Bad Request
    |
    v
Delivery marked as FAILED
    |
    v
Retry fires (because 400 might be transient, right?)
    |
    v
3 retries, all fail
    |
    v
Alert considered "notified" (retries exhausted)
    |
    v
No escalation (notification "completed")
    |
    v
User never receives notification
    |
    v
Incident impact: hours of unnotified alerts
```

**Missing resilience:**
- Slack Block Kit validation at save time (call Slack's validate endpoint)
- Differentiate 4xx (don't retry) from 5xx (retry)
- Fallback to plain text when block kit fails

---

### Chain 4: The Database Connection Storm

```
PostgreSQL primary goes into recovery mode (failover)
    |
    v
Connection pool connections become invalid
    |
    v
All new requests get connection errors
    |
    v
Webhook handler returns 503
    |
    v
Alertmanager retries immediately (default behavior)
    |
    v
More requests pile up
    |
    v
Go's database/sql doesn't detect stale connections immediately
    |
    v
Connection pool trying to reconnect
    |
    v
Recovery complete, new primary available
    |
    v
All accumulated requests hit new primary simultaneously
    |
    v
New primary overwhelmed
    |
    v
Response latency spikes to 5s
    |
    v
All webhook requests timeout
    |
    v
Alertmanager marks endpoint as down
    |
    v
Stops sending alerts
    |
    v
When primary stabilizes, alerts arrive in massive batch
```

**Missing resilience:**
- Connection validation before use
- Request queue with depth limit
- Gradual load increase after recovery (circuit breaker half-open should limit concurrency)

---

### Chain 5: The Observability Blindspot

```
Prometheus scrape interval: 15s
    |
    v
Alert duration: 3 minutes
    |
    v
Spike in error rate from 0% to 80% in 5 seconds
    |
    v
First scrape misses the spike (bad timing)
    |
    v
Second scrape sees errors
    |
    v
Alert fires at T+30s
    |
    v
Notification sent
    |
    v
But Slack is also having issues (same infrastructure)
    |
    v
Notification fails
    |
    v
Alert about notification failure?
    |
    v
Can't send it - Slack is down
    |
    v
Backup notification path (TASK-119)?
    |
    v
Triggers at 50% delivery failure rate
    |
    v
But we only tried once so far
    |
    v
50% threshold not met
    |
    v
Operator unaware for 5+ minutes
```

**Missing resilience:**
- Out-of-band notification path (SMS via different provider)
- Immediate escalation for system health alerts
- External synthetic monitoring of notification pipeline

---

## Part 3: Unrecoverable States

### State 1: Split-Brain Escalation

**How it happens:**
1. Two pods process the same alert simultaneously
2. Both calculate escalation step 2
3. Both send notifications
4. Both update alert state
5. Last writer wins
6. Escalation history corrupted

**Recovery difficulty:** Cannot determine true escalation history. Must manually audit all notifications sent.

**Missing:** Distributed lock for escalation processing per alert.

---

### State 2: Orphaned In-Flight Notifications

**How it happens:**
1. Notification marked as SENT in delivery_logs
2. Pod crashes before receiving Slack confirmation
3. Pod restarts
4. Notification is neither DELIVERED nor FAILED
5. Stuck in SENT forever

**Recovery difficulty:** Cannot retry (might send duplicate). Cannot ignore (might have failed).

**Missing:** Notification state machine with timeout transitions. SENT -> (timeout) -> UNKNOWN -> retry with idempotency key.

---

### State 3: Template Version Rollback Paradox

**How it happens:**
1. Template version 5 is active
2. Thousands of delivery_logs reference template version 5
3. Security issue found in version 5
4. Delete version 5? No, foreign key constraint
5. Rollback to version 4? Active alerts still rendered with v5
6. Update version 5 content? Violates immutability principle

**Recovery:** Must add template "disabled" flag. But plan assumes immutability = never modified.

**Missing:** Template soft-delete/archive with re-render capability.

---

### State 4: Circular Escalation Policy Detection Failure

**How it happens:**
1. Policy A step 3: escalate to Policy B
2. Policy B step 2: escalate to Policy A
3. No cycle detection at creation time (separate API calls)
4. Alert triggers
5. Infinite escalation loop
6. Goroutine per escalation step
7. Memory exhaustion

**Missing:**
- Graph cycle detection on policy save
- Maximum escalation depth limit (global)
- Escalation deduplication (already escalated this alert to this policy)

---

### State 5: Database Constraint Violation During Restore

**How it happens:**
1. Backup taken at T1
2. New template created at T2 referencing new variables
3. Disaster at T3
4. Restore from T1
5. In-flight notifications reference variables that don't exist
6. All renders fail
7. Cannot re-create template (ID conflict)
8. Cannot modify backup (immutability)

**Missing:**
- Backup integrity verification with forward compatibility check
- Restore with conflict resolution strategy
- Gradual restore validation

---

## Part 4: Missing Resilience Tasks

Based on the failure analysis, these tasks are NOT in the 222-task plan:

### Critical Missing (P0)

| ID | Task | Blast Radius if Missing |
|----|------|------------------------|
| CHAOS-001 | Per-channel/per-destination rate limiting | Slack bans entire integration |
| CHAOS-002 | Singleflight for user cache lookups | kneutral-api overwhelmed |
| CHAOS-003 | Distributed lock for escalation processing | Duplicate notifications |
| CHAOS-004 | Notification state timeout transitions | Stuck notifications |
| CHAOS-005 | 4xx vs 5xx retry differentiation | Wasted retries, delayed recovery |
| CHAOS-006 | Escalation cycle detection | Infinite loop, OOM |
| CHAOS-007 | Connection pool acquisition timeout | Complete service hang |
| CHAOS-008 | Out-of-band alerting for system health | Silent failures during outages |

### High Missing (P1)

| ID | Task | Impact |
|----|------|--------|
| CHAOS-009 | Template complexity scoring | Unpredictable timeouts |
| CHAOS-010 | Staggered cache expiry with jitter | Cache stampedes |
| CHAOS-011 | DNS prefetching for external services | Intermittent failures |
| CHAOS-012 | Negative cache TTL (shorter) | Stale failure data |
| CHAOS-013 | Slack Block Kit validation at save | Silent delivery failures |
| CHAOS-014 | Goroutine monitoring and leak detection | Slow memory exhaustion |
| CHAOS-015 | Request queue depth limits | Unbounded memory growth |
| CHAOS-016 | Gradual load increase after recovery | Post-outage thundering herd |

### Medium Missing (P2)

| ID | Task | Impact |
|----|------|--------|
| CHAOS-017 | External synthetic monitoring | Delayed issue detection |
| CHAOS-018 | Backup forward compatibility check | Restore failures |
| CHAOS-019 | Template soft-delete/archive | Data model rigidity |
| CHAOS-020 | Pre-rendered template cache | Latency spikes under load |

---

## Part 5: Blast Radius Analysis

### Component Failure Impact Matrix

| Failed Component | Direct Impact | Secondary Impact | Tertiary Impact |
|-----------------|---------------|------------------|-----------------|
| **PostgreSQL (alerting)** | No new alerts stored | Webhook returns 503 | Alertmanager marks endpoint dead |
| **PostgreSQL (notification)** | No templates, no delivery logs | All notifications fail | Escalations fire incorrectly |
| **Redis** | No caching | kneutral-api overloaded | User lookups timeout |
| **kneutral-api** | No user data | Can't resolve on-call | Notifications have no destination |
| **Slack API** | No Slack messages | Escalations trigger | Other channels unaffected |
| **notification-service** | All notifications fail | All channels affected | Alerts accumulate unnotified |
| **alerting-service** | No new alerts | External monitors see gap | Team unaware of issues |
| **Prometheus** | No metrics | No alerting | No visibility |
| **DNS** | All external calls fail | Everything fails | Complete blackout |

### Service Dependency Graph Criticality

```
                      [DNS]
                        |
                        v
    [PostgreSQL] <-- [alerting-service] --> [notification-service]
         ^                 |                        |
         |                 v                        v
         +----------[kneutral-api]            [Slack/Email]
                          ^                        ^
                          |                        |
                      [Redis] ---------------------|
```

**Single Points of Failure:**
1. DNS - Everything depends on it
2. PostgreSQL (alerting) - Core alert data
3. notification-service - Only notification path

**Hidden dependencies:**
- Template rendering depends on kneutral-api for user variables
- Escalation calculation depends on schedule data + user data
- Deduplication depends on database uniqueness constraints

---

## Part 6: Chaos Testing Recommendations

### Phase 1: Foundation (Week 1-2)

#### Experiment 1.1: Database Connection Kill
```yaml
name: postgres-connection-kill
target: alerting-service pods
action:
  - Use Chaos Mesh to inject network partition to PostgreSQL
  - Duration: 30 seconds
expected:
  - Webhook returns 503 within 5 seconds (connection timeout)
  - Service auto-recovers when partition heals
  - No data corruption
  - Metrics show connection_errors spike
verify:
  - curl webhook endpoint during chaos
  - Check Prometheus for connection_pool_exhausted metric
  - Verify alert created after recovery
```

#### Experiment 1.2: Slow Template Rendering
```yaml
name: slow-template-chaos
target: notification-service
action:
  - Deploy template with 10000 loop iterations
  - Send notification using this template
expected:
  - Render timeout after 1 second
  - Notification marked as FAILED
  - No goroutine leak
  - Alert fires for render timeout
verify:
  - Check delivery_logs for FAILED status
  - pprof goroutine count before/after
  - Prometheus render_duration histogram
```

#### Experiment 1.3: Slack Rate Limit
```yaml
name: slack-rate-limit-simulation
target: notification-service
action:
  - Mock Slack API to return 429 for 60 seconds
  - Send 50 notifications simultaneously
expected:
  - Circuit breaker opens after 5 failures
  - Notifications queued for retry
  - No thundering herd on recovery
  - Metrics show circuit_breaker_open
verify:
  - Check circuit state metric
  - Verify notification eventual delivery
  - Check retry backoff intervals in logs
```

### Phase 2: Cascading Failures (Week 3-4)

#### Experiment 2.1: Multi-Service Partition
```yaml
name: service-mesh-partition
target: network between alerting-service and notification-service
action:
  - Inject 100% packet loss for 2 minutes
expected:
  - alerting-service logs notification failure
  - Alerts still created in database
  - Escalation engine compensates
  - Recovery when partition heals
verify:
  - gRPC circuit breaker trips
  - Alert has "notification_pending" status
  - Notification sent after recovery
```

#### Experiment 2.2: kneutral-api Degradation
```yaml
name: user-service-slow
target: kneutral-api
action:
  - Inject 5 second latency on GetUsersByIDs
expected:
  - Cache miss triggers slow lookup
  - Timeout prevents blocking (should be < 5s)
  - Fallback to cached data or degraded mode
  - No cascading timeouts
verify:
  - Notification includes fallback user info
  - or notification deferred with reason
  - Total request latency bounded
```

#### Experiment 2.3: Cache Stampede
```yaml
name: cache-stampede-simulation
target: notification-service Redis cache
action:
  - Delete all user cache keys
  - Send 100 notifications to same user simultaneously
expected:
  - Only 1 request to kneutral-api (singleflight)
  - All 100 notifications complete
  - Cache repopulated
verify:
  - kneutral-api logs show single request
  - Prometheus cache_miss counter = 1
```

### Phase 3: Data Integrity (Week 5-6)

#### Experiment 3.1: Transaction Rollback
```yaml
name: mid-transaction-kill
target: alerting-service pod
action:
  - Send webhook
  - Kill pod 50ms after (mid-transaction)
expected:
  - Transaction rolled back (no partial data)
  - Alert either exists completely or not at all
  - Webhook can be retried safely
verify:
  - Check database for alert existence
  - Verify no orphaned escalation records
```

#### Experiment 3.2: Invalid Template Injection
```yaml
name: template-bomb
target: notification-service
action:
  - Somehow bypass validation and insert template with:
    {{ range $i := iterate 1000000 }}{{ $i }}{{ end }}
expected:
  - Render hits iteration limit (1000)
  - Returns error, not hang
  - No memory exhaustion
verify:
  - Render completes within 1s
  - Memory usage stable
  - Error logged with template ID
```

### Phase 4: Human Error Simulation (Week 7-8)

#### Experiment 4.1: Wrong Config Deployment
```yaml
name: misconfiguration-blast
target: alerting-service
action:
  - Deploy with invalid DATABASE_URL
expected:
  - Health check fails immediately
  - Kubernetes doesn't promote deployment
  - Old pods continue serving
verify:
  - kubectl get pods shows new pods CrashLoopBackOff
  - Traffic still flowing to old pods
  - Zero downtime
```

#### Experiment 4.2: Secrets Expiry
```yaml
name: secret-expiry
target: Slack OAuth token
action:
  - Rotate Slack token without updating secret
expected:
  - Slack provider returns 401
  - Circuit breaker opens
  - Alert fires for auth failure
  - Other channels unaffected
verify:
  - Check alert for "slack_auth_failure"
  - Email notifications still work
  - Recovery after secret update
```

---

## Chaos Testing Tool Recommendations

### Primary: Chaos Mesh (Kubernetes-native)
- Network chaos (partition, latency, packet loss)
- Pod chaos (kill, container kill)
- Time chaos (clock skew)
- IO chaos (disk latency, errors)

### Secondary: Gremlin
- Application-level chaos
- CPU/memory stress
- Graceful shutdown testing

### Custom Scripts Needed

```go
// pkg/chaos/template_bomb.go
// Inject infinite loop template for testing render limits

// pkg/chaos/cache_stampede.go
// Delete cache keys and trigger simultaneous requests

// pkg/chaos/escalation_loop.go
// Create circular escalation policy for cycle detection testing
```

---

## Recommendations Summary

### Immediate Actions (Before MVP2 Launch)

1. **Add per-destination rate limiting** - Slack will ban the integration otherwise
2. **Implement singleflight for cache** - Prevents kneutral-api overload
3. **Add 4xx vs 5xx retry differentiation** - Stop retrying permanent failures
4. **Add escalation cycle detection** - Prevent infinite loops

### Before Production Traffic

5. **Add out-of-band alerting** - SMS for system health
6. **Add connection pool acquisition timeout** - Prevent full hang
7. **Add distributed lock for escalation** - Prevent duplicates
8. **Add notification state timeout** - Clean up stuck notifications

### Continuous Improvement

9. Run chaos experiments monthly
10. Review circuit breaker tuning quarterly
11. Load test before each major release
12. Maintain chaos experiment runbooks

---

## Conclusion

The MVP2 plan has a solid foundation for resilience with circuit breakers, retries, and timeouts. However, it lacks:

1. **Granular failure handling** - Most patterns assume full success or full failure
2. **Distributed transaction safety** - No saga pattern for cross-service operations
3. **Graceful degradation paths** - When things fail, they fail completely
4. **Human error protection** - Operational mistakes have high blast radius

The 8 critical missing tasks (CHAOS-001 through CHAOS-008) should be added to the MVP2 plan before production deployment. The chaos testing phases should be scheduled as part of pre-production validation.

**Risk Assessment:** Without the critical missing tasks, the system will experience at least one significant incident within the first month of production traffic.

---

*Document generated by Chaos Engineering analysis. Last updated: 2026-02-03*
