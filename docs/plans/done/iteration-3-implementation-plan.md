# Iteration 3 — Implementation Plan

## Goal

Implement the behavioral verification engine from spec section 8. Relationship templates declare a `pattern` field; the engine builds state machines from built-in behavioral patterns, runs bytestream-deterministic simulations with SPRT-driven adaptive stopping, and shrinks failures to minimal reproductions. This adds the `modelr verify` CLI command, wires up the `verify_model` MCP tool, and writes `.verified.yaml` output.

## Prerequisites

Iterations 0–2 are complete:
- Full Parse → Validate → Check pipeline
- Expression evaluator, constraint checker
- `$MODELR_PATH` resolution, definition cache
- DOT/SVG visualization
- MCP server with `verify_model` stub
- CLI with `check`, `validate`, `visualize`, `cache`, `definitions`, `init`, `mcp`

## Scope

### In scope

| Spec Section | What |
|---|---|
| 8.2 | Two behavioral patterns: `finite_resource`, `finite_pooled_resource` |
| 8.3 | Composition via shared state across relationships |
| 8.4 | `pattern` field on relationship templates + embedded definitions |
| 8.5 | Bytestream-based deterministic simulation + shrinking |
| 8.6 | SPRT adaptive stopping |
| 8.7–8.8 | CLI flags, output format, `.verified.yaml` |
| 6.1 | `modelr verify <path>` CLI command |
| 6.2 | Wire up `verify_model` MCP tool |

### Deferred

| Feature | Why |
|---|---|
| Custom invariants (8.10) | Future extension, not needed for core engine |
| Engine soundness validation | Separate effort after engine exists (metamorphic testing, TLA+ cross-validation) |

## File Layout

```
internal/verify/
  types.go                  # VerificationResult, SimulationResult, RoleMap, etc.
  types_test.go
  roles.go                  # Role inference from resolve bindings + pattern
  roles_test.go
  bytestream.go             # Deterministic byte source for simulations
  bytestream_test.go
  machine.go                # State machine interface + rule/invariant types
  machine_test.go
  finite_resource.go        # finite_resource pattern implementation
  finite_resource_test.go
  finite_pooled.go          # finite_pooled_resource pattern implementation
  finite_pooled_test.go
  composer.go               # Shared state unification across relationships
  composer_test.go
  simulator.go              # Single simulation runner
  simulator_test.go
  sprt.go                   # Wald's SPRT calculator
  sprt_test.go
  shrinker.go               # Bytestream shrinking
  shrinker_test.go
  verify.go                 # Top-level Verify() orchestrator
  verify_test.go
  output.go                 # .verified.yaml serialization
  output_test.go
internal/loader/
  types.go                  # MODIFIED: add Pattern field to RelationshipDef
embed/
  definitions.yaml          # MODIFIED: add pattern fields to standard templates
cmd/modelr/
  main.go                   # MODIFIED: add verify command
  main_test.go              # MODIFIED: verify command tests
internal/mcp/
  server.go                 # MODIFIED: wire up verify_model handler
  server_test.go            # MODIFIED: verify_model tests
```

---

## Phase 1 — Pattern Field and Role Inference [COMPLETE]

Add the `pattern` field to the type system and embedded definitions, then build the role inference engine that maps resolve bindings to pattern roles.

### Step 1.1: Add `pattern` field to `RelationshipDef` [x]

**Test (red):** `internal/loader/loader_test.go`
- `TestRelationshipDefPatternField` — create `RelationshipDef` with `Pattern: "finite_resource"`, marshal to YAML → contains `pattern:`, unmarshal back → field preserved
- `TestEmbeddedCapacityChainHasPattern` — load embedded defs → `capacity_chain` has `Pattern: "finite_resource"`
- `TestEmbeddedPooledCapacityChainHasPattern` — `pooled_capacity_chain` has `Pattern: "finite_pooled_resource"`

**Code (green):**
- Add `Pattern string \`yaml:"pattern,omitempty"\`` to `RelationshipDef` in `internal/loader/types.go`
- Add `pattern: finite_resource` to `capacity_chain` in `embed/definitions.yaml`
- Add `pattern: finite_pooled_resource` to `pooled_capacity_chain` in `embed/definitions.yaml`

**Refactor:** None expected.

### Step 1.2: Verification types (`internal/verify/types.go`) [x]

**Test (red):** `internal/verify/types_test.go`
- `TestRoleMapHasRequiredRoles` — `RoleMap` for `finite_resource` with all roles set → `Valid()` returns true
- `TestRoleMapMissingRole` — `RoleMap` missing `instances` → `Valid()` returns false, `Missing()` lists it

**Code (green):**

```go
type RoleMap struct {
    Instances        int
    ResourceCapacity int
    OperationTime    int     // milliseconds
    PoolCapacity     int     // finite_pooled_resource only
    AcquireTime      int     // finite_pooled_resource only
    Pattern          string
}

type VerificationResult struct {
    Verifications []Verification
    Summary       string
}

type Verification struct {
    Upstream         string
    Downstream       string
    Pattern          string
    Result           string  // "pass" or "fail"
    Simulations      int
    Failures         int
    Confidence       float64
    FailureRateBound float64
    MinimalFailure   []Step   // nil if passed
    Assumptions      []model.Assumption
}

type Step struct {
    Rule     string
    Instance int
    State    map[string]int  // snapshot after step
}
```

**Refactor:** None expected.

### Step 1.3: Role inference (`internal/verify/roles.go`) [x]

**Test (red):** `internal/verify/roles_test.go`
- `TestInferRolesFiniteResource` — `capacity_chain` resolve bindings → `RoleMap` with correct instances, resource_capacity, operation_time
- `TestInferRolesFinitePooledResource` — `pooled_capacity_chain` resolve bindings → correct pool_capacity, acquire_time, etc.
- `TestInferRolesUnknownPattern` — `pattern: "unknown"` → error
- `TestInferRolesMissingBinding` — resolve bindings missing `instances` equivalent → error listing what's missing
- `TestInferRolesFromCustomTemplate` — custom template with `pattern: finite_resource` and matching bindings → works

**Code (green):**

```go
func InferRoles(tmpl loader.RelationshipDef, model *model.SystemModel) (*RoleMap, error)
```

Inference rules (per spec 8.2):
- Binding ending in `max_instances` → `instances`
- Binding ending in `max_connections` or `max_ops_per_sec` → `resource_capacity`
- Binding ending in `_ms` scoped to `edge` → `operation_time`
- Binding ending in `max_pool_size` → `pool_capacity`
- Binding ending in `conn_establish_ms` or `_ms` scoped to `downstream` → `acquire_time`

Resolve the actual numeric values from the model's components/edges.

**Refactor:** None expected.

---

## Phase 2 — State Machines [COMPLETE]

Implement the two behavioral patterns as state machines. Each pattern defines its state shape, rules with guards, and invariants.

### Step 2.1: State machine interface (`internal/verify/machine.go`) [x]

**Test (red):** `internal/verify/machine_test.go`
- `TestRuleHasGuard` — rule with guard function → `CanFire(state)` respects guard
- `TestRuleAppliesEffect` — rule with effect → `Apply(state)` modifies state correctly
- `TestInvariantChecks` — invariant with check function → `Check(state)` returns true/false

**Code (green):**

```go
type State struct {
    PerInstance []InstanceState
    Shared      SharedState
}

type InstanceState struct {
    Values map[string]int
}

type SharedState struct {
    Values map[string]int
}

type Rule struct {
    Name    string
    CanFire func(state *State, instance int) bool
    Apply   func(state *State, instance int)
}

type Invariant struct {
    Name        string
    Description string
    Check       func(state *State, instance int) bool  // instance=-1 for global
}

type Machine struct {
    Roles      *RoleMap
    Rules      []Rule
    Invariants []Invariant
    InitState  func(roles *RoleMap) *State
}
```

**Refactor:** None expected.

### Step 2.2: `finite_resource` pattern (`internal/verify/finite_resource.go`) [x]

**Test (red):** `internal/verify/finite_resource_test.go`
- `TestFiniteResourceInitState` — 3 instances → state has 3 instance states (active=0) + shared (used=0)
- `TestFiniteResourceRequestArrives` — fires on any instance → active +1, used +1
- `TestFiniteResourceRequestCompletes` — guard: active > 0 → active -1, used -1
- `TestFiniteResourceRequestCompletesGuard` — active == 0 → cannot fire
- `TestFiniteResourceConservationPass` — used < capacity → invariant passes
- `TestFiniteResourceConservationFail` — used > capacity → invariant fails
- `TestFiniteResourceConcurrentExhaustion` — 3 instances, capacity 2 → sequence exists that violates conservation

**Code (green):**

```go
func NewFiniteResourceMachine(roles *RoleMap) *Machine
```

Two rules (RequestArrives, RequestCompletes), one invariant (Conservation).

**Refactor:** None expected.

### Step 2.3: `finite_pooled_resource` pattern (`internal/verify/finite_pooled.go`) [x]

**Test (red):** `internal/verify/finite_pooled_test.go`
- `TestPooledInitState` — 2 instances → per-instance (pool=0, in_flight=0, pending=0) + shared (used=0)
- `TestPooledRequestArrives` — pending +1
- `TestPooledStartGrowthGuard` — pending > 0 and pool + in_flight < pool_capacity → can fire
- `TestPooledStartGrowthBlocked` — pending == 0 → cannot fire
- `TestPooledStartGrowthAtCapacity` — pool + in_flight == pool_capacity → cannot fire
- `TestPooledStartGrowthEffect` — pending -1, in_flight +1, used_connections +1
- `TestPooledGrowComplete` — in_flight > 0 → in_flight -1, pool +1
- `TestPooledRequestCompletes` — pool > 0 → pool -1, used_connections -1
- `TestPooledConservationPass` — used <= capacity → passes
- `TestPooledConservationFail` — used > capacity → fails
- `TestPooledPoolBoundedPass` — pool + in_flight <= pool_capacity → passes
- `TestPooledPoolBoundedFail` — pool + in_flight > pool_capacity → fails

**Code (green):**

```go
func NewFinitePooledResourceMachine(roles *RoleMap) *Machine
```

Four rules, two invariants.

**Refactor:** None expected.

---

## Phase 3 — Bytestream and Simulation [COMPLETE]

### Step 3.1: Bytestream source (`internal/verify/bytestream.go`) [x]

**Test (red):** `internal/verify/bytestream_test.go`
- `TestBytestreamDeterministic` — same seed → same sequence of bytes
- `TestBytestreamDifferentSeeds` — different seeds → different sequences
- `TestBytestreamDrawByte` — draws byte in range [0, 255]
- `TestBytestreamDrawInt` — `DrawInt(n)` returns value in [0, n)
- `TestBytestreamRecordable` — after drawing, `Bytes()` returns all bytes drawn

**Code (green):**

```go
type Bytestream struct {
    rng     *rand.Rand
    drawn   []byte
}

func NewBytestream(seed int64) *Bytestream
func FromBytes(data []byte) *Bytestream  // replay from recorded bytes
func (b *Bytestream) DrawByte() byte
func (b *Bytestream) DrawInt(n int) int  // [0, n)
func (b *Bytestream) Bytes() []byte      // all bytes drawn so far
```

**Refactor:** None expected.

### Step 3.2: Single simulation runner (`internal/verify/simulator.go`) [x]

**Test (red):** `internal/verify/simulator_test.go`
- `TestSimulatePassingModel` — machine where invariants can't be violated (capacity >> instances) → no violation, returns steps executed
- `TestSimulateFailingModel` — machine where capacity is too low → finds violation, returns failing step with invariant name
- `TestSimulateDeterministic` — same bytestream → same step sequence
- `TestSimulateRespectsGuards` — only fireable rules are selected
- `TestSimulateMaxSteps` — simulation stops after max step count if no violation found

**Code (green):**

```go
type SimulationConfig struct {
    MaxSteps int  // default: 1000
}

type SimulationOutcome struct {
    Violated    bool
    Invariant   string        // which invariant failed
    Steps       []Step        // full trace
    FailedAt    int           // step index where violation occurred (-1 if pass)
}

func Simulate(machine *Machine, bs *Bytestream, config SimulationConfig) *SimulationOutcome
```

Each step: draw instance (via bytestream), collect fireable rules, draw rule (via bytestream), apply rule, check invariants. If no rules fireable, step is a no-op (draw bytes to keep stream advancing).

**Refactor:** None expected.

### Step 3.3: Composer — shared state unification (`internal/verify/composer.go`) [x]

**Test (red):** `internal/verify/composer_test.go`
- `TestComposeSingleRelationship` — 1 patterned relationship → 1 machine
- `TestComposeTwoRelationshipsSameDownstream` — A→C and B→C both `finite_resource` → 2 machines sharing resource counter; combined simulation can exhaust shared resource
- `TestComposeTwoRelationshipsDifferentDownstream` — A→C and A→D → 2 independent machines, no shared state
- `TestComposeSkipsNoPattern` — relationship without pattern → skipped
- `TestComposeUnknownPatternWarning` — unknown pattern → warning, relationship skipped

**Code (green):**

```go
type ComposedSimulation struct {
    Machines  []*Machine
    Shared    *SharedState  // unified shared state
}

func Compose(model *model.SystemModel, registry *loader.Registry) (*ComposedSimulation, []string, error)
```

For each relationship with a pattern:
1. Infer roles
2. Build machine
3. If downstream is shared with another machine's downstream, unify their shared state

Returns composed simulation + warnings.

**Refactor:** None expected.

---

## Phase 4 — SPRT [COMPLETE]

### Step 4.1: SPRT calculator (`internal/verify/sprt.go`) [x]

**Test (red):** `internal/verify/sprt_test.go`
- `TestSPRTAcceptAfterAllPass` — 500 simulations, 0 failures, target 0.1% → accepts H0
- `TestSPRTRejectAfterFailures` — 20 simulations, 5 failures, target 0.1% → rejects (accepts H1)
- `TestSPRTContinue` — 5 simulations, 0 failures → neither accepted nor rejected yet (continue)
- `TestSPRTDefaultParams` — default target=0.001, confidence=0.99 → correct thresholds
- `TestSPRTCustomParams` — target=0.01, confidence=0.95 → different thresholds
- `TestSPRTBoundary` — exactly at threshold → correct decision

**Code (green):**

```go
type SPRTConfig struct {
    TargetFailureRate float64  // p0, default 0.001
    Confidence        float64  // 1-α = 1-β, default 0.99
}

type SPRTDecision int
const (
    SPRTContinue SPRTDecision = iota
    SPRTAccept   // safe
    SPRTReject   // flawed
)

type SPRT struct {
    config     SPRTConfig
    lnA        float64  // upper threshold (reject)
    lnB        float64  // lower threshold (accept)
}

func NewSPRT(config SPRTConfig) *SPRT
func (s *SPRT) Update(failed bool) SPRTDecision
func (s *SPRT) Simulations() int
func (s *SPRT) Failures() int
func (s *SPRT) EstimatedFailureRate() float64
```

Wald's SPRT: after each observation, compute cumulative log-likelihood ratio. Compare against `ln(A)` and `ln(B)` derived from α, β, p0, p1.

**Refactor:** None expected.

---

## Phase 5 — Shrinking [COMPLETE]

### Step 5.1: Bytestream shrinker (`internal/verify/shrinker.go`) [x]

**Test (red):** `internal/verify/shrinker_test.go`
- `TestShrinkReducesLength` — failing bytestream of length 100 → shrunk version is shorter
- `TestShrinkPreservesFailure` — shrunk bytestream still produces a failing simulation
- `TestShrinkMinimal` — simple 2-instance, 2-capacity model → shrunk to minimal step count that triggers failure
- `TestShrinkAlreadyMinimal` — 1-step failure → shrinking returns same bytestream
- `TestShrinkMaxAttempts` — shrinking terminates within bounded attempts

**Code (green):**

```go
type ShrinkConfig struct {
    MaxAttempts int  // default: 1000
}

func Shrink(
    machine *Machine,
    failingBytes []byte,
    config SimulationConfig,
    shrinkConfig ShrinkConfig,
) []byte
```

Shrinking operations in order:
1. Delete chunks (try removing progressively smaller chunks)
2. Zero chunks (try zeroing progressively smaller chunks)
3. Reduce individual bytes (binary search toward zero)

Each attempt: apply operation → replay simulation with modified bytes → if still fails, keep; if passes, discard.

**Refactor:** None expected.

---

## Phase 6 — Orchestrator and Output [COMPLETE]

### Step 6.1: Top-level Verify function (`internal/verify/verify.go`) [x]

**Test (red):** `internal/verify/verify_test.go`
- `TestVerifyPassingModel` — model with generous capacity → all verifications pass, correct confidence
- `TestVerifyFailingModel` — model with tight capacity → at least one verification fails with minimal failure case
- `TestVerifyNoPatterns` — model with no patterned relationships → empty verifications, summary says "no behavioral patterns"
- `TestVerifyMixedRelationships` — model with one patterned + one non-patterned relationship → only patterned one verified
- `TestVerifyCustomSeed` — same seed → same result (deterministic)

**Code (green):**

```go
type VerifyConfig struct {
    TargetFailureRate float64
    Confidence        float64
    MaxStepsPerSim    int
    ShrinkAttempts    int
    Seed              int64   // 0 = random
}

func Verify(
    model *model.SystemModel,
    registry *loader.Registry,
    validation *model.ValidationResult,
    config VerifyConfig,
) (*VerificationResult, error)
```

Orchestration:
1. Compose machines from model
2. For each machine: SPRT loop — generate bytestream, simulate, update SPRT, stop when decided
3. On failure: shrink bytestream, record minimal failure
4. Collect all verifications into result

**Refactor:** None expected.

### Step 6.2: `.verified.yaml` output (`internal/verify/output.go`) [x]

**Test (red):** `internal/verify/output_test.go`
- `TestVerifiedYAMLStructure` — output has `model`, `verified_at`, `verifications`, `behavioral_findings`, `known_unknowns`, `assumptions`, `summary`
- `TestVerifiedYAMLTimestamp` — `verified_at` is valid ISO 8601
- `TestVerifiedYAMLPassingVerification` — verification with result "pass" serialized correctly
- `TestVerifiedYAMLFailingVerification` — verification with minimal failure case serialized correctly

**Code (green):**

```go
type VerifiedOutput struct {
    Model               string              `yaml:"model"`
    VerifiedAt          string              `yaml:"verified_at"`
    Verifications       []Verification      `yaml:"verifications"`
    BehavioralFindings  []model.Finding     `yaml:"behavioral_findings"`
    KnownUnknowns       []model.KnownUnknown `yaml:"known_unknowns"`
    Assumptions         []model.Assumption  `yaml:"assumptions"`
    Summary             string              `yaml:"summary"`
}

func WriteVerifiedYAML(inputPath string, modelName string, result *VerificationResult, validation *model.ValidationResult) error
```

Output path: replace `.model.yaml` with `.verified.yaml` (sibling of input).

**Refactor:** None expected.

---

## Phase 7 — CLI and MCP Integration [COMPLETE]

### Step 7.1: `verify` CLI command [x]

**Test (red):** `cmd/modelr/main_test.go`
- `TestVerifyCommandPassingModel` — model with generous capacity → exits 0, `.verified.yaml` written, output says "Accepted"
- `TestVerifyCommandFailingModel` — tight capacity model → exits 0, `.verified.yaml` written, output includes failure case
- `TestVerifyCommandNoPatterns` — model with no patterned relationships → reports no behavioral patterns
- `TestVerifyCommandFlags` — `--failure-rate 0.01 --confidence 0.95` → accepted by parser
- `TestVerifyCommandVerbose` — `--verbose` → shows definition resolution

**Code (green):**

Register `verify` command:
```go
{
    Name:      "verify",
    Usage:     "Behavioral verification via state exploration",
    ArgsUsage: "<path>",
    Flags: []cli.Flag{
        verboseFlag,
        &cli.FloatFlag{Name: "failure-rate", Value: 0.001, Usage: "Target failure rate"},
        &cli.FloatFlag{Name: "confidence", Value: 0.99, Usage: "Required confidence level"},
    },
    Action: runVerify,
}
```

`runVerify`: run pipeline → `verify.Verify(model, registry, validation, config)` → write `.verified.yaml` → print results.

**Refactor:** None expected.

### Step 7.2: Wire up `verify_model` MCP tool [x]

**Test (red):** `internal/mcp/server_test.go`
- `TestVerifyModelPassing` — call with generous model → result text says "Accepted"
- `TestVerifyModelFailing` — call with tight model → result text includes violation
- `TestVerifyModelWritesFile` — `.verified.yaml` written

**Code (green):**

Replace `verifyModelHandler` stub with real implementation that calls `verify.Verify`.

**Refactor:** None expected.

---

## Phase 8 — End-to-End Integration [COMPLETE]

### Step 8.1: End-to-end verification tests [x]

**Test (red):** `cmd/modelr/main_test.go`
- `TestEndToEndVerifyCleanModel` — generous margins → `.verified.yaml` with all "pass", zero failures
- `TestEndToEndVerifyViolation` — capacity equals instances with pooling → `.verified.yaml` with "fail", minimal failure case shows conservation violation
- `TestEndToEndVerifyMixedResults` — model with 2 relationships, one safe + one unsafe → 1 pass + 1 fail
- `TestEndToEndCheckThenVerify` — run `check` then `verify` on same model → both outputs exist, arithmetic and behavioral results consistent

**Code (green):**

Create test fixture models in `testdata/`:
- `testdata/verify-pass.model.yaml` — generous capacity (100 instances, 10000 downstream connections)
- `testdata/verify-fail.model.yaml` — tight capacity (50 instances, 50-connection pool, 50 downstream connections)
- `testdata/verify-mixed.model.yaml` — two relationships, one safe and one unsafe

These should pass with existing code.

**Refactor:** None expected.

---

## Self-Critique and Revisions

### Issues found and addressed

1. **Role inference depends on naming conventions** — The spec says roles are inferred from binding suffixes (`max_instances`, `max_connections`, `_ms`). What if a custom template uses different names? **Resolution:** The spec explicitly says "if the engine cannot infer all required roles, `modelr verify` reports an error." This is correct behavior — custom templates must follow naming conventions or they can't use behavioral patterns. The error message should explain which role is missing and what binding patterns are expected.

2. **Shared state unification correctness** — Two relationships sharing a downstream must share the same `used_connections` counter. But the counter is internal to the state machine, not a model-level concept. **Resolution:** The composer identifies shared downstreams by component name. When building machines, it creates one `SharedState` per unique downstream component. Multiple machines that share a downstream get a pointer to the same `SharedState`. The shared state key is `downstream_component_name`.

3. **Bytestream length varies per simulation** — Different bytestreams lead to different rule selections, which may produce different numbers of steps (some rules can't fire, causing no-ops). **Resolution:** Simulations run for a fixed `MaxSteps` number of byte-consuming decisions. Each step always draws bytes (even if no rules fire), keeping the bytestream consumption deterministic. Shrinking works because a shorter bytestream produces fewer steps.

4. **SPRT p1 derivation** — The spec says p1 is "derived from target" but doesn't specify how. **Resolution:** Standard practice is `p1 = 10 * p0`. For default `p0 = 0.001`, `p1 = 0.01`. This gives good discrimination between "truly safe" and "clearly broken" models.

5. **Composed simulation step ordering** — With multiple machines in a composed simulation, how are steps interleaved? **Resolution:** Each step: bytestream draws a machine index, then draws instance and rule within that machine. This produces natural interleaving across machines and is deterministic from the bytestream.

6. **Shrinking composed simulations** — Shrinking operates on one bytestream that drives the entire composed simulation. This is correct — the shrinker doesn't need to know about composition; it just shortens the bytestream and replays. **Resolution:** No change needed; the existing design handles this.

7. **The `RoleMap` stores resolved int values** — But model properties may be `float64` (YAML numbers), `int`, or `nil`. **Resolution:** `InferRoles` coerces numeric values to `int` (resource counts are inherently integers). `nil` values → error (can't verify with missing values; these are known unknowns).

8. **Max simulation count** — SPRT can theoretically run indefinitely if the true failure rate is close to the boundary. **Resolution:** Add a `MaxSimulations` field to `VerifyConfig` with default 10000. If reached without decision, report as "inconclusive."

### Ordering validation

- Phase 1 (pattern field + role inference) has no new dependencies — correct start
- Phase 2 (state machines) depends on types from Phase 1 — correct
- Phase 3 (bytestream + simulator + composer) depends on state machines from Phase 2 — correct
- Phase 4 (SPRT) is independent of Phases 2–3 but logically ordered here — correct
- Phase 5 (shrinking) depends on simulator from Phase 3 — correct
- Phase 6 (orchestrator + output) depends on all prior phases — correct
- Phase 7 (CLI + MCP) depends on Phase 6 — correct
- Phase 8 (integration) depends on all — correct final phase

---

## Estimated Step Count

| Phase | Steps | Tests |
|-------|-------|-------|
| 1. Pattern Field + Role Inference | 3 | ~10 |
| 2. State Machines | 3 | ~16 |
| 3. Bytestream + Simulation + Composition | 3 | ~14 |
| 4. SPRT | 1 | ~6 |
| 5. Shrinking | 1 | ~5 |
| 6. Orchestrator + Output | 2 | ~9 |
| 7. CLI + MCP | 2 | ~8 |
| 8. Integration | 1 | ~4 |
| **Total** | **16** | **~72** |
