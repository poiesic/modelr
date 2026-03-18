---
name: modelr-verify
description: Run behavioral verification on system models and interpret results
---

# modelr: Behavioral Verification

## When to use this skill

Use this skill when the user wants to:
- Run behavioral verification on a system model
- Understand whether a model has timing-dependent failure modes
- Investigate concurrency issues like cold-start races or connection pool contention
- Interpret verification results (pass/reject, minimal failure cases)

## How it works

Behavioral verification uses property-based simulation with SPRT (Sequential Probability Ratio Test) to find timing-dependent failures that arithmetic checks cannot catch. It simulates concurrent actors sharing downstream resources under randomized interleavings.

Arithmetic checks (via `check_model`) verify steady-state capacity math. Behavioral verification catches what arithmetic misses: cold-start races, pool growth contention, and resource exhaustion under realistic concurrent load.

## Running verification

Call the `verify_model` MCP tool with the absolute path to the `.model.yaml` file. The tool runs simulations and writes a `.verified.yaml` file alongside the model.

If the model has not been checked yet, run `check_model` first — arithmetic violations are faster to find and should be fixed before running behavioral verification.

## Interpreting results

### Pass

```
Accepted api-server → postgres after 312 simulations (0 failures)
Confidence: 99% that failure rate < 0.1%
```

The system was simulated under random concurrent conditions and no invariant violations were found. SPRT provides a statistical guarantee: the true failure rate is below the target with the stated confidence.

### Reject

```
Rejected api-server → postgres after 23 simulations (4 failures)
Estimated failure rate: 17.4%
Minimal failure case:
  1. RequestArrives(instance=3)  → pending=1, pool=0
  2. StartGrowth(instance=3)    → in_flight=1, used_connections=1
  ...
  Property violated: Conservation (used_connections=51 > downstream_capacity=50)
```

The simulation found a concrete sequence of events that violates an invariant. The minimal failure case is the shortest event sequence that reproduces the violation — the shrinker removes unnecessary steps so the root cause is clear.

### What to look for in a rejection

1. **Which invariant was violated** — `Conservation` means total resource usage exceeded capacity; `PoolBounded` means a connection pool grew past its configured limit
2. **Which instances were involved** — the instance numbers show which upstream actors participated in the failure
3. **The state transitions** — trace through the steps to understand the interleaving that caused the failure
4. **The estimated failure rate** — a high rate (>10%) means the model fails routinely under load; a low rate (<1%) means it's a rare race condition

## Behavioral patterns

Only relationships with a `pattern` field in their template get behavioral verification. The standard patterns are:

- **`finite_resource`** — non-pooled contention for a shared downstream. Checks that total concurrent usage doesn't exceed capacity.
- **`finite_pooled_resource`** — pooled contention with connection establishment time. Checks both total connection count and per-instance pool bounds.

Relationships without a pattern get arithmetic checks only — `verify` skips them.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--failure-rate` | 0.001 (0.1%) | Maximum acceptable failure probability |
| `--confidence` | 0.99 (99%) | Required confidence in the result |
| `--shrink-budget` | 2000 | Maximum attempts to minimize a failure case |
| `--seed` | 0 (random) | Random seed for reproducibility |

Lower failure rate or higher confidence → more simulations needed to accept. A fixed seed makes results reproducible for debugging.

## After verification

- If verification passes: the model's concurrency properties are sound at the stated confidence level
- If verification rejects: examine the minimal failure case, then update the model (increase capacity, reduce pool sizes, add instances) and re-verify
- Consider running the outage report skill on models that have both check findings and verification rejections — the behavioral details add realism to the incident narrative
