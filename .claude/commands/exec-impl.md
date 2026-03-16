---
argument-hint: <plan-name>
description: Execute an implementation plan using TDD practices
---

Execute the implementation plan at `docs/plans/$1.md`. (Try `docs/plans/$1-implementation-plan.md`, too.)

Before starting, review these documents to understand project conventions:
- `CLAUDE.md` - Project guidelines and patterns
- `docs/spec.md` - modelr specification

Then implement the plan step by step, following TDD practices:

1. **Write the test first** (red) - Create the failing test as specified in the plan
2. **Implement minimal code** (green) - Write just enough code to pass the test
3. **Refactor if needed** - Clean up while keeping tests green
4. **Update the plan** - Mark the completed step with `[x]` in the implementation plan
5. **Move to next step** - Only proceed when current step's tests pass

When completing a phase, update the phase status in the implementation plan to reflect progress, and then move the plan to the docs/plans/done subdirectory.

## Useful Taskfile commands

| Command      | Purpose                          |
| ------------ | -------------------------------- |
| `task build` | Build modelr binary → bin/modelr |
| `task test`  | Run all tests                    |
| `task lint`  | Run linter                       |

Run tests frequently to verify progress. Do not proceed to the next implementation step until the current step's tests pass.

The implementation plan is not done until `task build` and `task test` both pass with no errors.
