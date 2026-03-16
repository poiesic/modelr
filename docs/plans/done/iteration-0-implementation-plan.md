# Iteration 0 — Implementation Plan

## Goal

Implement the core Parse → Validate → Check pipeline from the spec (sections 2–5), with embedded definitions, CLI commands (`check`, `validate`), and `.checked.yaml` output. This gives practitioners a working tool that can load a model, fill defaults, surface assumptions, and check relationship constraints.

## Scope

### In scope

| Spec Section | What |
|---|---|
| 2 | Model format — all YAML structures (components, edges, relationships, known unknowns) |
| 3 | Type system — type schemas, validation, assumptions, defaults |
| 4 | Relationships and checks — templates, expression evaluator, findings |
| 5 | Pipeline — Parse → Validate → Check, `.checked.yaml` output |
| 6.1 | CLI — `check` and `validate` commands with `--verbose` flag |
| 7 | Building blocks — embedded definitions + inline definitions (no `$MODELR_PATH`, no cache) |

### Deferred

| Feature | Why |
|---|---|
| `$MODELR_PATH` resolution + cache (7.2, 7.6) | Embedded + inline is sufficient for iteration 0 |
| `definitions` command (6.1) | Depends on full loader |
| Behavioral verification (8) | Complex subsystem, iteration 1 |
| Visualization (10) | Nice-to-have, iteration 1 |
| MCP server (6.2) | Integration layer, iteration 1 |
| Code generation (9) | Explicitly "to be designed" in spec |

## Dependencies

```
go get gopkg.in/yaml.v3
go get github.com/urfave/cli/v3
go get github.com/stretchr/testify
```

`mcp-go` is deferred (MCP server not in scope).

## File Layout

```
embed/
  definitions.yaml          # Standard node + relationship definitions
  embed.go                  # go:embed variable
internal/
  model/
    types.go                # SystemModel, Component, Edge, etc.
    parser.go               # YAML → SystemModel
    parser_test.go
    validator.go            # Fill defaults, assumptions, known unknowns
    validator_test.go
  check/
    expr.go                 # Tokenizer + recursive descent parser + evaluator
    expr_test.go
    checker.go              # Resolve bindings, evaluate checks, produce findings
    checker_test.go
  loader/
    loader.go               # Load definitions from inline + embedded
    loader_test.go
cmd/modelr/
  main.go                   # CLI entry point
  main_test.go              # Integration tests
```

---

## Phase 1 — Types and Embedded Definitions [COMPLETE]

No behavioral code yet — just data structures and the YAML definition files that everything else resolves against.

### Step 1.1: Core model types (`internal/model/types.go`) [x]

**Test (red):** `internal/model/types_test.go`
- `TestUncertaintyCategoryIsValid` — verify all 6 categories pass, unknown string fails
- `TestOperationIsValid` — verify `read`, `write`, `read_write` pass, unknown string fails

**Code (green):**
Define all types:

```go
type SystemModel struct {
    Version       string
    Name          string
    Description   string
    Components    []Component
    Edges         []Edge
    Relationships []Relationship
    KnownUnknowns []KnownUnknown
}

type Component struct {
    Name        string
    Type        string
    Description string
    Properties  map[string]any  // number, string, or nil
}

type Edge struct {
    Source      string
    Target      string
    Operation   string  // read | write | read_write
    Description string
    Properties  map[string]any
}

type Relationship struct {
    Template   string
    Upstream   string
    Downstream string
}

type KnownUnknown struct {
    ID          string
    Component   string  // optional
    Edge        string  // optional
    Category    string  // UncertaintyCategory
    Description string
    Impact      string
}

type Assumption struct {
    Property  string
    Component string
    Edge      string
    Value     any
    Source    string
}

type Finding struct {
    Severity     string  // "error" or "warning"
    Relationship string
    Upstream     string
    Downstream   string
    Description  string
    Suggestion   string
    Kind         string  // "arithmetic" or "behavioral"
}

type CheckResult struct {
    Findings      []Finding
    KnownUnknowns []KnownUnknown
    Assumptions   []Assumption
    Warnings      []string
    Summary       string
}
```

Also define `UncertaintyCategory` constants and `Operation` constants with validation helpers.

**Refactor:** None expected.

### Step 1.2: Definition types (`internal/loader/types.go`) [x]

**Test (red):** `internal/loader/loader_test.go`
- `TestPropertySchemaHasExpectedFields` — construct a PropertySchema, verify fields accessible

**Code (green):**

```go
type NodeDef struct {
    Kind        string                    `yaml:"kind"`
    Name        string                    `yaml:"name"`
    Description string                    `yaml:"description"`
    Properties  map[string]PropertySchema `yaml:"properties"`
}

type PropertySchema struct {
    Type        string `yaml:"type"`
    Unit        string `yaml:"unit"`
    Description string `yaml:"description"`
    Default     any    `yaml:"default"`
}

type CheckDef struct {
    Name       string `yaml:"name"`
    Expression string `yaml:"expression"`
    Violation  string `yaml:"violation"`
}

type RelationshipDef struct {
    Kind        string            `yaml:"kind"`
    Name        string            `yaml:"name"`
    Description string            `yaml:"description"`
    Resolve     map[string]string `yaml:"resolve"`
    Checks      []CheckDef        `yaml:"checks"`
}
```

**Refactor:** None expected.

### Step 1.3: Embedded definitions (`embed/`) [x]

**Test (red):** `internal/loader/loader_test.go`
- `TestLoadEmbeddedDefinitions` — load embedded YAML, verify 3 node defs (`server`, `datastore`, `queue`) and 2 relationship defs (`capacity_chain`, `pooled_capacity_chain`)
- `TestServerNodeDefProperties` — verify `max_connections`, `min_instances` (default 1), `max_instances` (default 1)
- `TestDatastoreNodeDefProperties` — verify `max_connections`, `max_ops_per_sec`, `conn_establish_ms` (default 20)
- `TestQueueNodeDefProperties` — verify `max_depth`, `max_throughput`
- `TestCapacityChainResolveBindings` — verify 5 variable bindings match spec
- `TestPooledCapacityChainResolveBindings` — verify 7 variable bindings match spec
- `TestPooledCapacityChainChecks` — verify 2 checks with correct expressions

**Code (green):**
- Create `embed/definitions.yaml` with all standard node + relationship definitions as multi-document YAML
- Create `embed/embed.go` with `//go:embed definitions.yaml` exposing the raw bytes
- Implement `LoadEmbedded() ([]NodeDef, []RelationshipDef, error)` in `internal/loader/loader.go` — parses the embedded YAML multi-document stream, separates by `kind`

**Refactor:** None expected.

---

## Phase 2 — Parser [COMPLETE]

### Step 2.1: Parse minimal valid model (`internal/model/parser.go`) [x]

**Test (red):** `internal/model/parser_test.go`
- `TestParseMinimalModel` — YAML with version, name, description, 1 component, empty edges → returns `SystemModel` with correct fields
- `TestParseComponentProperties` — component with numeric and string properties → `Properties` map populated correctly
- `TestParseNullProperty` — property explicitly set to `null` → value is `nil` in map

**Code (green):**
- Implement `Parse(reader io.Reader) (*SystemModel, error)` — uses `yaml.v3` decoder to unmarshal, handles multi-document (separates inline definitions from the model document)
- YAML struct tags on all model types

**Refactor:** Extract YAML-specific structs if needed (separate from domain types).

### Step 2.2: Parse edges and relationships [x]

**Test (red):**
- `TestParseEdges` — edges with source, target, operation, description, properties → correct `Edge` structs
- `TestParseRelationships` — relationships with template, upstream, downstream → correct `Relationship` structs
- `TestParseKnownUnknowns` — known unknowns with all fields → correct `KnownUnknown` structs

**Code (green):**
Extend `Parse` to handle all top-level fields.

**Refactor:** None expected.

### Step 2.3: Structural validation errors [x]

**Test (red):**
- `TestParseMissingVersion` — error mentions "version"
- `TestParseMissingName` — error mentions "name"
- `TestParseMissingDescription` — error mentions "description"
- `TestParseMissingComponents` — error mentions "components"
- `TestParseEmptyComponents` — empty component list → error
- `TestParseMissingEdges` — error mentions "edges" (empty edges is OK, missing is not)
- `TestParseMissingComponentName` — component without name → error
- `TestParseMissingComponentType` — component without type → error
- `TestParseMissingComponentDescription` — component without description → error
- `TestParseMissingEdgeSource` — edge without source → error
- `TestParseMissingEdgeTarget` — edge without target → error
- `TestParseMissingEdgeOperation` — edge without operation → error
- `TestParseMissingEdgeDescription` — edge without description → error
- `TestParseInvalidOperation` — operation not in {read, write, read_write} → error

**Code (green):**
Add validation in `Parse` after unmarshaling — check required fields, valid enums.

**Refactor:** Extract a `validate` helper within the parser for structural checks.

### Step 2.4: Uniqueness and reference validation [x]

**Test (red):**
- `TestParseDuplicateComponentNames` — two components with same name → error
- `TestParseEdgeUnknownSource` — edge source not in components → error
- `TestParseEdgeUnknownTarget` — edge target not in components → error
- `TestParseRelationshipUnknownUpstream` — upstream not in components → error
- `TestParseRelationshipUnknownDownstream` — downstream not in components → error
- `TestParseRejectsPoolSize` — edge with `pool_size` property → error mentioning `min_pool_size` and `max_pool_size`

**Code (green):**
Build a component name set after parsing. Validate edges and relationships reference existing components. Check for `pool_size` in edge properties.

**Refactor:** None expected.

### Step 2.5: Inline definitions [x]

**Test (red):**
- `TestParseInlineNodeDefinition` — model file with `---` separated node definition document + model document → returns model + extracts inline node def
- `TestParseInlineRelationshipDefinition` — model file with inline relationship def → extracted correctly
- `TestParseMultipleInlineDefinitions` — multiple inline defs in one file → all extracted

**Code (green):**
- Modify `Parse` to return `(*SystemModel, []NodeDef, []RelationshipDef, error)` — or introduce a `ParseResult` struct
- Use `yaml.Decoder` to read multiple documents, dispatch by `kind` field

**Refactor:** Introduce `ParseResult` struct:
```go
type ParseResult struct {
    Model         *SystemModel
    InlineNodes   []NodeDef
    InlineRels    []RelationshipDef
}
```

---

## Phase 3 — Definition Loader [COMPLETE]

### Step 3.1: Definition registry (`internal/loader/loader.go`) [x]

**Test (red):**
- `TestRegistryLookupNode` — register a node def, look up by name → found
- `TestRegistryLookupRelationship` — register a rel def, look up by name → found
- `TestRegistryNodeNotFound` — look up missing name → not found
- `TestRegistryFirstWins` — register two node defs with same name → first one returned

**Code (green):**
```go
type Registry struct {
    nodes map[string]NodeDef
    rels  map[string]RelationshipDef
}

func NewRegistry() *Registry
func (r *Registry) AddNode(def NodeDef) bool        // returns false if already exists
func (r *Registry) AddRelationship(def RelationshipDef) bool
func (r *Registry) LookupNode(name string) (NodeDef, bool)
func (r *Registry) LookupRelationship(name string) (RelationshipDef, bool)
```

**Refactor:** None expected.

### Step 3.2: Build registry with resolution order [x]

**Test (red):**
- `TestBuildRegistryEmbeddedOnly` — no inline defs → registry has all 3 embedded nodes, 2 embedded rels
- `TestBuildRegistryInlineShadowsEmbedded` — inline node def named "server" → inline version wins, embedded server skipped
- `TestBuildRegistryInlineRelShadowsEmbedded` — inline relationship shadows embedded

**Code (green):**
```go
func BuildRegistry(inline []NodeDef, inlineRels []RelationshipDef) (*Registry, error)
```
Resolution order: inline first, then embedded.

**Refactor:** None expected.

---

## Phase 4 — Validator [COMPLETE]

### Step 4.1: Fill defaults from type schema (`internal/model/validator.go`) [x]

**Test (red):** `internal/model/validator_test.go`
- `TestValidateFillsDefaultMinInstances` — server component with no `min_instances` → property filled to 1, assumption recorded with source "server type schema"
- `TestValidateFillsDefaultMaxInstances` — similar for `max_instances`
- `TestValidateFillsDefaultConnEstablishMs` — datastore with no `conn_establish_ms` → filled to 20
- `TestValidateExplicitValueNotOverridden` — component with explicit `min_instances: 4` → value preserved, no assumption

**Code (green):**
```go
type ValidationResult struct {
    Model         *SystemModel  // model with defaults filled in
    Assumptions   []Assumption
    KnownUnknowns []KnownUnknown
    Warnings      []string
}

func Validate(model *SystemModel, registry *loader.Registry) (*ValidationResult, error)
```
For each component, look up type schema. For each schema property not present in the component, apply default or generate known unknown.

**Refactor:** None expected.

### Step 4.2: Generate known unknowns for missing properties [x]

**Test (red):**
- `TestValidateMissingPropertyNoDefault` — server with no `max_connections` (no default in schema) → property set to nil, known unknown generated with category `unstated_constraint`
- `TestValidateKnownUnknownHasComponentRef` — known unknown references the component name
- `TestValidateKnownUnknownDescription` — description mentions the property name

**Code (green):**
Extend `Validate` to generate `KnownUnknown` entries for missing properties without defaults.

**Refactor:** None expected.

### Step 4.3: Unknown component types [x]

**Test (red):**
- `TestValidateUnknownTypeWarning` — component with type "custom_thing" (not in registry) → warning generated, not an error
- `TestValidateUnknownTypePropertiesUntouched` — properties left as-is for unknown types

**Code (green):**
If type schema not found, append warning, skip property validation for that component.

**Refactor:** None expected.

### Step 4.4: Relationship binding validation [x]

**Test (red):**
- `TestValidateRelationshipBindings` — relationship with all bindings resolvable → no warnings
- `TestValidateRelationshipMissingEdge` — relationship where no edge exists between upstream/downstream → warning
- `TestValidateRelationshipMissingProperty` — binding references property not on component → value set to nil, known unknown generated
- `TestValidateRelationshipMismatchedSchema` — `pooled_capacity_chain` applied to `queue` downstream (no `conn_establish_ms`) → warning about property not in type schema

**Code (green):**
For each relationship in the model:
1. Look up template in registry
2. For each resolve binding, parse `scope.property`
3. If scope is `edge`, find the edge between upstream/downstream
4. Check that referenced properties exist
5. Generate warnings and known unknowns as needed

**Refactor:** Extract edge-finding helper: `findEdge(model, source, target) *Edge`.

### Step 4.5: Merge known unknowns [x]

**Test (red):**
- `TestValidateMergesModelKnownUnknowns` — model has explicit known unknowns + validator generates additional ones → all merged in result

**Code (green):**
Merge `model.KnownUnknowns` with validator-generated known unknowns in the result.

**Refactor:** None expected.

---

## Phase 5 — Expression Evaluator [COMPLETE]

### Step 5.1: Tokenizer (`internal/check/expr.go`) [x]

**Test (red):** `internal/check/expr_test.go`
- `TestTokenizeNumber` — `"42"` → `[NUMBER(42)]`
- `TestTokenizeFloat` — `"3.14"` → `[NUMBER(3.14)]`
- `TestTokenizeIdentifier` — `"upstream_rate"` → `[IDENT("upstream_rate")]`
- `TestTokenizeOperators` — `"+ - * / <= >= < >"` → correct token sequence
- `TestTokenizeParens` — `"(a + b)"` → correct tokens including LPAREN, RPAREN
- `TestTokenizeComma` — `"max(a, b)"` → tokens include COMMA
- `TestTokenizeComplexExpression` — `"upstream_rate * instances * operation_cost / 1000 <= downstream_capacity * max(downstream_instances, 1)"` → correct full token sequence

**Code (green):**
Implement a simple tokenizer (lexer) that scans an expression string and produces a slice of tokens. Token types: `NUMBER`, `IDENT`, `PLUS`, `MINUS`, `STAR`, `SLASH`, `LTE`, `GTE`, `LT`, `GT`, `LPAREN`, `RPAREN`, `COMMA`, `EOF`.

**Refactor:** None expected.

### Step 5.2: Parser — arithmetic expressions [x]

**Test (red):**
- `TestParseNumber` — `"42"` → NumberLiteral node
- `TestParseIdentifier` — `"x"` → Identifier node
- `TestParseAddition` — `"a + b"` → BinaryOp(+, Ident(a), Ident(b))
- `TestParseMultiplication` — `"a * b"` → BinaryOp(*, ...)
- `TestParsePrecedence` — `"a + b * c"` → BinaryOp(+, a, BinaryOp(*, b, c))
- `TestParseParentheses` — `"(a + b) * c"` → BinaryOp(*, BinaryOp(+, a, b), c)
- `TestParseUnaryNegation` — `"-a"` → UnaryOp(-, a)
- `TestParseNestedParens` — `"((a))"` → Ident(a)

**Code (green):**
Implement recursive descent parser producing an AST. Node types: `NumberLiteral`, `Identifier`, `BinaryOp`, `UnaryOp`, `FuncCall`.

**Refactor:** None expected.

### Step 5.3: Parser — comparisons and functions [x]

**Test (red):**
- `TestParseComparison` — `"a <= b"` → Comparison(<=, a, b)
- `TestParseComparisonWithArithmetic` — `"a * b <= c + d"` → Comparison(<=, BinaryOp(*), BinaryOp(+))
- `TestParseFunctionMax` — `"max(a, b)"` → FuncCall("max", [a, b])
- `TestParseFunctionMin` — `"min(a, b)"` → FuncCall("min", [a, b])
- `TestParseFunctionInExpression` — `"max(a, 1) * b"` → BinaryOp(*, FuncCall, b)
- `TestParseFullCapacityChainExpr` — `"upstream_rate * instances * operation_cost / 1000 <= downstream_capacity * max(downstream_instances, 1)"` → parses without error, correct structure

**Code (green):**
Extend parser with comparison handling at the top level and function call parsing in `primary`.

**Refactor:** None expected.

### Step 5.4: Evaluator — numeric expressions [x]

**Test (red):**
- `TestEvalNumber` — `"42"`, no vars → 42.0
- `TestEvalVariable` — `"x"`, vars `{x: 10}` → 10.0
- `TestEvalAddition` — `"a + b"`, vars `{a: 3, b: 4}` → 7.0
- `TestEvalSubtraction` — `"a - b"`, vars `{a: 10, b: 3}` → 7.0
- `TestEvalMultiplication` — `"a * b"`, vars `{a: 3, b: 4}` → 12.0
- `TestEvalDivision` — `"a / b"`, vars `{a: 12, b: 4}` → 3.0
- `TestEvalDivisionByZero` — `"a / b"`, vars `{a: 12, b: 0}` → error
- `TestEvalPrecedence` — `"a + b * c"`, vars `{a: 1, b: 2, c: 3}` → 7.0
- `TestEvalUnaryNegation` — `"-a"`, vars `{a: 5}` → -5.0
- `TestEvalNestedArithmetic` — `"(a + b) * c / d"` → correct result

**Code (green):**
```go
// Returns *float64 (nil for null) or error
func EvaluateNumeric(expr string, vars map[string]any) (*float64, error)
```
Walk the AST, resolve identifiers from vars map, apply arithmetic.

**Refactor:** None expected.

### Step 5.5: Evaluator — null propagation [x]

**Test (red):**
- `TestEvalNullVariable` — `"x"`, vars `{x: nil}` → nil (not error)
- `TestEvalNullPlusNumber` — `"a + b"`, vars `{a: nil, b: 5}` → nil
- `TestEvalNullTimesNumber` — `"a * b"`, vars `{a: nil, b: 5}` → nil
- `TestEvalNullDivNumber` — `"a / b"`, vars `{a: nil, b: 5}` → nil
- `TestEvalNumberDivNull` — `"a / b"`, vars `{a: 5, b: nil}` → nil (not div-by-zero error)
- `TestEvalUndefinedVariable` — `"x"`, vars `{}` → nil (missing variable is null)

**Code (green):**
Modify evaluator to use `*float64` throughout. Nil propagates through all arithmetic. Missing variables resolve to nil.

**Refactor:** None expected.

### Step 5.6: Evaluator — comparisons [x]

**Test (red):**
- `TestEvalComparisonTrue` — `"a <= b"`, vars `{a: 5, b: 10}` → true
- `TestEvalComparisonFalse` — `"a <= b"`, vars `{a: 15, b: 10}` → false
- `TestEvalComparisonNullLeft` — `"a <= b"`, vars `{a: nil, b: 10}` → nil (indeterminate)
- `TestEvalComparisonNullRight` — `"a <= b"`, vars `{a: 5, b: nil}` → nil
- `TestEvalAllComparisonOps` — test `<`, `>`, `<=`, `>=` each with true/false cases

**Code (green):**
```go
// Returns *bool (nil for indeterminate) or error
func EvaluateComparison(expr string, vars map[string]any) (*bool, error)
```

**Refactor:** Share AST parsing between `EvaluateNumeric` and `EvaluateComparison` — parse once, evaluate differently.

### Step 5.7: Evaluator — functions [x]

**Test (red):**
- `TestEvalMax` — `"max(a, b)"`, vars `{a: 3, b: 7}` → 7.0
- `TestEvalMaxNullFirst` — `"max(a, b)"`, vars `{a: nil, b: 7}` → 7.0 (spec: max(null, x) = x)
- `TestEvalMaxNullSecond` — `"max(a, b)"`, vars `{a: 3, b: nil}` → 3.0
- `TestEvalMaxBothNull` — `"max(a, b)"`, vars `{a: nil, b: nil}` → nil
- `TestEvalMin` — `"min(a, b)"`, vars `{a: 3, b: 7}` → 3.0
- `TestEvalMinNullFirst` — `"min(a, b)"`, vars `{a: nil, b: 7}` → nil (spec: min(null, x) = null)
- `TestEvalMinNullSecond` — `"min(a, b)"`, vars `{a: 3, b: nil}` → nil
- `TestEvalUnknownFunction` — `"foo(a)"` → error

**Code (green):**
Implement function evaluation in the AST walker. `max` and `min` have special null semantics per spec.

**Refactor:** None expected.

### Step 5.8: Evaluator — integration with full spec expressions [x]

**Test (red):**
- `TestEvalCapacityChainThroughputPass` — `"upstream_rate * instances * operation_cost / 1000 <= downstream_capacity * max(downstream_instances, 1)"` with safe values → true
- `TestEvalCapacityChainThroughputFail` — same expression with oversubscribed values → false
- `TestEvalPooledConnectionLimitPass` — `"max_pool_size * instances <= downstream_capacity"` with safe values → true
- `TestEvalPooledConnectionLimitFail` — oversubscribed → false
- `TestEvalPooledThroughputPass` — `"max_pool_size * instances * 1000 / operation_cost <= downstream_max_ops"` with safe values → true

**Code (green):**
Should pass with no new code if Steps 5.1–5.7 are complete. These tests are integration-level validation.

**Refactor:** None expected.

---

## Phase 6 — Constraint Checker [COMPLETE]

### Step 6.1: Resolve relationship variables (`internal/check/checker.go`) [x]

**Test (red):** `internal/check/checker_test.go`
- `TestResolveUpstreamVar` — binding `upstream.max_connections`, upstream component has the property → resolves to the value
- `TestResolveDownstreamVar` — binding `downstream.max_connections` → resolves
- `TestResolveEdgeVar` — binding `edge.avg_operation_ms`, edge has the property → resolves
- `TestResolveMissingProperty` — binding references property not on component → resolves to nil
- `TestResolveStringProperty` — binding references a string-valued property → string value returned (checker will skip)
- `TestResolveMissingEdge` — no edge between upstream/downstream → nil for edge-scoped bindings

**Code (green):**
```go
func resolveVariables(
    template *RelationshipDef,
    upstream *Component,
    downstream *Component,
    edge *Edge,  // may be nil
) map[string]any
```

**Refactor:** None expected.

### Step 6.2: Check execution [x]

**Test (red):**
- `TestCheckPassingRelationship` — model where capacity_chain check passes → no findings
- `TestCheckFailingRelationship` — model where throughput exceeds capacity → finding with severity "error", correct upstream/downstream names, description includes expression and resolved values
- `TestCheckIndeterminateResult` — null variable makes comparison indeterminate → known unknown generated, no finding
- `TestCheckSkipsStringVariables` — binding resolves to string → check skipped entirely (spec 4.4 step 3)
- `TestCheckMultipleRelationships` — model with 2 relationships → both checked
- `TestCheckMultipleChecksPerTemplate` — pooled_capacity_chain has 2 checks → both evaluated

**Code (green):**
```go
func Check(model *SystemModel, registry *loader.Registry) (*CheckResult, error)
```

For each relationship:
1. Look up template
2. Resolve variables
3. Skip if any string values
4. For each check, evaluate expression
5. nil result → known unknown; false → finding

**Refactor:** None expected.

### Step 6.3: Finding descriptions and suggestions [x]

**Test (red):**
- `TestFindingIncludesResolvedValues` — finding description shows what values were plugged in (e.g., "100 * 50 * 10 / 1000 = 50 > 40")
- `TestFindingSuggestion` — finding includes remediation text from the check's violation field

**Code (green):**
Format the finding description to include the expression with values substituted, and the computed left/right side values.

**Refactor:** None expected.

### Step 6.4: Check result summary [x]

**Test (red):**
- `TestCheckResultSummaryClean` — no findings → summary "All relationship constraints satisfied."
- `TestCheckResultSummaryViolations` — 2 findings → summary "Found 2 relationship violation(s)."
- `TestCheckResultMergesKnownUnknowns` — known unknowns from validation + checker both present

**Code (green):**
Set `CheckResult.Summary` based on finding count.

**Refactor:** None expected.

---

## Phase 7 — CLI Integration [COMPLETE]

### Step 7.1: CLI skeleton (`cmd/modelr/main.go`) [x]

**Test (red):** `cmd/modelr/main_test.go`
- `TestCLINoArgs` — running with no args → shows help, exits cleanly
- `TestCLIUnknownCommand` — `modelr foo` → error

**Code (green):**
Set up `urfave/cli/v3` app with name, description, version "0.2". Register `check` and `validate` as subcommands (stubs for now).

**Refactor:** None expected.

### Step 7.2: `validate` command [x]

**Test (red):**
- `TestValidateCommandSuccess` — valid model file → exits 0, prints assumptions and warnings to stdout
- `TestValidateCommandFileNotFound` — nonexistent path → error message, non-zero exit
- `TestValidateCommandInvalidModel` — model with parse errors → error message listing issues
- `TestValidateCommandVerbose` — `--verbose` flag → shows definition resolution sources

**Code (green):**
Implement validate command: read file → `Parse` → `BuildRegistry` (inline + embedded) → `Validate` → print results.

**Refactor:** None expected.

### Step 7.3: `check` command [x]

**Test (red):**
- `TestCheckCommandSuccess` — valid model → exits 0, writes `.checked.yaml` sibling file
- `TestCheckCommandFindings` — model with violations → exits 0, findings printed, `.checked.yaml` written
- `TestCheckCommandVerbose` — `--verbose` → shows resolution details

**Code (green):**
Implement check command: read file → `Parse` → `BuildRegistry` → `Validate` → `Check` → write `.checked.yaml` → print summary.

**Refactor:** Extract shared pipeline logic (parse + build registry + validate) into a helper used by both commands.

### Step 7.4: `.checked.yaml` output [x]

**Test (red):**
- `TestCheckedYAMLStructure` — output file has all required fields: `model`, `checked_at`, `known_unknowns`, `findings`, `assumptions`, `warnings`, `summary`
- `TestCheckedYAMLTimestamp` — `checked_at` is valid ISO 8601
- `TestCheckedYAMLFindings` — findings correctly serialized
- `TestCheckedYAMLAssumptions` — assumptions correctly serialized

**Code (green):**
Define output struct with YAML tags, marshal and write to `<input>.checked.yaml` (replace `.model.yaml` extension).

**Refactor:** Move output formatting into a shared `internal/model/output.go` or similar.

---

## Phase 8 — End-to-End Integration [COMPLETE]

### Step 8.1: Full pipeline integration test [x]

**Test (red):** `cmd/modelr/main_test.go`
- `TestEndToEndCleanModel` — model with safe capacity values → `check` produces `.checked.yaml` with no findings, correct assumptions
- `TestEndToEndViolation` — model with oversubscribed connections → findings in output
- `TestEndToEndWithInlineDefinitions` — model file containing inline node def that shadows embedded → inline definition used
- `TestEndToEndNullPropagation` — model with missing properties → indeterminate results become known unknowns, not findings

**Code (green):**
These should pass with existing code. Create test fixture YAML files in `testdata/` directories.

**Refactor:** Create `testdata/` directories under relevant packages for fixture models.

---

## Self-Critique and Revisions

### Issues found and addressed

1. **Parser return type change** — Step 2.5 changes `Parse` return signature to include inline definitions. Steps 2.1–2.4 build against the original signature. **Resolution:** Steps 2.1–2.4 tests don't depend on inline definitions, so the signature change in 2.5 is additive. Tests from 2.1–2.4 still pass (they just ignore the extra return values via `ParseResult`). Introduced `ParseResult` in step 2.5 rather than changing the raw function signature.

2. **Circular dependency risk** — `internal/model` depends on `internal/loader` for the `Registry` type (in `Validate`). `internal/loader` defines types used in model validation. **Resolution:** This is one-directional: model → loader is fine. Loader doesn't import model. No circular dependency.

3. **Edge lookup for relationships** — The relationship's `upstream`/`downstream` are component names, but the edge is identified by `source`/`target`. Need to find the edge connecting the two components. **Resolution:** Addressed in Step 4.4 with `findEdge` helper. Also used in Step 6.1.

4. **Variable `vars` type** — Expression evaluator uses `map[string]any` where values can be `float64`, `string`, or `nil`. The resolver must coerce YAML-parsed values (which may be `int`) to `float64`. **Resolution:** The resolver in Step 6.1 should handle `int` → `float64` coercion. Added a note that `resolveVariables` normalizes numeric types.

5. **Test fixture strategy** — Multiple phases need test YAML models. **Resolution:** Each package creates its own `testdata/` with focused fixtures. Integration tests in `cmd/modelr/` use full model files.

6. **Missing: `go get` dependencies** — Need to run dependency installation before any code compiles. **Resolution:** Added as a prerequisite step before Phase 1.

### Ordering validation

- Phase 1 (types + embedded defs) has no dependencies — correct start
- Phase 2 (parser) depends on types from Phase 1 — correct
- Phase 3 (loader registry) depends on types from Phase 1 — correct
- Phase 4 (validator) depends on parser (Phase 2) + loader (Phase 3) — correct
- Phase 5 (expression evaluator) has no dependency on Phases 2–4, but is logically needed by Phase 6 — correct ordering
- Phase 6 (checker) depends on evaluator (Phase 5) + validator output (Phase 4) — correct
- Phase 7 (CLI) depends on all prior phases — correct
- Phase 8 (integration) depends on everything — correct final step

---

## Prerequisites

Before beginning Phase 1:

```bash
go get gopkg.in/yaml.v3
go get github.com/urfave/cli/v3
go get github.com/stretchr/testify
```

## Estimated Step Count

| Phase | Steps | Tests |
|-------|-------|-------|
| 1. Types + Embedded | 3 | ~12 |
| 2. Parser | 5 | ~22 |
| 3. Loader | 2 | ~7 |
| 4. Validator | 5 | ~12 |
| 5. Expression Evaluator | 8 | ~35 |
| 6. Constraint Checker | 4 | ~13 |
| 7. CLI | 4 | ~10 |
| 8. Integration | 1 | ~4 |
| **Total** | **32** | **~115** |
