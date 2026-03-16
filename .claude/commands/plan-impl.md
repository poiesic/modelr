---
argument-hint: <description of work>
description: Create TDD implementation plan for modelr features or bug fixes
---

Create a detailed implementation plan for the following work:

$ARGUMENTS

First, ask the user for a short kebab-case name for the implementation plan (e.g., `add-expression-evaluator`, `fix-cache-staleness`). This will be used for the output filename.

Then create the plan following TDD practices:

1. **Review project guidelines** - Review `CLAUDE.md` and `docs/spec.md` to understand project conventions and the modelr specification.
2. **Analyze the requirements** - Identify all packages, functions, and data requirements
3. **Break down into implementable units** - Each unit should be small enough to implement with a test-first approach
4. **Interleave unit tests** - For each implementation step, specify:
   - The test(s) to write first (red)
   - The minimal code to pass (green)
   - Any refactoring needed (refactor)

Then review your plan performing a self-critique for edge cases, redundant work, unnecessary work, work ordering, and missing dependencies between steps. Address any issues you find.

Then write your revised plan to `docs/plans/<plan-name>-implementation-plan.md`.

## Useful Taskfile commands

| Command | Purpose |
|---------|---------|
| `task build` | Build modelr binary → bin/modelr |
| `task test` | Run all tests |
| `task lint` | Run linter |
