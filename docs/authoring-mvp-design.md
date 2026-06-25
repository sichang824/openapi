# OAPI Authoring CLI Design

Status: current on 2026-05-02
Owner: @ann
Audience: backend engineers, tech leads

---

## Scope

This document defines the current authoring command surface for `oapi`.

The goal is intentionally narrow:

- initialize a maintainable split OpenAPI workspace
- automate repetitive file creation and root `$ref` wiring
- orchestrate external tools for bundling, validation, and code generation

The MVP explicitly does not try to infer business schemas, reverse-engineer handlers, or preserve YAML comments through AST-aware edits.

## Command Surface

The implemented command set is:

```bash
oapi init --dir ./api/openapi --title "Experts Backend API" --version 0.1.0
oapi add path --dir ./api/openapi --business workflow --path /workflow-runs
oapi add schema --dir ./api/openapi --business common --file common --name ErrorResponse --kind object
oapi add parameter --dir ./api/openapi --business common --file pagination --name Page --in query
oapi add response --dir ./api/openapi --business common --file errors --name Error
oapi add schema --dir ./api/openapi --business workflow --name WorkflowRun --kind object
oapi fmt --dir ./api/openapi
oapi bundle --dir ./api/openapi --out ./api/openapi/dist/openapi.yaml
oapi doctor --dir ./api/openapi
oapi validate --dir ./api/openapi
oapi generate --dir ./api/openapi --lang go --out ./internal/sdk/generated/openapi
```

`oapi add path` groups path items into `<business>/paths.yaml`, and each path key is wired back into `index.yaml` through a JSON Pointer `$ref`.

## File Layout

`oapi init` creates this structure:

```text
api/openapi/
  index.yaml
  common/
    common.yaml
    pagination.yaml
    errors.yaml
  workflow/
    paths.yaml
    schemas.yaml
    parameters.yaml
    responses.yaml
```

`common/` holds shared component files. Each business directory then keeps its own `paths.yaml`, `schemas.yaml`, `parameters.yaml`, and `responses.yaml` together.

The root entry file starts with only these writable sections:

- `paths`
- `components.schemas`
- `components.parameters`
- `components.responses`

Everything else stays minimal so the generated scaffold remains easy to edit.

## Behavioral Rules

### Minimal templates

Generated files are intentionally skeletal.

`oapi add path` adds an entry under `<business>/paths.yaml` like:

```yaml
/workflow-runs:
  get:
    summary: TODO
    operationId: todoOperation
    responses:
      "200":
        description: OK
```

`oapi add schema --kind object` writes:

```yaml
MySchema:
  type: object
  properties: {}
```

`oapi add parameter` writes a minimal reusable component with a guessed wire name and a conservative schema type.

### Stable root edits

When a new path or reusable component is added, `oapi` rewrites `index.yaml` to:

- insert the matching `$ref`
- keep keys sorted under `paths`
- keep keys sorted under `components.schemas`, `components.parameters`, and `components.responses`

The tool rewrites YAML formatting when needed. Comment preservation is not a first-phase requirement.

### Duplicate protection

All add commands reject existing files or definitions unless `--force` is supplied.

This prevents silent drift between split files and the root aggregator.

## Internal Modules

The current implementation is split into these modules:

- `internal/scaffold`: directory creation, file naming, and template generation
- `internal/edit`: YAML loading, `$ref` insertion, sorted writes, and component upserts
- `internal/bundle`: external tool orchestration for producing a single-file spec
- `internal/generate`: temporary bundling plus `openapi-generator` validation/generation orchestration
- `internal/doctor`: split-workspace and tool readiness checks

`internal/cli` remains the command adapter layer that validates flags and delegates to those modules.

## Bundle Strategy

`oapi bundle` does not implement a custom resolver.

Instead it delegates to one of these tools:

- `redocly`
- `swagger-cli`
- `npx @redocly/cli`

Auto-detection tries them in that order. The command can also be pinned with `--tool`.

This keeps the MVP focused on authoring workflow automation rather than spec-resolution internals.

## Validate and Generate Strategy

`oapi validate` and `oapi generate` do not point external tools directly at the split directory.

Instead they:

1. bundle `index.yaml` and its refs into a temporary single-file spec
2. invoke `openapi-generator validate` or `openapi-generator generate` against that temporary file
3. remove the temporary bundle on exit

Generator CLI auto-detection currently tries:

- `openapi-generator`
- `npx @openapitools/openapi-generator`

`oapi generate` currently supports a small, explicit option surface:

- `--lang`
- `--out`
- `--config`
- repeatable `--additional-property`
- repeatable `--global-property`

## Doctor Strategy

`oapi doctor` provides a fast preflight check before bundle, validate, or generate.

It currently checks:

- workspace directory existence
- root `index.yaml`
- `common/`
- bundle tool resolution
- generator tool resolution

It supports both:

- human-readable text output for terminal use
- `--json` output for CI and scripts

## Format Strategy

`oapi fmt` provides a deterministic rewrite pass for the tracked split workspace files.

It currently rewrites:

- root `index.yaml`
- files under `common/`
- files under business directories such as `workflow/paths.yaml` and `workflow/schemas.yaml`

The formatter intentionally stays mechanical:

- sort mappings recursively
- normalize YAML indentation/layout
- avoid semantic rewrites such as renames or inferred field changes

## Non-Goals

The following items are still intentionally deferred:

- `oapi move`
- `oapi remove`
- schema field inference
- handler-to-spec reverse generation
- comment-preserving structural edits

## Validation Strategy

The current implementation is considered acceptable when it can:

1. initialize a split OpenAPI workspace
2. create a path file and wire it into `index.yaml`
3. create schema, parameter, and response definitions and wire them into `index.yaml`
4. reject duplicates unless `--force` is used
5. rewrite tracked YAML files into a stable sorted layout through `oapi fmt`
6. bundle the split spec into a single artifact via an external tool
7. report missing workspace and tool prerequisites through `oapi doctor`
8. validate the split spec through `openapi-generator`
9. generate code from the split spec through `openapi-generator`

The repository test suite covers these behaviors with temporary directories and fake bundler/generator executables.
