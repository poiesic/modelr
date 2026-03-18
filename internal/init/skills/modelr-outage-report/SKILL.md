---
name: modelr-outage-report
description: Generate plausible prose outage summaries from modelr check and verify findings
---

# modelr: Outage Report Generation

## When to use this skill

Use this skill when the user wants to:
- Generate outage reports from model check findings
- Understand what would happen in production if a constraint violation went unaddressed
- Use pre-mortem analysis to surface risks in a system model

## How it works

Read the `.checked.yaml` file for a model (running `check_model` first if needed). Also read the `.verified.yaml` file if it exists (running `verify_model` if needed). Combine arithmetic check findings with behavioral verification results to generate plausible incident reports.

- **Check findings** (from `.checked.yaml`) reveal steady-state capacity violations — situations where the math doesn't work even under ideal conditions
- **Verification results** (from `.verified.yaml`) reveal timing-dependent failures — cold-start races, connection pool contention, and resource exhaustion under concurrent load

Together, these produce richer, more realistic outage narratives.

## Generating the report

For each finding in the checked file, write a narrative outage summary with these sections:

### Format

```
## Incident: [short title]

**Severity**: [P1/P2/P3 based on finding severity and blast radius]
**Duration**: [plausible duration based on the failure mode]
**Affected components**: [from the finding's upstream/downstream]

### Timeline

Write a plausible minute-by-minute timeline of the incident, starting from the trigger condition through detection, escalation, and resolution. Include:
- A realistic trigger (traffic spike, deploy, upstream dependency change)
- The specific constraint violation from the finding, with the actual numbers
- Cascading effects on other components in the model
- How the failure would manifest to end users
- Detection (or lack thereof) and response

### Root cause

Explain the underlying constraint violation in plain language, referencing the specific values from the finding. Connect it to the model's known unknowns if any are relevant — an unresolved tradeoff or deferred decision that contributed to the failure.

### What would have caught this

Describe what constraint, property value, or design decision in the model would have prevented this incident.
```

### Incorporating behavioral verification

When `.verified.yaml` exists and contains rejections for the same upstream → downstream pair as a check finding, enrich the report:

- **In the timeline**: Use the minimal failure case from verification to ground the event sequence. Instead of inventing an interleaving, use the actual steps from the shrinker output. Reference the specific instances, rule names (e.g., `RequestArrives`, `StartGrowth`, `GrowComplete`), and state transitions.

- **In the root cause**: Distinguish between the steady-state arithmetic violation and the dynamic failure mode. A capacity check might show a 2x oversubscription, but the verification might reveal that failures happen even earlier — during cold-start when pools are empty and connection establishment creates a window of vulnerability.

- **As separate incidents**: If verification rejected a relationship that had no arithmetic finding (the steady-state math passes but the concurrency dynamics fail), generate an additional incident report for the behavioral failure. Title it to reflect the dynamic nature: "Cold-start connection storm", "Pool growth race under traffic spike", etc.

### Example: behavioral enrichment

When the verification output shows:
```yaml
- upstream: api-server
  downstream: postgres
  pattern: finite_pooled_resource
  result: reject
  simulations: 23
  failures: 4
  minimal_failure:
    - rule: RequestArrives
      instance: 0
      state: {pending: 1, pool: 0}
    - rule: StartGrowth
      instance: 0
      state: {in_flight: 1, used_connections: 1}
    ...
  violated_invariant: Conservation
```

The timeline should reference this directly:
> At 02:14, the deployment completed and all 50 api-server instances started simultaneously with empty connection pools. Instance 0 received its first request and began establishing a connection to postgres (20ms establishment time). Before that connection was ready, instances 1-49 also began pool growth. Within 200ms, 50 concurrent connection establishments exhausted postgres's 50-connection limit, leaving no headroom for the 3 pre-existing connections from the background worker...

## Guidelines

- Write as if the incident actually happened — past tense, concrete details
- Use the actual numbers from the finding (connection counts, ops/sec, instance counts)
- Reference the model's known unknowns when they're relevant to the failure — these are deferred decisions that contributed to the gap
- Make the timeline feel real: include clock times, realistic team responses, and the fog of an active incident
- Scale severity to the finding: a 33x throughput gap is a hard outage, a 1.1x margin is a performance degradation
- Keep each report concise — aim for a report that takes 2 minutes to read
- If the model has multiple findings, generate a separate report for each, but note interactions between them if cascading is plausible
- When both arithmetic and behavioral findings exist for the same pair, combine them into a single report rather than duplicating — the behavioral detail strengthens the narrative
- Behavioral-only failures (verification rejects but arithmetic passes) deserve their own reports — these are the subtle, timing-dependent outages that are hardest to anticipate
