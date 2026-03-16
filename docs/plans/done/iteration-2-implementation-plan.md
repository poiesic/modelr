# Iteration 2 — Implementation Plan

## Goal

Add visualization (DOT/SVG), an MCP server exposing modelr tools over stdio, a `modelr init` command that scaffolds Claude Code integration (skills + mcp.json), and the `modelr visualize` CLI command. After this iteration, practitioners can use Claude Code with modelr as an MCP server to generate, check, and visualize system models.

## Prerequisites

Iterations 0 and 1 are complete:
- Full Parse → Validate → Check pipeline with `.checked.yaml` output
- `$MODELR_PATH` resolution, definition cache, `definitions` command
- Registry with source tracking, shadow events, verbose output

## Scope

### In scope

| Spec Section | What |
|---|---|
| 10 | DOT generation — node styling by type, edge styling by status, labels with properties |
| 10.2 | SVG rendering if graphviz installed; `.dot` fallback otherwise |
| 6.1 | `modelr visualize <path>` CLI command |
| 6.2 | `modelr mcp` — MCP server over stdio with 5 tools |
| — | `modelr init` — scaffold `.claude/skills/` and `.claude/mcp.json` |

### Deferred

| Feature | Why |
|---|---|
| Behavioral verification (8) | Complex subsystem, iteration 3+ |
| `verify_model` MCP tool | Depends on verification engine — registers as stub returning "not yet implemented" |

## New Dependencies

```
go get github.com/mark3labs/mcp-go
```

## File Layout

```
internal/viz/
  dot.go              # DOT generation from checked model
  dot_test.go
  render.go           # SVG/PNG rendering via graphviz
  render_test.go
internal/mcp/
  server.go           # MCP server setup and tool registration
  server_test.go
internal/init/
  scaffold.go         # File generation for init command
  scaffold_test.go
  skills/
    model.md          # Embedded skill content
    outage-report.md  # Embedded skill content
cmd/modelr/
  main.go             # MODIFIED: add visualize, mcp, init commands
  main_test.go        # MODIFIED: new command tests
```

---

## Phase 1 — DOT Generation [COMPLETE]

### Step 1.1: Basic DOT structure (`internal/viz/dot.go`) [x]

**Test (red):** `internal/viz/dot_test.go`
- `TestDOTMinimalModel` — model with 1 component, no edges → valid DOT with `digraph`, graph attributes, 1 node
- `TestDOTNodeHasLabel` — node label includes component name and type
- `TestDOTOutputIsParseable` — output starts with `digraph` and ends with `}`

**Code (green):**

```go
type DOTInput struct {
    Model         *model.SystemModel
    CheckResult   *model.CheckResult
    ValidationResult *model.ValidationResult
}

func GenerateDOT(input *DOTInput) string
```

Generate minimal DOT: `digraph "name" { ... }` with graph attributes (`rankdir=LR`, `fontname`), one node per component.

**Refactor:** None expected.

### Step 1.2: Node styling by component type [x]

**Test (red):**
- `TestDOTServerNodeStyle` — server component → `shape=box`, `fillcolor="#4A90D9"`
- `TestDOTDatastoreNodeStyle` — datastore → `shape=cylinder`, `fillcolor="#50B83C"`
- `TestDOTQueueNodeStyle` — queue → `shape=parallelogram`, `fillcolor="#E6A23C"`
- `TestDOTUnknownTypeNodeStyle` — unknown type → `shape=box`, `fillcolor="#999999"`
- `TestDOTNodeHasProperties` — node label includes non-null property values

**Code (green):**

Map component types to shapes and colors per spec section 10.1. Build HTML-like labels with component name, type, and properties.

**Refactor:** Extract a `nodeStyle` helper that returns shape/color for a component type.

### Step 1.3: Edge rendering [x]

**Test (red):**
- `TestDOTEdgeBasic` — edge between two components → DOT edge with label showing operation
- `TestDOTEdgeProperties` — edge with non-null properties → properties shown in label
- `TestDOTEdgeClean` — edge with no findings or known unknowns → `color=black`, `penwidth=1`, solid
- `TestDOTEdgeFinding` — edge between components that have a finding → `color=red`, `penwidth=2.5`, solid, warning symbol in label
- `TestDOTEdgeKnownUnknown` — edge with known unknown but no finding → `color=orange`, `penwidth=1.5`, `style=dashed`

**Code (green):**

For each edge, determine status by checking if any findings or known unknowns reference the upstream/downstream pair. Apply styling per spec.

**Refactor:** Extract `edgeStatus` helper that returns style attributes given findings/known unknowns.

### Step 1.4: Component borders for known unknowns [x]

**Test (red):**
- `TestDOTComponentWithKnownUnknown` — component referenced by a known unknown → `penwidth=2`, `color="#E6A23C"` (orange border)
- `TestDOTComponentClean` — component with no known unknowns → default border

**Code (green):**

When building node attributes, check if any known unknown references the component. If so, add orange border.

**Refactor:** None expected.

---

## Phase 2 — SVG Rendering [COMPLETE]

### Step 2.1: Graphviz detection and rendering (`internal/viz/render.go`) [x]

**Test (red):** `internal/viz/render_test.go`
- `TestGraphvizAvailable` — if `dot` is in PATH → returns true
- `TestGraphvizNotAvailable` — with empty PATH override → returns false
- `TestRenderDOTToSVG` — valid DOT string + graphviz available → returns SVG bytes, no error (skip if graphviz not installed)
- `TestRenderDOTToSVGNoGraphviz` — graphviz not available → returns error with install suggestion

**Code (green):**

```go
func GraphvizAvailable() bool
func RenderDOT(dot string, format string) ([]byte, error)  // format: "svg" or "png"
```

`GraphvizAvailable` checks for `dot` binary via `exec.LookPath`. `RenderDOT` pipes DOT to `dot -T<format>` and captures stdout.

**Refactor:** None expected.

### Step 2.2: `visualize` CLI command (`cmd/modelr/main.go`) [x]

**Test (red):** `cmd/modelr/main_test.go`
- `TestVisualizeCommandWritesDOT` — valid model → `.dot` file written as sibling of input
- `TestVisualizeCommandContentValid` — `.dot` file contains `digraph`, component names, edge connections
- `TestVisualizeCommandSVGWhenAvailable` — graphviz installed → `.svg` also written (skip if not installed)
- `TestVisualizeCommandVerbose` — `--verbose` → shows definition resolution

**Code (green):**

Register `visualize` command:
```go
{
    Name:      "visualize",
    Usage:     "Check and generate DOT/SVG visualization",
    ArgsUsage: "<path>",
    Flags:     []cli.Flag{verboseFlag},
    Action:    runVisualize,
}
```

`runVisualize`: run full pipeline (parse → validate → check) → `GenerateDOT` → write `.dot` file → if graphviz available, render to `.svg`.

**Refactor:** None expected.

---

## Phase 3 — MCP Server [COMPLETE]

### Step 3.1: MCP server setup (`internal/mcp/server.go`) [x]

**Test (red):** `internal/mcp/server_test.go`
- `TestNewServerHasTools` — create server → has 5 registered tools
- `TestToolNames` — tool names are `list_definitions`, `check_model`, `validate_model`, `visualize_model`, `verify_model`

**Code (green):**

```go
func NewServer(env EnvConfig) *server.MCPServer

type EnvConfig struct {
    ModelrPath string
    HomeDir    string
}
```

Create MCP server with `server.NewMCPServer("modelr", "0.2", ...)`. Register 5 tools with descriptions and input schemas. `verify_model` is a stub.

**Refactor:** None expected.

### Step 3.2: `list_definitions` tool handler [x]

**Test (red):** `internal/mcp/server_test.go`
- `TestListDefinitionsReturnsEmbedded` — call tool with no path defs → result text contains server, datastore, queue, capacity_chain, pooled_capacity_chain
- `TestListDefinitionsJSON` — result is parseable (structured text listing definitions)

**Code (green):**

Handler loads path definitions (via cache), builds registry, formats definitions list as text. Reuses `loader.BuildRegistry` and `registry.Sources()`.

**Refactor:** None expected.

### Step 3.3: `check_model` tool handler [x]

**Test (red):** `internal/mcp/server_test.go`
- `TestCheckModelValid` — call with path to clean model → result contains "All relationship constraints satisfied", `.checked.yaml` written
- `TestCheckModelFindings` — call with violation model → result contains finding descriptions
- `TestCheckModelBadPath` — call with nonexistent path → error result

**Code (green):**

Handler runs the full pipeline: parse → load path defs → build registry → validate → check → write `.checked.yaml` → format results as text.

**Refactor:** Extract the pipeline logic shared between CLI and MCP into a reusable function in a shared location (e.g., `internal/pipeline/pipeline.go` or keep in `cmd/modelr` and have MCP call it). The simplest approach: MCP handlers call the same `model.Parse`, `loader.BuildRegistry`, `model.Validate`, `check.Check` functions directly.

### Step 3.4: `validate_model` tool handler [x]

**Test (red):** `internal/mcp/server_test.go`
- `TestValidateModelValid` — call with clean model → result contains assumptions and warnings
- `TestValidateModelBadPath` — nonexistent path → error result

**Code (green):**

Handler runs parse → load path defs → build registry → validate. Returns formatted text with warnings, assumptions, known unknowns.

**Refactor:** None expected.

### Step 3.5: `visualize_model` tool handler [x]

**Test (red):** `internal/mcp/server_test.go`
- `TestVisualizeModelReturnsDOT` — call with valid model → result contains DOT content (starts with `digraph`)
- `TestVisualizeModelWritesFile` — `.dot` file written as sibling

**Code (green):**

Handler runs full pipeline + DOT generation. Returns DOT content as text. Writes `.dot` (and `.svg` if graphviz available) as side effects.

**Refactor:** None expected.

### Step 3.6: `verify_model` stub [x]

**Test (red):** `internal/mcp/server_test.go`
- `TestVerifyModelStub` — call with any path → result text says behavioral verification is not yet implemented

**Code (green):**

Handler returns `mcp.NewToolResultText("Behavioral verification is not yet implemented.")`.

**Refactor:** None expected.

### Step 3.7: `mcp` CLI command [x]

**Test (red):** `cmd/modelr/main_test.go`
- `TestMCPCommandExists` — app has `mcp` command registered

Note: Full stdio integration testing is not practical in unit tests (it blocks on stdin). The test verifies the command exists; actual MCP protocol testing is done via the tool handler tests in Step 3.2–3.6.

**Code (green):**

Register `mcp` command:
```go
{
    Name:  "mcp",
    Usage: "Start MCP server on stdio",
    Action: func(ctx context.Context, cmd *cli.Command) error {
        s := mcpserver.NewServer(mcpserver.EnvConfig{...})
        return server.ServeStdio(s)
    },
}
```

**Refactor:** None expected.

---

## Phase 4 — Init Command [COMPLETE]

### Step 4.1: Skill content (`internal/init/scaffold.go`) [x]

**Test (red):** `internal/init/scaffold_test.go`
- `TestModelSkillContent` — `ModelSkillContent()` returns string containing "list_definitions", "check_model", model YAML structure example
- `TestOutageReportSkillContent` — `OutageReportSkillContent()` returns string containing "check_model", incident report format, severity levels

**Code (green):**

Embed skill markdown files via `//go:embed`:
```go
//go:embed skills/model.md
var modelSkill string

//go:embed skills/outage-report.md
var outageReportSkill string
```

Create `internal/init/skills/model.md` and `internal/init/skills/outage-report.md` adapted from the mbase versions but using modelr tool names (`list_definitions`, `check_model`, `visualize_model`).

**Refactor:** None expected.

### Step 4.2: MCP config content [x]

**Test (red):** `internal/init/scaffold_test.go`
- `TestMCPConfigContent` — `MCPConfigContent()` returns valid JSON with `modelr` server entry pointing to `modelr mcp` command
- `TestMCPConfigParsesAsJSON` — JSON unmarshals without error

**Code (green):**

Generate `.claude/mcp.json` content:
```json
{
  "mcpServers": {
    "modelr": {
      "command": "modelr",
      "args": ["mcp"]
    }
  }
}
```

**Refactor:** None expected.

### Step 4.3: Scaffold writer with conflict detection [x]

**Test (red):** `internal/init/scaffold_test.go`
- `TestScaffoldCreatesFiles` — empty target dir → creates `.claude/skills/model.md`, `.claude/skills/outage-report.md`, `.claude/mcp.json`; returns list of created files
- `TestScaffoldCreatesDirectories` — `.claude/skills/` doesn't exist → created
- `TestScaffoldSkipsExistingFile` — `.claude/skills/model.md` already exists → skipped, listed in skipped files, not overwritten
- `TestScaffoldSkipsExistingMCPConfig` — `.claude/mcp.json` exists → skipped
- `TestScaffoldPartialExisting` — one file exists, others don't → existing skipped, others created
- `TestScaffoldAllExist` — all files exist → all skipped, no error

**Code (green):**

```go
type ScaffoldResult struct {
    Created []string
    Skipped []string
}

func Scaffold(targetDir string) (*ScaffoldResult, error)
```

For each file: check if exists → skip or write. Return result listing what happened.

**Refactor:** None expected.

### Step 4.4: `init` CLI command [x]

**Test (red):** `cmd/modelr/main_test.go`
- `TestInitCommandCreatesFiles` — run `init` in temp dir → skill files and mcp.json created, output lists created files
- `TestInitCommandSkipsExisting` — run `init` twice → second run reports files skipped
- `TestInitCommandOutput` — output mentions each created/skipped file

**Code (green):**

Register `init` command:
```go
{
    Name:  "init",
    Usage: "Initialize Claude Code integration (skills + MCP config)",
    Action: func(ctx context.Context, cmd *cli.Command) error {
        return runInit(cmd)
    },
}
```

`runInit`: call `scaffold.Scaffold(".")`, print results.

**Refactor:** None expected.

---

## Phase 5 — Integration [COMPLETE]

### Step 5.1: End-to-end visualization [x]

**Test (red):** `cmd/modelr/main_test.go`
- `TestEndToEndVisualize` — check + visualize clean model → `.checked.yaml` and `.dot` both written, DOT contains all component names and edges
- `TestEndToEndVisualizeWithFindings` — violation model → DOT contains red-styled edges
- `TestEndToEndVisualizeWithKnownUnknowns` — model with missing properties → DOT contains orange-styled elements

**Code (green):**
Should pass with existing code. Creates test fixture models if needed.

**Refactor:** None expected.

### Step 5.2: End-to-end MCP tool calls [x]

**Test (red):** `internal/mcp/server_test.go`
- `TestMCPCheckThenVisualize` — call `check_model` then `visualize_model` on same model → both succeed, files written
- `TestMCPListThenCheck` — call `list_definitions` then `check_model` → definitions listed, model checked

**Code (green):**
Should pass with existing code. Integration-level validation of tool sequencing.

**Refactor:** None expected.

### Step 5.3: End-to-end init + MCP [x]

**Test (red):** `cmd/modelr/main_test.go`
- `TestEndToEndInitCreatesValidSkills` — run `init` → read created `model.md`, verify it contains `list_definitions` and `check_model` tool references
- `TestEndToEndInitMCPConfig` — run `init` → read `.claude/mcp.json`, verify it references `modelr mcp`

**Code (green):**
Should pass with existing code.

**Refactor:** None expected.

---

## Skill Content Design

### `.claude/skills/model.md`

Adapted from the mbase version with these changes:
- Tool names: `list_definitions` (not `list_schemas`), `check_model`, `visualize_model`
- Adds a discovery step calling `list_definitions` before model generation
- Adds a visualization step: after checking, call `visualize_model` to generate DOT/SVG
- References modelr-specific uncertainty categories
- Describes the `.checked.yaml` output format

### `.claude/skills/outage-report.md`

Adapted from mbase with:
- References `check_model` tool (not `verify_model` since behavioral verification is deferred)
- Focuses on arithmetic findings for now
- Notes that behavioral findings will be available in a future version
- Keeps the incident report format, timeline structure, and guidelines

### `.claude/mcp.json`

```json
{
  "mcpServers": {
    "modelr": {
      "command": "modelr",
      "args": ["mcp"]
    }
  }
}
```

---

## Self-Critique and Revisions

### Issues found and addressed

1. **Pipeline code duplication** — CLI commands and MCP handlers both need parse → validate → check. **Resolution:** Both call the same lower-level functions (`model.Parse`, `loader.BuildRegistry`, `model.Validate`, `check.Check`) directly. No shared "pipeline" abstraction is needed — the glue code is 10-15 lines in each handler and the parameters differ (CLI reads from args + flags; MCP reads from tool request). Premature abstraction would couple the two unnecessarily.

2. **MCP handler testing without stdio** — Can't easily test the full MCP protocol in unit tests. **Resolution:** Test the handlers directly by calling them with constructed `mcp.CallToolRequest` objects. The MCP framework's stdio transport is tested by `mcp-go` itself; we test our handler logic.

3. **`verify_model` is a stub** — The outage report skill references verification. **Resolution:** The stub returns a clear "not yet implemented" message. The outage report skill is written to work with arithmetic findings alone, with a note about future behavioral findings.

4. **Graphviz may not be installed** — Rendering tests would fail. **Resolution:** SVG rendering tests use `t.Skip()` when graphviz is not available. DOT generation tests don't depend on graphviz.

5. **`init` must be idempotent** — Running it twice shouldn't break anything. **Resolution:** Step 4.3 explicitly tests skip-on-existing behavior. No file is ever overwritten.

6. **Skill content references MCP tool names** — Must match exactly. **Resolution:** Skills use `list_definitions`, `check_model`, `validate_model`, `visualize_model` — matching the spec section 6.2 tool table.

7. **DOT edge finding correlation** — Need to match findings to edges. A finding has `Upstream` and `Downstream` fields; an edge has `Source` and `Target`. **Resolution:** An edge is "in violation" if any finding exists where `finding.Upstream == edge.Source && finding.Downstream == edge.Target`. Same pattern for known unknowns.

8. **MCP server `EnvConfig`** — The MCP server needs `$MODELR_PATH` and `$HOME` for cache/path loading, same as CLI commands. **Resolution:** `EnvConfig` struct passed into `NewServer`, same pattern as CLI's `envConfig`.

9. **Init command working directory** — `modelr init` should scaffold relative to the current directory. **Resolution:** `Scaffold(".")` uses the current working directory. The CLI command doesn't need a path argument.

### Ordering validation

- Phase 1 (DOT generation) has no new dependencies — correct start
- Phase 2 (SVG rendering + CLI command) depends on Phase 1 — correct
- Phase 3 (MCP server) depends on Phase 1 for `visualize_model` handler — correct
- Phase 4 (init command) has no dependency on Phases 1–3 (just embeds static files) — could run in parallel, but sequential is fine
- Phase 5 (integration) depends on all — correct final phase

---

## Estimated Step Count

| Phase | Steps | Tests |
|-------|-------|-------|
| 1. DOT Generation | 4 | ~14 |
| 2. SVG Rendering + CLI | 2 | ~7 |
| 3. MCP Server | 7 | ~14 |
| 4. Init Command | 4 | ~10 |
| 5. Integration | 3 | ~7 |
| **Total** | **20** | **~52** |
