---
name: openapi
description: Use when working with OpenAPI specs, reviewing OAS 3.x contracts, scaffolding split specs, generating code, locating endpoints in a large bundled or standalone spec file, or calling a documented API endpoint through the local oapi CLI
---

# Skill: openapi

Practical guidance for OpenAPI review plus the local `oapi` authoring/query/call CLI. Treat the spec as the source of truth and verify commands against the real file before advising.

## When to use

- The user wants help writing, reviewing, or fixing an OpenAPI 3.x document.
- The user wants to validate a spec before generation or integration work.
- The user wants to generate SDK/server code with OpenAPI Generator.
- The user wants to initialize or maintain a split OpenAPI directory with minimal templates.
- The user wants to normalize split-spec YAML formatting before review or CI.
- The user needs to find an endpoint quickly in a large `openapi.json` or `openapi.yaml`.
- The user wants to inspect an endpoint contract at increasing detail levels.
- The user wants to call a documented endpoint directly from the terminal.

## Constraints and safety

- Never invent endpoints, schemas, security flows, or parameter names not present in the spec.
- Prefer non-destructive commands first: `query`, `bundle`, `doctor`, `validate`, `config-help`.
- For `oapi call`, warn about side effects for `POST`/`PUT`/`PATCH`/`DELETE`.
- Use explicit file paths, generator names, output directories, and base URLs.
- Validate assumptions against the actual spec before suggesting commands.
- If `query -h` or `call -h` exits with status 1 because of `flag: help requested`, treat that as normal Go flag behavior, not a CLI failure.
- Authoring MVP does not infer business fields; it only creates/editable skeletons and wires `$ref`s.
- `oapi fmt` rewrites only `index.yaml` plus tracked split YAML files under `common/` and business directories.
- `oapi doctor` checks split-workspace shape plus bundle/generator tool availability before heavier workflows.
- `oapi validate` and `oapi generate` bundle to a temporary single-file spec before invoking external generator tooling.

## Local references to consult first

- `assets/openapi-docs/introduction.md`
- `assets/openapi-docs/specification.md`
- `assets/openapi-docs/specification-paths.md`
- `assets/openapi-docs/specification-components.md`
- `assets/openapi-docs/best-practices.md`

## Core command patterns

### Initialize a split spec workspace

```bash
oapi init --dir ./api/openapi --title "Experts Backend API" --version 0.1.0
```

### Add split-spec files and wire refs

```bash
oapi add path --dir ./api/openapi --business workflow --path /workflow-runs
oapi add path --dir ./api/openapi --business workflow --path /workflow-runs/{runZid}
oapi add schema --dir ./api/openapi --business common --file common --name ErrorResponse --kind object
oapi add parameter --dir ./api/openapi --business common --file pagination --name Page --in query
oapi add response --dir ./api/openapi --business common --file errors --name Error
oapi add schema --dir ./api/openapi --business workflow --name WorkflowRun --kind object
```

### Normalize split-spec YAML

```bash
oapi fmt --dir ./api/openapi
```

### Bundle a split spec

```bash
oapi bundle --dir ./api/openapi --out ./api/openapi/dist/openapi.yaml
```

### Check workspace/tool readiness

```bash
oapi doctor --dir ./api/openapi
oapi doctor --dir ./api/openapi --json
```

### Validate a spec

```bash
oapi validate --dir ./api/openapi
```

### List generators

```bash
openapi-generator list -s | tr ',' '\n'
```

### Inspect generator-specific options

```bash
openapi-generator config-help -g typescript-fetch
```

### Generate code

```bash
oapi generate \
  --dir ./api/openapi \
  --lang typescript-fetch \
  --out ./out/typescript-sdk
```

### Query endpoints from an OpenAPI spec file

```bash
# List endpoints with the default summary view
oapi query -f ./openapi.json
oapi query -n skill
oapi query -f ./openapi.yaml -q upload -vv

# Search and inspect progressively
oapi query -f ./openapi.json -q user -v
oapi query -f ./openapi.json -q order -vv
oapi query -f ./openapi.json -q payment -vvv --limit 20
```

`-n` / `--name` loads `${OAPI_SPECS_DIR:-~/.openapi/specs}/<name>.openapi.yaml`; for example, `-n skill-internal` loads `skill-internal.openapi.yaml`. Use either `-n` or `-f`, not both.

### Call API endpoints

```bash
# Call from YAML or JSON specs
oapi call -f ./openapi.yaml -e "GET /users" --base-url https://api.example.com
oapi call -n skill -e "GET /users" --base-url https://api.example.com

# Call with inline JSON params (preferred for small payloads)
oapi call -f ./openapi.json -e "POST /cart/add" --params '{"item_id":"123","quantity":2}'

# Call with params from file for larger payloads
oapi call -f ./openapi.json -e "POST /goods/list" --params-file params.json

# Enforce strict validation
oapi call -f ./openapi.json -e "POST /cart/add" --params '{"item_id":"123"}' --strict

# Override server URL from the spec
oapi call -f ./openapi.json -e "GET /users" --base-url https://api.example.com

# Bearer auth and custom headers
oapi call -f ./openapi.json -e "GET /protected" --base-url https://api.example.com --bearer-token "$TOKEN"
oapi call -f ./openapi.json -e "GET /protected" --base-url https://api.example.com --header "X-Trace-Id: debug-123"

# Opt-in contract-filtered environment headers
OAPI_HEADER_X_API_KEY="$TOKEN" oapi call -n kb -e "GET /protected" --auto-headers
OAPI_AUTO_HEADERS=1 OAPI_HEADER_AUTHORIZATION="Bearer $TOKEN" oapi call -n iam -e "GET /protected"

# Cookie session (curl -b style; never commit secrets)
oapi call -f ./openapi.json -e "GET /api/me" --base-url https://api.example.com --cookie 'session_id=...; access_token=...'
oapi call -f ./openapi.json -e "GET /api/me" --base-url https://api.example.com --cookie-path ~/.config/myapp/cookies.jar

# Binary upload (curl --data-binary style; path/query params still from --params)
oapi call -f ./openapi.yaml -e "PUT /files/{id}" \
  --base-url https://api.example.com \
  --params '{"id":"file-abc"}' \
  --body-file ./payload.bin \
  --content-type application/octet-stream
```

## `oapi query` behavior

### Search semantics

- `-q` is optional in practice.
- Without `-q`, `oapi query` lists endpoints in path order using the default summary view.
- With `-q`, results are keyword-ranked using method, path, summary, description, `operationId`, and tags.
- `--limit` applies both to search results and the no-keyword listing mode.

### Verbosity contract

- Default: show `METHOD PATH` and `summary`.
- `-v`: add `operationId`, `tags`, and operation `description`.
- `-vv`: add parameter summary, request body content-type summary, and response status-code summary.
- `-vvv`: add full contract detail, including resolved parameter types and expanded request/response schema structure.

### Choosing the right verbosity

- Use default when the user just needs a fast endpoint inventory.
- Use `-v` when deciding whether an endpoint is the right one.
- Use `-vv` when preparing a call and needing the contract outline.
- Use `-vvv` when the user needs the actual request/response shape without manually opening the spec.

### What `-vvv` now expands

- Parameter detail: `in`, `name`, `required`, resolved `type`
- Parameter metadata when available: `description`, `example`, `enum`, `default`, `maximum`
- Request body description and one section per content type
- Response description and one section per status code
- Expanded object schemas: `required` fields and nested `property` entries
- Nested arrays via `items`
- `$ref` targets retained in output while also showing resolved structure

## `oapi call` behavior

### Parameter sources

- Prefer `--params '{"k":"v"}'` for small or medium payloads.
- Use `--params-file params.json` for large or awkward nested payloads.
- Use `--params-url "k1=v1&k2=v2"` when the shell quoting experience is poor (especially PowerShell on Windows).
- Use `--body-file path` for **raw binary or non-JSON bodies** (same idea as `curl --data-binary`). Pair with `--content-type` when the spec does not define one or you need to override it. Path and query parameters still come from `--params` / `--params-url`.
- `--params`, `--params-file`, and `--params-url` are mutually exclusive; pass exactly one source.
- `--params-url` decodes percent-encoded values (for example, `%20` becomes a space).
- If neither is provided, the CLI validates and sends an empty parameter map.
- Path parameters can be supplied through `--params` or `--params-file` using the parameter name from the spec.
- Query parameters defined at the path-item level are also valid call inputs; do not assume `call` only respects operation-local parameters.

### PowerShell/Windows guidance

- Prefer `--params-url` for flat key/value payloads to avoid JSON escape noise.
- Quote the entire query string with double quotes and URL-encode special characters in values.
- If the payload is nested or complex, use `--params-file` rather than forcing escaped JSON inline.
- For values containing `&`, `%`, or spaces, encode them first (for example, space -> `%20`).

Example:

```bash
oapi call -f .\openapi.json -e "POST /goods/list" --params-url "page=1&page_size=20&keyword=hello%20world"
```

### Cookie / session (call)

- `--cookie` sends a raw `Cookie` header string (semicolon-separated pairs; same idea as `curl -H 'Cookie: ...'`).
- `--cookie-path` reads a Netscape cookie jar file (same idea as `curl -b /path/to/cookies.jar`).
- Do not use both `--cookie` and `--cookie-path` in one invocation.
- At `-vvv`, the CLI prints only `Cookie: <redacted; length=N>` so secrets are not echoed.
- Treat cookie values like passwords: do not commit them or paste them into logs.

### Header / bearer auth (call)

- `--bearer-token` is the shortest path for `Authorization: Bearer <token>`.
- `--header 'Name: Value'` is repeatable and is the generic escape hatch for request headers.
- Do not pass both `--bearer-token` and `--header 'Authorization: ...'` in the same command.
- Prefer `--bearer-token` for protected endpoint debugging unless the target API uses a non-bearer scheme.

### Automatic environment headers (call)

- Automatic environment headers are disabled by default. Enable them with `--auto-headers` or `OAPI_AUTO_HEADERS=1`; explicit `--auto-headers=false` overrides the environment.
- Candidate variables use `OAPI_HEADER_*`, for example `OAPI_HEADER_X_API_KEY` -> `X-Api-Key`. Empty values are ignored; CR/LF, duplicate normalized names, and reserved transport/content headers are rejected.
- The current operation must declare the candidate through effective OpenAPI security or an `in: header` parameter. `security: []` disables inherited authentication for a public operation.
- Explicit `--header`, `--bearer-token`, `--cookie`, and `--content-type` inputs take precedence over environment candidates.
- Environment-sourced values are always treated as secrets. Output reports only header names, target origin, and redaction markers.
- An absolute `--base-url` must have the same origin as the spec server when automatic headers are applied. Cross-origin redirects are rejected.

### Base URL rules

- `--base-url` overrides the spec.
- If `--base-url` is omitted, `oapi call` uses the first `servers[].url` from the spec.
- If neither exists, the command fails and the user must provide `--base-url`.
- With automatic headers active, a different origin is rejected. A relative or unresolved spec server produces a warning because the origin cannot be verified.

### Validation modes

Loose mode, default:

- validates required parameters
- warns on unknown parameters
- still sends the request if there are only warnings

Strict mode, `--strict`:

- validates required parameters
- validates parameter names and expected schema usage more aggressively
- fails fast on validation errors
- blocks the request when validation fails

### Call verbosity

- Default: print formatted response only.
- `-v` and above: show request preamble such as method/path and resolved base URL before the response.
- Higher verbosity on `call` is mainly for request inspection; use it when debugging constructed requests.

## Typical playbooks

### Playbook 1: Review an unfamiliar spec

1. Check top-level structure: `openapi`, `info`, `servers`, `paths`, `components`.
2. Run `oapi fmt --dir ...` if the immediate goal is deterministic review diffs.
3. Run `oapi doctor --dir ...` if the workspace or local toolchain may be incomplete.
4. Run `oapi validate --dir ...` if the goal includes generation or compliance.
5. Inspect a few representative endpoints with `oapi query -v` or `-vv`.
6. Use `-vvv` only for endpoints that need contract-level investigation.

### Playbook 2: Find the right endpoint fast

1. Start with `oapi query -f ./openapi.json` if the API surface is small enough.
2. If it is large, search by domain term with `-q`.
3. Use `-v` to shortlist, then `-vv` or `-vvv` on the same keyword.
4. Tighten with `--limit` when many matches are returned.

### Playbook 3: Prepare a direct API call

1. Find the endpoint with `oapi query`.
2. Inspect at `-vv` or `-vvv` depending on how much schema detail is needed.
3. Choose one parameter source:
   - `--params` for simple JSON
   - `--params-url` for flat key/value calls and PowerShell-friendly usage
   - `--params-file` for large or nested payloads
4. Add authentication explicitly when needed:

- `--bearer-token` for bearer auth
- `--header` for custom request headers
- `--cookie` or `--cookie-path` for cookie-backed sessions

5. Start in loose mode for connectivity and flow debugging.
6. Switch to `--strict` when validating the final parameter shape.

### Playbook 4: Generator setup

1. Identify target generator and runtime.
2. Run `config-help -g <generator>`.
3. Choose explicit options instead of relying on defaults the user may not want.
4. Prefer `oapi generate --dir ... --lang ... --out ...` so split-spec bundling stays consistent.

## Output expectations

- Give commands the user can run immediately.
- Keep authoring templates minimal and editable; avoid pretending to know domain fields.
- Explain why a specific verbosity level is the right next step.
- Call out mismatches between spec content and CLI rendering explicitly.
- When relevant, mention that `oapi query` can be used without `-q` to list endpoints.
- For `oapi call`, state expected side effects and which validation mode you chose.
