# modelr

A composable system modeling tool with explicit uncertainty. Describe systems as YAML models and modelr validates, checks constraints, and runs behavioral simulations.

## Key Ideas

- **Declarative models, imperative checks** — the model says "this is my system"; the tool says "here's what could go wrong"
- **Explicit uncertainty** — unknown values, deferred decisions, and unresolved tradeoffs are first-class model elements
- **Assumptions are visible** — any value filled from a default is reported, not hidden

## Install

```bash
go install github.com/poiesic/modelr/cmd/modelr@latest
```

## Usage

```bash
modelr validate model.model.yaml   # Validate a model
modelr check model.model.yaml      # Run constraint checks
modelr verify model.model.yaml     # Run behavioral simulations
modelr viz model.model.yaml        # Generate DOT visualization
```

modelr also runs as an MCP server over stdio.

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
