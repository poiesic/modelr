# modelr - Claude Code Context

## Project Overview

modelr is a composable system modeling tool with explicit uncertainty. Practitioners describe systems as YAML models and modelr validates, checks constraints, and runs behavioral simulations. See `docs/spec.md` for the full specification.

## Technology Stack

| Component | Technology |
|-----------|------------|
| Language | Go |
| CLI | urfave/cli v3 |
| YAML | gopkg.in/yaml.v3 |
| MCP | mark3labs/mcp-go |
| Task Runner | Task (taskfile.dev) |

## Build & Test

```bash
task build              # Build binary → bin/modelr
task test               # Run all tests
task lint               # Run linter
```

## Project Structure

```
cmd/modelr/             # CLI entry point
internal/model/         # Types, parser, validator
internal/check/         # Expression evaluator, constraint checker
internal/verify/        # Simulation engine, SPRT
internal/viz/           # DOT generation
internal/loader/        # Definition loading, caching
internal/mcp/           # MCP server tool registration
embed/                  # Embedded node/relationship definitions
docs/                   # Specification and design docs
```

## Code Patterns

- Use custom types for context keys
- Use `testify` for all test assertions
- Use `//go:embed` for bundling standard definitions
- Use `internal/` for non-public packages
