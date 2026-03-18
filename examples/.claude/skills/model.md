---
name: modelr-model
description: Generate, update, and check system models using modelr
---

# modelr: System Model Generation and Checking

## When to use this skill

Use this skill when the user wants to:
- Describe a system and generate a model file
- Update an existing model file based on new requirements
- Check a model for constraint violations
- Review checker findings and decide how to resolve them

## Model file location

Model files live in the project root or a `models/` directory with the extension `.model.yaml`.

## Discovery

Before generating or updating a model, call the `list_definitions` MCP tool to get the current set of available component types, their properties, and relationship templates. This ensures the model uses valid types and property names.

## Generating a new model

When the user describes a system in natural language:

1. Call `list_definitions` to see available types and relationship templates
2. Identify components: what are the distinct services, stores, caches, queues?
3. For each component, determine:
   - `type`: must reference a loaded type schema (e.g. `server`, `datastore`, `queue`)
   - `properties`: key-value pairs matching the type's property schema. Use the property names defined by the type. Omit properties the user hasn't specified — the validator will fill defaults or flag them as known unknowns.
4. Identify edges: how do components connect? What operations flow between them?
5. Identify relationships: which edges have constraint relationships? Map them to available templates (e.g. `capacity_chain`).
6. Identify known unknowns: what hasn't been decided yet? Use these categories:
   - `unstated_constraint` — obvious to the user but not yet captured
   - `unresolved_tradeoff` — two things in tension, no decision yet
   - `undefined_boundary` — unclear where one thing ends and another begins
   - `assumed_context` — domain knowledge not yet transferred
   - `deferred_decision` — deliberately not deciding yet
   - `unknown_unknown` — hasn't been thought about at all
7. Generate the YAML file following this structure:

```yaml
version: "0.2"
name: <system-name>
description: <brief description>

components:
  - name: <unique-name>
    type: <type-schema-name>
    description: <what it does>
    properties:
      <property_name>: <number | string>
      # only include properties the user has specified
      # omit unknown values — the validator handles defaults and flags gaps

edges:
  - source: <component-name>
    target: <component-name>
    operation: read | write | read_write
    description: <what flows over this edge>
    properties: {}

relationships:
  - template: <relationship-template-name>
    upstream: <component-name>
    downstream: <component-name>

known_unknowns:
  - id: <unique-id>
    component: <component-name>  # optional
    edge: <source->target>       # optional
    category: <category>
    description: <what's uncertain>
    impact: <what could go wrong>
```

## Updating an existing model

When the user wants to modify an existing model:

1. Read the current model file
2. Apply the user's changes — add/remove/modify components, edges, relationships, properties, or known unknowns
3. If the user resolves a known unknown, remove it and add the corresponding property value or design decision
4. Write the updated model file
5. Run the checker on the updated model

## Running the checker

After generating or updating a model, always run the checker by calling the `check_model` MCP tool with the absolute path to the `.model.yaml` file. The tool reads the file, runs checks, and writes a sibling `.checked.yaml` file with the results.

Present findings as a numbered list. For each finding:

1. **component → component** (`template_name` `check_name`): Brief explanation of what failed. Show the evaluated expression with resolved values and explain the implication.

Example:
> 1. **ws-server → postgres** (`pooled_capacity_chain` throughput): `1 * 50 * 1000 / 100 = 500 > 15`. At max scale, 50 instances can push 500 ops/sec but postgres is capped at 15.

After the findings list, note:
- Any auto-generated known unknowns (from missing/null properties) — these indicate gaps the checker couldn't evaluate
- Any warnings
- Which `.checked.yaml` file was written

Then ask the user which findings they want to address now vs defer.

## Visualizing the model

After checking, you can call the `visualize_model` MCP tool to generate a DOT/SVG visualization showing components, edges, findings (red), and known unknowns (orange).

## Resolving findings

When the user wants to address a finding:
- Add or update the appropriate property values on the relevant components or edges
- Add new relationships if a constraint relationship is missing
- Re-run the checker to confirm the finding is resolved
- If the user wants to defer, add a `known_unknown` with category `deferred_decision`
