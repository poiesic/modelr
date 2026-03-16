---
name: modelr-outage-report
description: Generate plausible prose outage summaries from modelr check findings
---

# modelr: Outage Report Generation

## When to use this skill

Use this skill when the user wants to:
- Generate outage reports from model check findings
- Understand what would happen in production if a constraint violation went unaddressed
- Use pre-mortem analysis to surface risks in a system model

## How it works

Read the `.checked.yaml` file for a model (running `check_model` first if needed). For each finding, generate a plausible incident report as if the failure has already happened.

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

## Guidelines

- Write as if the incident actually happened — past tense, concrete details
- Use the actual numbers from the finding (connection counts, ops/sec, instance counts)
- Reference the model's known unknowns when they're relevant to the failure — these are deferred decisions that contributed to the gap
- Make the timeline feel real: include clock times, realistic team responses, and the fog of an active incident
- Scale severity to the finding: a 33x throughput gap is a hard outage, a 1.1x margin is a performance degradation
- Keep each report concise — aim for a report that takes 2 minutes to read
- If the model has multiple findings, generate a separate report for each, but note interactions between them if cascading is plausible
