# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Purpose

This directory contains example `.model.yaml` files used to test and demonstrate modelr during development. It is a subdirectory of the main modelr project.

## Working with models

The `modelr` binary is assumed to be on `$PATH`. Key commands:

```bash
modelr check <file>.model.yaml       # Run constraint checks, writes .checked.yaml
modelr validate <file>.model.yaml    # Validate model against type schemas
modelr verify <file>.model.yaml      # Run behavioral simulations
modelr viz <file>.model.yaml         # Generate DOT/SVG visualization
```

## MCP integration

This directory configures modelr as an MCP server (`.claude/mcp.json`). The MCP tools (`list_definitions`, `check_model`, `visualize_model`) are available and should be preferred when working interactively with models.

## Skills

Two custom skills are configured in `.claude/skills/`:

- **modelr-model** (`/modelr-model`): Generate, update, and check system models. Always call `list_definitions` first to discover available types and relationship templates before creating or modifying a model.
- **modelr-outage-report** (`/modelr-outage-report`): Generate plausible prose incident reports from check findings for pre-mortem analysis.

## Model file conventions

- Extension: `.model.yaml`
- Checked output: `.checked.yaml` (auto-generated sibling file)
- Model version: `"0.2"`
