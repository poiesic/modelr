# modelr

Your system model says 50 connections will be fine. modelr runs 300 simulations and finds the cold-start race that exhausts them in 200ms.

modelr is a composable system modeling tool with explicit uncertainty. Describe your system as a YAML model — components, edges, capacity constraints, and the things you *haven't decided yet* — and modelr tells you what's going to break.

## What it does

**Arithmetic checks** catch the obvious stuff: your 50 app instances each holding 10 connections will blow past your database's 200-connection limit. Milliseconds, deterministic, exact.

**Behavioral verification** catches what arithmetic can't: the cold-start race where all 50 instances open pools simultaneously, the thundering herd after a deploy, the 0.1% failure rate that pages you at 3am. Property-based simulation with [SPRT](https://en.wikipedia.org/wiki/Sequential_probability_ratio_test) gives you statistical confidence bounds, not vibes.

**Explicit uncertainty** makes gaps visible instead of dangerous. A `null` property isn't a bug — it's a known unknown that propagates through checks and shows up in findings. Deferred decisions, unresolved tradeoffs, and unstated constraints are first-class model elements, not comments you'll forget about.

## Quick start

```bash
go install github.com/poiesic/modelr/cmd/modelr@latest
```

```bash
modelr check infra.model.yaml       # Arithmetic constraint checks → .checked.yaml
modelr verify infra.model.yaml      # Behavioral simulation → .verified.yaml
modelr visualize infra.model.yaml   # DOT/SVG graph with findings highlighted
modelr definitions                  # Show available component types and relationships
```

## Example

```yaml
version: "0.2"
name: chat-backend
description: WebSocket servers talking to Postgres

components:
  - name: ws-server
    type: server
    description: WebSocket connection handlers
    properties:
      max_connections: 10000
      max_instances: 50

  - name: postgres
    type: datastore
    description: Primary database
    properties:
      max_connections: 200

edges:
  - source: ws-server
    target: postgres
    operation: read_write
    description: Session state and message persistence
    properties:
      min_pool_size: 2
      max_pool_size: 10
      avg_operation_ms: 5

relationships:
  - template: pooled_capacity_chain
    upstream: ws-server
    downstream: postgres

known_unknowns:
  - id: read-replica-strategy
    component: postgres
    category: deferred_decision
    description: Read replica topology not yet decided
    impact: Write primary may be overloaded if reads aren't offloaded
```

`modelr check` evaluates the arithmetic: can 50 instances with 10-connection pools fit inside 200 database connections? (50 * 10 = 500 > 200 — no.)

`modelr verify` goes further: even if you fix the pool sizes, does the system survive a cold start where every instance grows its pool at once, each connection taking 20ms to establish?

## Claude Code integration

modelr ships with Claude Code skills and runs as an MCP server:

```bash
modelr init    # Set up .claude/skills/ and .claude/mcp.json
modelr mcp     # Start MCP server on stdio
```

Three skills are scaffolded:
- **modelr-model** — generate and check system models from natural language
- **modelr-verify** — run behavioral verification and interpret results
- **modelr-outage-report** — turn findings into plausible incident narratives (pre-mortem analysis)

## Design principles

- **Declarative models, imperative checks** — the model says "this is my system"; the tool says "here's what could go wrong"
- **Medium-fidelity defaults** — connection establishment time, instance counts, and other infrastructure constants have well-known defaults so you don't need to supply them
- **Assumptions are visible** — any value filled from a default is reported, not hidden
- **Composable building blocks** — define your own component types and relationship templates via `$MODELR_PATH`

## Build

Requires Go 1.25+ and [Task](https://taskfile.dev).

```bash
task build   # Build binary → bin/modelr
task test    # Run all tests
task lint    # Run linter
```

## Documentation

See [docs/spec.md](docs/spec.md) for the full specification.

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.
