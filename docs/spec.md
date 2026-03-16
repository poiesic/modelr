# modelr Specification

## 1. Overview

modelr is a composable system modeling tool with explicit uncertainty. Practitioners describe systems as YAML models containing components, edges, relationships, and known unknowns. modelr parses these models, validates them against a type system, runs constraint checks, and surfaces findings — violations, gaps, and assumptions.

The model file is the primary artifact. All tools (check, validate, visualize, verify) operate on it.

### Design principles

- **Declarative models, imperative checks** — the model says "this is my system"; the tool says "here's what could go wrong"
- **Explicit uncertainty** — unknown values, deferred decisions, and unresolved tradeoffs are first-class model elements
- **Medium-fidelity defaults** — infrastructure characteristics (connection setup time, etc.) have well-known defaults so practitioners don't need to supply them
- **Assumptions are visible** — any value filled from a default is reported, not hidden

### Implementation

- Language: Go
- Module: `github.com/poiesic/modelr`
- YAML: `gopkg.in/yaml.v3`
- MCP: `github.com/mark3labs/mcp-go`
- CLI: `github.com/urfave/cli/v3`
- Standalone binary, also runs as MCP server over stdio

---

## 2. Model Format

A model is a YAML file with extension `.model.yaml`.

### 2.1 Top-level structure

```yaml
version: "0.2"
name: string              # human-readable system name
description: string       # brief summary
components: []            # required, non-empty
edges: []                 # required, may be empty
relationships: []         # optional, defaults to []
known_unknowns: []        # optional, defaults to []
```

Required fields: `version`, `name`, `description`, `components`, `edges`.

### 2.2 Components

```yaml
- name: string            # unique identifier within the model
  type: string            # references a type schema (e.g., "server", "datastore")
  description: string
  properties:             # key-value map, keys defined by the type schema
    <key>: number | string | null
```

Required fields: `name`, `type`, `description`. Properties default to empty map if omitted.

Component names must be unique. Duplicate names are a parse error.

### 2.3 Edges

```yaml
- source: string          # component name
  target: string          # component name
  operation: read | write | read_write
  description: string
  properties:
    <key>: number | string | null
```

Required fields: `source`, `target`, `operation`, `description`. Properties default to empty map if omitted.

Source and target must reference existing component names. Unknown references are a parse error.

Edge property `pool_size` is rejected with a clear error directing users to `min_pool_size` and `max_pool_size`.

### 2.4 Relationships

```yaml
- template: string        # references a relationship template name
  upstream: string        # component name
  downstream: string      # component name
```

Required fields: `template`, `upstream`, `downstream`. Upstream and downstream must reference existing component names.

### 2.5 Known unknowns

```yaml
- id: string              # unique identifier
  component: string       # optional, component name
  edge: string            # optional, "source->target" notation
  category: UncertaintyCategory
  description: string
  impact: string
```

Required fields: `id`, `category`, `description`, `impact`.

### 2.6 Uncertainty categories

| Category | Meaning |
|----------|---------|
| `unstated_constraint` | A limit or bound that should exist but isn't specified |
| `unresolved_tradeoff` | A design decision with competing options, not yet decided |
| `undefined_boundary` | A system boundary or scope that isn't defined |
| `assumed_context` | An assumption about the operating environment |
| `deferred_decision` | A decision explicitly postponed |
| `unknown_unknown` | Something we know we don't know, but can't yet characterize |

### 2.7 Property null semantics

A property value of `null` means "not specified." Null values propagate through arithmetic expressions and produce indeterminate results when they reach a comparison operator. This triggers a known unknown rather than a finding.

---

## 3. Type System

### 3.1 Type schemas

A type schema defines the expected properties for a component type.

```yaml
name: string
description: string
properties:
  <property_name>:
    type: number | string
    unit: string
    description: string
    default: number | string    # optional
```

Type schemas are node definitions loaded via the building block resolution order (see section 7).

### 3.2 Standard type schemas

**server**
| Property | Type | Unit | Default | Description |
|----------|------|------|---------|-------------|
| `max_connections` | number | connections | — | Maximum concurrent connections |
| `min_instances` | number | instances | 1 | Minimum number of server instances |
| `max_instances` | number | instances | 1 | Maximum number of server instances |

**datastore**
| Property | Type | Unit | Default | Description |
|----------|------|------|---------|-------------|
| `max_connections` | number | connections | — | Maximum concurrent connections |
| `max_ops_per_sec` | number | ops/s | — | Maximum operations per second |
| `conn_establish_ms` | number | ms | 20 | Time to establish a new connection (TCP + TLS + auth) |

**queue**
| Property | Type | Unit | Default | Description |
|----------|------|------|---------|-------------|
| `max_depth` | number | messages | — | Maximum queue depth before backpressure |
| `max_throughput` | number | msg/s | — | Maximum messages per second |

### 3.3 Validation behavior

During validation, for each component:

1. Look up the type schema by component type
2. For each property defined in the schema but missing from the component:
   - If the schema has a `default`, fill it in and record an **assumption**
   - Otherwise, set to `null` and generate a **known unknown** (category: `unstated_constraint`)
3. Unknown component types produce a warning (not an error)

### 3.4 Assumptions

An assumption records that a property value was filled from a default rather than explicitly provided by the practitioner.

```go
type Assumption struct {
    Property  string
    Component string // optional
    Edge      string // optional
    Value     any    // the default value used
    Source    string // e.g., "datastore type schema"
}
```

Assumptions appear in all output formats so practitioners know which values were inferred.

---

## 4. Relationships and Checks

### 4.1 Relationship templates

A relationship template defines a set of checks between an upstream and downstream component.

```yaml
name: string
description: string
pattern: string              # optional: behavioral verification pattern (see section 8)
resolve:
  <variable_name>: <scope>.<property_name>
checks:
  - name: string
    expression: string       # arithmetic comparison expression
    violation: string        # human-readable violation description
```

Scope is one of: `upstream`, `downstream`, `edge`.

The optional `pattern` field declares a behavioral verification pattern for `modelr verify`. Templates without it get arithmetic checks only. See section 8.4 for details.

Relationship templates are loaded via the building block resolution order (see section 7).

### 4.2 Standard relationship templates

**capacity_chain** — upstream concurrency vs downstream capacity. Pattern: `finite_resource`.

Resolve:
| Variable | Binding |
|----------|---------|
| `upstream_rate` | `upstream.max_connections` |
| `instances` | `upstream.max_instances` |
| `operation_cost` | `edge.avg_operation_ms` |
| `downstream_capacity` | `downstream.max_connections` |
| `downstream_instances` | `downstream.max_instances` |

Checks:
- `throughput`: `upstream_rate * instances * operation_cost / 1000 <= downstream_capacity * max(downstream_instances, 1)`

**pooled_capacity_chain** — pooled connection count and throughput. Pattern: `finite_pooled_resource`.

Resolve:
| Variable | Binding |
|----------|---------|
| `min_pool_size` | `edge.min_pool_size` |
| `max_pool_size` | `edge.max_pool_size` |
| `instances` | `upstream.max_instances` |
| `operation_cost` | `edge.avg_operation_ms` |
| `downstream_capacity` | `downstream.max_connections` |
| `downstream_max_ops` | `downstream.max_ops_per_sec` |
| `conn_establish_ms` | `downstream.conn_establish_ms` |

Checks:
- `connection_limit`: `max_pool_size * instances <= downstream_capacity`
- `throughput`: `max_pool_size * instances * 1000 / operation_cost <= downstream_max_ops`

### 4.3 Expression language

A small arithmetic expression language with comparison operators and null propagation.

**Grammar:**
```
comparison := arithmetic (("<=" | ">=" | "<" | ">") arithmetic)?
arithmetic := term (('+' | '-') term)*
term       := unary (('*' | '/') unary)*
unary      := '-'? primary
primary    := NUMBER | IDENTIFIER | '(' comparison ')' | FUNC '(' args ')'
args       := arithmetic (',' arithmetic)*
```

**Operators:** `+`, `-`, `*`, `/`, `<`, `>`, `<=`, `>=`

**Functions:**
- `max(a, b)` — returns the larger value. `max(null, x) = x`, `max(x, null) = x`, `max(null, null) = null`
- `min(a, b)` — returns the smaller value. `min(null, x) = null`, `min(x, null) = null`

**Null propagation:**
- Arithmetic with null propagates: `null + 5 = null`, `null * 5 = null`
- Division by zero is an error
- If null reaches a comparison operator, the result is `null` (indeterminate)

**Evaluation:**
- `evaluateComparison(expr, variables) → bool | null` — evaluates a comparison expression
- `evaluateNumeric(expr, variables) → number | null` — evaluates an arithmetic expression without comparison

### 4.4 Check execution

For each relationship in the model:

1. Look up the relationship template
2. Resolve all variables from upstream/downstream/edge properties
3. If any variable resolves to a string, skip (not evaluable)
4. For each check in the template:
   - Evaluate the expression
   - If result is `null`: generate a known unknown (indeterminate)
   - If result is `false`: generate a **finding** (violation)

### 4.5 Findings

```go
type Finding struct {
    Severity     string // "error" or "warning"
    Relationship string // template name
    Upstream     string // component name
    Downstream   string // component name
    Description  string // includes expression and resolved values
    Suggestion   string // remediation guidance
    Kind         string // "arithmetic" or "behavioral" (optional)
}
```

### 4.6 Check result

```go
type CheckResult struct {
    Findings     []Finding
    KnownUnknowns []KnownUnknown
    Summary      string
}
```

Summary: "All relationship constraints satisfied." or "Found N relationship violation(s)."

---

## 5. Pipeline

### 5.1 Parse → Validate → Check

```
.model.yaml → Parse → SystemModel → Validate → (model, warnings, assumptions) → Check → CheckResult
```

1. **Parse**: YAML → `SystemModel`. Structural validation (required fields, unique names, valid references). Rejects `pool_size`.
2. **Validate**: Fill defaults, generate known unknowns for missing properties, resolve relationship bindings, record assumptions.
3. **Check**: Evaluate relationship template expressions. Produce findings and indeterminate known unknowns.

### 5.2 Output formats

**`.checked.yaml`** — written as a sibling of the input model.
```yaml
model: <name>
checked_at: <ISO 8601 timestamp>
known_unknowns: []    # merged from model + validator + checker
findings: []
assumptions: []
warnings: []
summary: <string>
```

**`.verified.yaml`** — written by behavioral verification.
```yaml
model: <name>
verified_at: <ISO 8601 timestamp>
verifications: []
behavioral_findings: []
known_unknowns: []
assumptions: []
summary: <string>
```

### 5.3 Validation of relationship bindings

During validation, for each relationship:
1. Look up the template
2. For each variable in `resolve`:
   - Parse `scope.property`
   - If scope is `edge` and no edge exists between upstream/downstream: warning
   - If the referenced property is missing: set to `null`, generate known unknown

---

## 6. CLI Interface

### 6.1 Commands

```
modelr check <path>           # parse, validate, check; write .checked.yaml
modelr validate <path>        # parse, validate only; print warnings and assumptions
modelr visualize <path>       # check + generate DOT/SVG
modelr verify <path>          # behavioral verification; write .verified.yaml
modelr definitions            # list available node and relationship definitions
modelr cache status            # cache age, size, staleness
modelr cache refresh           # refresh cache (--rebuild | --auto)
```

### 6.2 MCP server mode

```
modelr mcp                    # start MCP server on stdio
```

Registers five tools:

| Tool | Description |
|------|-------------|
| `list_definitions` | List available node and relationship definitions |
| `check_model` | Parse, validate, check; write .checked.yaml |
| `validate_model` | Parse, validate; return model with warnings and assumptions |
| `visualize_model` | Check + generate DOT/SVG visualization |
| `verify_model` | Behavioral verification via state exploration |

Tool input/output follows the MCP protocol via `mcp-go`.

### 6.3 Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `$MODELR_PATH` | (empty) | Colon-separated list of directories containing building block YAML files |

When `$MODELR_PATH` is empty, only inline definitions (in the model file) and embedded defaults are used. See section 7 for full resolution behavior.

### 6.4 Tool state directory

All tool state lives in `$HOME/.modelr/`:

| Path | Purpose |
|------|---------|
| `$HOME/.modelr/cache/definitions.json` | Cached definition index built by `modelr cache` |

### 6.5 Verbose flag

All model commands accept `--verbose` to show definition resolution, validation details, and assumption provenance.

---

## 7. Building Blocks

### 7.1 Definitions

A **building block** is either a node definition (`kind: node`) or a relationship definition (`kind: relationship`). Both are YAML documents with a `kind` and `name` field.

```yaml
---
kind: node
name: postgres
description: PostgreSQL database
properties:
  max_connections:
    type: number
    unit: connections
    description: Maximum concurrent connections
  conn_establish_ms:
    type: number
    unit: ms
    description: Time to establish a new connection
    default: 20
---
kind: relationship
name: pooled_capacity_chain
description: Pooled connection check
resolve:
  max_pool_size: edge.max_pool_size
  instances: upstream.max_instances
  downstream_capacity: downstream.max_connections
checks:
  - name: connection_limit
    expression: "max_pool_size * instances <= downstream_capacity"
    violation: "Connection exhaustion"
```

A single YAML file can contain multiple documents (separated by `---`), mixing node and relationship definitions. This allows packaging related definitions together (e.g., a `postgres.yaml` that defines the node type and its typical relationships).

### 7.2 Resolution order

Definitions are resolved by `kind` + `name`. First match wins.

1. **Inline definitions** — `kind: node` or `kind: relationship` documents in the model file itself
2. **`$MODELR_PATH` entries** — environment variable containing a colon-separated list of directories, walked left to right. Each directory contains `*.yaml` files with compound documents.
3. **Embedded defaults** — standard definitions compiled into the binary via `//go:embed`

When `$MODELR_PATH` is empty, only inline and embedded definitions are used.

### 7.3 Loading behavior

1. Parse the model file. Separate documents by `kind`:
   - `kind: node` and `kind: relationship` → index as inline definitions
   - Document with `version`/`name`/`components` → the model itself
2. Walk `$MODELR_PATH` entries. For each directory, load all `*.yaml` files, parse all `---`-separated documents, index by `kind` + `name`. Skip any `kind` + `name` already defined by a higher-precedence source.
3. Load embedded defaults. Skip any `kind` + `name` already defined.

### 7.4 Verbose output

With `--verbose`, commands show where each definition was loaded from:

```
Loading definitions:
  node/server                       ← embedded
  node/postgres                     ← /opt/mycompany/modelr/postgres.yaml
  node/queue                        ← embedded
  relationship/capacity_chain       ← embedded
  relationship/pooled_capacity_chain ← /opt/mycompany/modelr/postgres.yaml
```

### 7.5 Standard embedded definitions

The following are compiled into the binary:

**Nodes:** `server`, `datastore`, `queue`

**Relationships:** `capacity_chain`, `pooled_capacity_chain`

See sections 3.2 and 4.2 for their full definitions.

### 7.6 Definition cache

To avoid re-parsing every YAML file on every command, `modelr cache` walks `$MODELR_PATH`, parses all files, and writes a cache to `$HOME/.modelr/cache/definitions.json`. The cache maps `kind/name` to file path, document index, and source.

```json
{
  "path": "/opt/mycompany/modelr:/home/user/project/modelr",
  "entries": [
    {"kind": "node", "name": "postgres", "file": "/opt/mycompany/modelr/postgres.yaml", "doc": 0},
    {"kind": "relationship", "name": "pooled_capacity_chain", "file": "/opt/mycompany/modelr/postgres.yaml", "doc": 1}
  ]
}
```

**Cache subcommands:**

`modelr cache status` — prints cache metadata:
```
Last refreshed: 2026-03-16T14:32:00Z (2 hours ago)
Size on disk:   4.2 KB
Status:         stale ($MODELR_PATH changed)
```

`modelr cache refresh` — refreshes the cache. Accepts two mutually exclusive flags:
- `--rebuild` — deletes the existing cache and rebuilds from scratch
- `--auto` — only rebuilds if the cache is stale; no-op if current

With no flags, `refresh` always rebuilds (same behavior as `--rebuild`).

**Staleness detection:**

The cache stores the following metadata for staleness comparison:
- The value of `$MODELR_PATH` at cache time
- For each file on the path: file path, mtime, file size

The cache is considered **stale** if any of:
- `$MODELR_PATH` value differs from cached value
- Any cached file has a newer mtime than the cached mtime
- Any cached file no longer exists
- New `*.yaml` files exist in a `$MODELR_PATH` directory that weren't in the cache

**Auto-refresh behavior:**

If no cache exists when a command runs, an automatic refresh is performed. Commands do **not** auto-refresh a stale cache — they emit a warning instead:

```
warning: definition cache is stale (run 'modelr cache refresh' to update)
```

This avoids surprising filesystem walks during normal usage. Use `modelr cache refresh --auto` in scripts or hooks to refresh only when needed.

**General cache properties:**
- Embedded defaults are never cached (they're in the binary and always available)
- Inline definitions (in the model file) are resolved at parse time, not cached

**Shadowing notifications:**

When building the cache, if a definition in `$MODELR_PATH` shadows an embedded default or a definition from an earlier path entry, `modelr cache` emits an informational message:

```
info: node/datastore shadowed (embedded → /opt/mycompany/modelr/postgres.yaml)
info: relationship/pooled_capacity_chain shadowed (embedded → /opt/mycompany/modelr/postgres.yaml)
info: node/server shadowed (/opt/mycompany/modelr/server.yaml → ./modelr/server.yaml)
```

These messages are also emitted during auto-cache (first run or `$MODELR_PATH` change). Commands that trigger auto-cache print the shadowing messages to stderr before proceeding with normal output.

Inline definitions that shadow cached or embedded definitions produce the same informational message at parse time, not cache time.

### 7.7 Validation of relationship-to-node wiring

During validation, if a relationship template's `resolve` bindings reference a property that doesn't exist in the downstream or upstream node's type schema, the validator produces a warning. This catches misapplied relationships (e.g., `pooled_capacity_chain` applied to a `queue` downstream that doesn't define `conn_establish_ms`).

---

## 8. Behavioral Verification

### 8.1 Approach

Behavioral verification uses **property-based simulation** with **Wald's sequential probability ratio test (SPRT)** to provide quantified confidence in temporal properties. No external tools are required — the simulation engine is implemented in Go.

The mental model for practitioners: "we simulated your system under random concurrent conditions and here's what we found."

Arithmetic checks (section 4) verify steady-state capacity math. Behavioral verification catches what arithmetic misses: timing-dependent failures, cold-start races, and resource contention under concurrent load.

### 8.2 Behavioral patterns

Most concurrency-related failures in system models fall into a small number of categories. Rather than requiring practitioners to write state machines, modelr provides **built-in behavioral patterns** that the simulation engine knows how to execute. A relationship template declares which pattern applies via the `pattern` field; the engine builds the state machine automatically from the template's `resolve` bindings.

Relationship templates without a `pattern` field get arithmetic checks only. `modelr verify` skips them.

#### 8.2.1 `finite_resource`

Models non-pooled contention for a shared downstream resource. Multiple upstream instances send requests that each consume a unit of downstream capacity for the duration of the operation.

**Roles (mapped from `resolve` bindings):**

| Role | Description | Typical binding |
|------|-------------|-----------------|
| `instances` | Number of concurrent upstream actors | `upstream.max_instances` |
| `resource_capacity` | Downstream resource limit | `downstream.max_connections` |
| `operation_time` | How long each request holds a resource unit | `edge.avg_operation_ms` |

**State (per instance):** `active_requests`

**Shared state:** `used_resources` (across all instances sharing the same downstream)

**Rules:**
- `RequestArrives(instance)` — increments `active_requests` and `used_resources`
- `RequestCompletes(instance)` — guard: `active_requests > 0`; decrements both

**Built-in invariants:**
- `Conservation` — `used_resources <= resource_capacity`

**How roles are inferred:** The engine maps resolved variables to roles by convention. A variable whose binding ends in `max_instances` maps to `instances`. A variable whose binding ends in `max_connections` or `max_ops_per_sec` maps to `resource_capacity`. A variable whose binding ends in `_ms` and is scoped to `edge` maps to `operation_time`. If the engine cannot infer all required roles, `modelr verify` reports an error for that relationship.

#### 8.2.2 `finite_pooled_resource`

Models pooled contention where each upstream instance maintains a connection pool that grows on demand. Pool growth takes time (connection establishment), creating a window where requests may queue while waiting for a ready connection.

**Roles (mapped from `resolve` bindings):**

| Role | Description | Typical binding |
|------|-------------|-----------------|
| `instances` | Number of concurrent upstream actors | `upstream.max_instances` |
| `pool_capacity` | Max pool size per instance | `edge.max_pool_size` |
| `resource_capacity` | Downstream resource limit | `downstream.max_connections` |
| `acquire_time` | Time to establish a new connection | `downstream.conn_establish_ms` |
| `operation_time` | How long each request holds a connection | `edge.avg_operation_ms` |

**State (per instance):** `pool` (ready connections), `in_flight` (connections being established), `pending` (requests waiting)

**Shared state:** `used_connections` (across all instances sharing the same downstream)

**Rules:**
- `RequestArrives(instance)` — increments `pending`
- `StartGrowth(instance)` — guard: `pending > 0` and `pool + in_flight < pool_capacity`; decrements `pending`, increments `in_flight` and `used_connections`
- `GrowComplete(instance)` — guard: `in_flight > 0`; decrements `in_flight`, increments `pool`
- `RequestCompletes(instance)` — guard: `pool > 0`; decrements `pool` and `used_connections`

**Built-in invariants:**
- `Conservation` — `used_connections <= resource_capacity`
- `PoolBounded` — `pool + in_flight <= pool_capacity` (per instance)

### 8.3 Composition

Relationships are the unit of behavioral simulation. Each relationship with a `pattern` field produces an independent state machine with per-instance state replicated by the upstream's instance count.

Composition happens through **shared state**. When two relationships reference the same downstream component, their shared state variables (e.g., `used_connections`) are unified — both state machines compete for the same resource. The engine infers sharing from the `resolve` bindings: if two relationships both resolve a variable from the same downstream component property, the corresponding shared state is the same counter.

A model with 3 relationships that have patterns produces 3 state machines. If two of them share a downstream, they share a resource counter, and the simulation naturally produces contention between them.

Simulations use the model's actual numbers (50 instances, 50 connections) — no scaling needed.

### 8.4 Pattern field on relationship templates

The `pattern` field is an optional addition to the relationship template format (section 4.1):

```yaml
name: string
description: string
pattern: string              # optional: finite_resource | finite_pooled_resource
resolve:
  <variable_name>: <scope>.<property_name>
checks:
  - name: string
    expression: string
    violation: string
```

Standard relationship templates with patterns:

| Template | Pattern |
|----------|---------|
| `capacity_chain` | `finite_resource` |
| `pooled_capacity_chain` | `finite_pooled_resource` |

Custom relationship templates loaded via `$MODELR_PATH` can use the same patterns. A template with an unknown `pattern` value produces a warning.

### 8.5 Bytestream-based determinism and shrinking

Inspired by Hypothesis: the entire simulation is deterministic from a single bytestream.

**Generation**: Every test case is produced by deterministically reading from a stream of random bytes. Rule selection is driven by bytes. Instance selection is driven by bytes. The entire simulation — which rules fire, which instance they target, how many steps — is determined by one bytestream. A simulation can be replayed exactly by replaying its bytestream.

**Shrinking**: When a violation is found, the shrinker operates on the bytestream, not on domain concepts. Shrinking operations are simple and type-agnostic:

- **Delete chunks** of bytes — removes unnecessary steps
- **Zero chunks** — produces "smaller" values from strategies
- **Reduce individual bytes** — binary search toward zero
- **Swap chunks** — move larger values later for lexicographic ordering

If a shrunk bytestream produces a simulation that doesn't fail, it's discarded. Any bytestream produces a valid (if different) test case. The shrinker doesn't need to understand what the bytes mean.

This gives practitioners a **minimal failure case** — the shortest event sequence that reproduces the violation.

### 8.6 SPRT integration

Wald's sequential probability ratio test determines how many simulations to run. Rather than a fixed count, it adapts based on observed evidence:

- **H0 (safe):** true failure rate ≤ target failure rate
- **H1 (flawed):** true failure rate ≥ p1 (derived from target)
- After each simulation, compute the likelihood ratio against acceptance/rejection thresholds derived from α and β error bounds
- **Three outcomes at each step:** accept H0 (stop — safe), accept H1 (stop — flawed), or continue

This gives:
- **Fast rejection** — broken systems fail in a handful of simulations
- **Fast acceptance** — safe systems reach confidence quickly
- **Rigorous confidence** — error bounds are mathematically guaranteed
- **No arbitrary sample size** — the evidence determines when to stop

### 8.7 Defaults and flags

| Parameter | Default | Flag | Description |
|-----------|---------|------|-------------|
| Target failure rate | 0.1% | `--failure-rate` | Maximum acceptable failure probability |
| Confidence level | 99% | `--confidence` | Required confidence in the result |

### 8.8 Output

**Pass:**
```
Accepted after 312 simulations (0 failures)
Confidence: 99% that failure rate < 0.1%
```

**Fail:**
```
Rejected after 23 simulations (4 failures)
Estimated failure rate: 17.4%
Minimal failure case:
  1. RequestArrives(instance=3)  → pending=1, pool=0
  Property violated: Conservation (used_connections=51 > downstream_capacity=50)
```

The `.verified.yaml` output includes:

```yaml
verifications:
  - upstream: ws-server
    downstream: postgres
    pattern: finite_pooled_resource
    result: pass
    simulations: 312
    failures: 0
    confidence: 0.99
    failure_rate_bound: 0.001
    assumptions:
      - property: conn_establish_ms
        value: 20
        source: datastore type schema
```

### 8.9 Relationship to arithmetic checks

Arithmetic checks and behavioral verification are complementary:

| Approach | What it catches | Speed | Guarantee |
|----------|----------------|-------|-----------|
| **Arithmetic checks** (section 4) | Static capacity oversubscription | Milliseconds, deterministic | Exact for steady-state |
| **Behavioral verification** | Timing-dependent failures, cold-start races, resource contention under concurrent load | Seconds, statistical | Confidence bounds via SPRT |

Arithmetic is the fast first pass. Behavioral verification catches concurrency dynamics that arithmetic cannot express — situations where the steady-state math works but transient states under realistic interleaving cause failures.

### 8.10 Future extensions

Custom invariants on behavioral patterns are a planned extension. This would allow practitioners to express constraints like SLA bounds (`no_request_waits_longer_than: 100ms`) or queue depth limits on top of the built-in pattern state, without defining custom state machines.

---

## 9. Code Generation

*To be designed. Questions to explore:*

- What gets generated from a model? (scaffolds, configs, infrastructure-as-code, application code)
- How do type schemas map to generated code? (e.g., "datastore" → connection pool setup)
- How does the model drive test generation?
- How do incremental model changes map to code changes?

---

## 10. Visualization

### 10.1 DOT generation

Generates Graphviz DOT from a checked model.

**Node styling by component type:**
| Type | Shape | Color |
|------|-------|-------|
| `server` | box | `#4A90D9` |
| `datastore` | cylinder | `#50B83C` |
| `queue` | parallelogram | `#E6A23C` |
| default | box | `#999999` |

**Node labels** include component name, type, and non-null properties.

**Edge styling by status:**
| Status | Style | Color |
|--------|-------|-------|
| Has findings | solid, penwidth=2.5 | red |
| Has known unknowns | dashed, penwidth=1.5 | orange |
| Clean | solid, penwidth=1 | black |

Components with known unknowns get an orange border (penwidth=2).

Edge labels include operation name, non-null properties, and warning symbols for violations.

### 10.2 Rendering

If `graphviz` is installed, render the DOT to SVG or PNG. If not, write the `.dot` file only and suggest installation.
