# openapi - Help

Operational quick reference for this repository (skill + `oapi` authoring/query/call CLI).

The repository has two entrypoints:

- docs and skill guidance in `SKILL.md`, `README.md`, and `assets/openapi-docs/`
- the local `oapi` binary built from `cmd/oapi`

## Cobra help flow

Start with the binary's own grouped help before reading the command-specific notes below:

```bash
oapi --help
oapi add --help
oapi add path --help
oapi fmt --help
oapi doctor --help
oapi query --help
oapi generate --help
```

The root help groups commands into:

- Authoring commands
- Orchestration commands
- Inspection commands

`oapi add --help` also groups its subcommands into:

- Path commands
- Component commands

Subcommand help also includes concrete `Examples:` blocks, so use the nearest help page before running a write or generate action:

```bash
oapi add path --help
oapi add parameter --help
oapi fmt --help
oapi doctor --help
oapi query --help
oapi validate --help
oapi generate --help
oapi call --help
```

`query`, `call`, `generate`, `doctor`, and `fmt` help pages now include `Long` sections for boundaries and common pitfalls.

## Quick start

1. Show available Make targets:

   ```bash
   make
   ```

2. Install skill and CLI in one command:

   ```bash
   make build-cli
   ```

3. Run the Go test suite:

   ```bash
   make test
   ```

4. Install the built CLI into your preferred bin dir:

   ```bash
   make install BIN_DIR=$HOME/.local/bin
   ```

## `oapi` authoring CLI

Purpose: initialize and maintain a split OpenAPI directory without hand-editing root `$ref` wiring.

### Initialize a split spec

```bash
oapi init --dir ./api/openapi --title "Experts Backend API" --version 0.1.0
```

Creates:

- `index.yaml`
- `common/`
- business directories are created on demand, for example `workflow/`

### Add path items

```bash
oapi add path --dir ./api/openapi --business workflow --path /workflow-runs
oapi add path --dir ./api/openapi --business workflow --path /workflow-runs/{runZid}
```

Notes:

- Generated templates stay intentionally thin.
- Path items are grouped into `<business>/paths.yaml`.
- Root `paths:` refs are inserted automatically and sorted stably.
- Existing files/refs fail unless `--force` is passed.

### Add shared components

```bash
oapi add schema --dir ./api/openapi --business common --file common --name ErrorResponse --kind object
oapi add parameter --dir ./api/openapi --business common --file pagination --name Page --in query
oapi add response --dir ./api/openapi --business common --file errors --name Error
oapi add schema --dir ./api/openapi --business workflow --name WorkflowRun --kind object
oapi add parameter --dir ./api/openapi --business workflow --name RunZid --in path
```

Notes:

- Component files are reusable maps keyed by component name.
- Shared components live under `common/*.yaml`.
- Business-specific components live under `<business>/schemas.yaml`, `<business>/parameters.yaml`, and `<business>/responses.yaml`.
- Root `components.schemas`, `components.parameters`, and `components.responses` refs are inserted automatically.
- Existing definitions fail unless `--force` is passed.

### Format split files

```bash
oapi fmt --dir ./api/openapi
```

Notes:

- `oapi fmt` rewrites `index.yaml`, tracked files under `common/`, and tracked files under each business directory.
- It sorts mappings and normalizes indentation only; it does not rename files or rewrite spec semantics.
- `dist/` and other generated output trees are intentionally ignored.

### Bundle split files

```bash
oapi bundle --dir ./api/openapi --out ./api/openapi/dist/openapi.yaml
```

Notes:

- `oapi bundle` orchestrates external tools instead of reimplementing `$ref` resolution.
- Auto-detection currently tries `redocly`, `swagger-cli`, then `npx @redocly/cli`.
- Use `--tool` to pin a specific bundler when multiple are installed.

### Doctor split files

```bash
oapi doctor --dir ./api/openapi
oapi doctor --dir ./api/openapi --json
```

Notes:

- `oapi doctor` checks split-workspace shape and required tool resolution before bundle, validate, or generate.
- It reports missing `index.yaml` or `common/` directly.
- It reports missing bundle or generator CLIs using the same discovery rules as the orchestration commands.
- `--json` emits the same report as structured JSON so CI or scripts can parse it directly.
- It does not validate spec semantics; use `oapi validate` for schema and contract validation.

### Validate split files

```bash
oapi validate --dir ./api/openapi
```

Notes:

- `oapi validate` bundles the split tree to a temporary spec and then calls `openapi-generator validate`.
- Auto-detection currently tries `openapi-generator`, then `npx @openapitools/openapi-generator`.
- Use `--tool` to pin the generator CLI and `--bundle-tool` to pin the bundler.

### Generate code from split files

```bash
oapi generate --dir ./api/openapi --lang go --out ./internal/sdk/generated/openapi
oapi generate --dir ./api/openapi --lang typescript-fetch --out ../experts-web/src/generated/api
```

Notes:

- `oapi generate` bundles the split tree to a temporary spec and then calls `openapi-generator generate`.
- `--lang` and `--out` are required.
- `--config`, `--additional-property`, and `--global-property` are forwarded to openapi-generator.
- Use `--tool` to pin the generator CLI and `--bundle-tool` to pin the bundler.

## `oapi` query CLI

Purpose: quickly inspect endpoints from a bundled or standalone OpenAPI spec file, either by listing all endpoints or searching by keyword.

### Basic command

```bash
oapi query -f ./openapi.json
oapi query -n skill
```

`-n` / `--name` maps a short name to `${OAPI_SPECS_DIR:-~/.openapi/specs}/<name>.openapi.yaml`. For example, `-n skill-internal` loads `skill-internal.openapi.yaml`. Use either `-n` or `-f`, not both.

### Verbosity levels

- default: show `METHOD PATH` and `summary`.
- `-v`: add `operationId`, `tags`, and operation `description`.
- `-vv`: add parameter summary, request body content types, and response status codes.
- `-vvv`: add parameter detail plus expanded request/response schema details.

### More examples

```bash
oapi query -f ./openapi.json -q user -v
oapi query -f ./openapi.json -q order -vv
oapi query -f ./openapi.json -q payment -vvv --limit 20
```

### Search notes

- `-q` is optional.
- Without `-q`, the command lists endpoints in path order.
- `--limit` applies both to search results and the no-keyword listing mode.

## OpenAPI Generator recipes

### Verify generator is installed

```bash
openapi-generator version
```

### Validate a spec before generation

```bash
openapi-generator validate -i ./openapi.yaml
```

### List generators

```bash
openapi-generator list -s | tr ',' '\n'
```

### Inspect generator options

```bash
openapi-generator config-help -g go
```

### Generate code

```bash
openapi-generator generate -i ./openapi.yaml -g go -o ./out/go-client
```

## `oapi` call CLI

Purpose: directly call API endpoints with automatic parameter validation and request building.

### Basic call command

```bash
oapi call -f openapi.json -e "POST /cart/add" --params '{"item_id":"123","quantity":2}'
oapi call -f openapi.yaml -e "GET /users" --base-url https://api.example.com
oapi call -n skill -e "GET /users" --base-url https://api.example.com
```

Named specs follow the same `${OAPI_SPECS_DIR:-~/.openapi/specs}/<name>.openapi.yaml` rule as `query`; `-n` / `--name` and `-f` are mutually exclusive.

### Parameter validation

**Loose mode (default):**

```bash
oapi call -f openapi.json -e "POST /cart/add" --params '{"item_id":"123"}'
# Unknown parameters show warnings but request is sent
```

**Strict mode:**

```bash
oapi call -f openapi.json -e "POST /cart/add" --params '{"item_id":"123"}' --strict
# Unknown parameters cause validation failure
```

### Parameter sources

**From JSON string:**

```bash
oapi call -f openapi.json -e "POST /goods/list" --params '{"page":1,"page_size":20}'
```

**From JSON file:**

```bash
oapi call -f openapi.json -e "POST /cart/add" --params-file params.json
```

**From URL query string (PowerShell-friendly):**

```bash
oapi call -f openapi.json -e "POST /goods/list" --params-url "page=1&page_size=20&keyword=hello%20world"
```

**Path and query injection from JSON params:**

```bash
oapi call -f openapi.json -e "GET /workflow-runs/{runZid}" --params '{"runZid":"QVLR8V8DMVRG2VY2"}' --base-url https://api.example.com
oapi call -f openapi.json -e "GET /workflow-runs" --params '{"workflowDefinitionZid":"Z1Z645INZMQLILJ8","workflowVersionZid":"S2CHPKA1PLMLWV33","page":1,"pageSize":10}' --base-url https://api.example.com
```

**Bearer token and custom headers:**

```bash
oapi call -f openapi.json -e "GET /protected" --base-url https://api.example.com --bearer-token "$TOKEN"
oapi call -f openapi.json -e "GET /protected" --base-url https://api.example.com --header "X-Trace-Id: debug-123"
```

**Opt-in environment headers:**

```bash
OAPI_HEADER_X_API_KEY="$TOKEN" oapi call -n kb -e "GET /protected" --auto-headers
OAPI_AUTO_HEADERS=1 OAPI_HEADER_AUTHORIZATION="Bearer $TOKEN" oapi call -n iam -e "GET /protected"
```

**Repeated keys and legacy array-style names are preserved:**

```bash
oapi call -f openapi.json -e "GET /Api/v1/search/conditions" --params-url "order[]=status&order[]=admin_order&client[]=type"
```

### Request building

- **form-urlencoded**: Automatically builds `key=value&key2=value2` format
- **JSON**: Automatically marshals params to JSON
- **Path parameters**: Substitutes `{param}` in URL path
- **Query parameters**: Appends `?param=value` to URL and preserves repeated keys for array-like inputs
- **Headers**: Supports explicit request headers plus opt-in, OpenAPI-filtered `OAPI_HEADER_*` candidates

### Auth and session notes

- `--bearer-token` sets `Authorization: Bearer <token>`.
- `--header` is repeatable and can be used for headers like `X-Trace-Id`, custom tenant routing, or manual `Authorization` overrides.
- Do not combine `--bearer-token` with `--header "Authorization: ..."`; the CLI treats that as conflicting input.
- `--cookie` and `--cookie-path` remain the right tools for session-cookie flows.

### Automatic environment headers

- Default is off. Enable with `--auto-headers` or `OAPI_AUTO_HEADERS=1`; use `--auto-headers=false` to override an enabled environment.
- Only variables prefixed with `OAPI_HEADER_` are candidates. Names replace `_` with `-`, so `OAPI_HEADER_X_API_KEY` becomes `X-Api-Key`.
- The operation's effective `security` or `in: header` parameters must allow each injected header. A public operation with `security: []` does not inherit root authentication.
- Explicit CLI headers, bearer tokens, cookies, and content type win over environment values.
- Reserved headers, CR/LF values, and normalized-name collisions fail before the request is sent.
- Environment values are always redacted. Automatic headers cannot be sent to a different origin than an absolute spec server, and cross-origin redirects are rejected.

### Response output

- Default: formatted response body only
- `-v` and above: request preamble such as method/path and base URL
- Higher verbosity levels: more request inspection detail before the response

## Troubleshooting checklist

- `oapi bundle` cannot find a tool: install `redocly` or `swagger-cli`, or use `--tool npx-redocly` if Node tooling is available.
- `oapi doctor` fails on workspace checks: run `oapi init --dir ...` first, or restore the missing split-spec files/directories.
- `oapi fmt` rewrote more than you expected: review only `openapi.yaml`, `common/`, and business YAML files; it does not touch bundled artifacts.
- `oapi validate` or `oapi generate` cannot find a tool: install `openapi-generator`, or use `--tool npx-openapi-generator` if Node tooling is available.
- `oapi add ...` fails with duplicate errors: rerun with `--force` only when you intend to replace the generated skeleton/ref.
- `make install` fails writing `/usr/local/bin`: use `PREFIX=$HOME/.local`.
- No query result: verify keyword and file path (`-f`) are correct, or run `oapi query -f ./openapi.json` to inspect the available endpoints first.
- Invalid OpenAPI file: validate the file first; `oapi call` accepts JSON and YAML, but malformed input still fails early.
- Unexpected generator output: run `config-help -g <generator>` and tune options.

## Safety notes

- Do not include secrets in examples or generated configs.
- Keep one source of truth for the API contract.
- Review generated code before committing.
