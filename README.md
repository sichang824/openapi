# OpenAPI Skill + oapi CLI

`oapi` is a local Go CLI for split-spec authoring, OpenAPI review, endpoint discovery, and direct API calls. This repository also ships the AI skill and curated reference material that document the intended workflows.

## What you get

- Split-spec authoring commands: `init`, `add`, `fmt`
- Orchestration commands: `bundle`, `doctor`, `validate`, `generate`
- Inspection commands: `query`, `call`
- OpenAPI references under `assets/openapi-docs/`
- Agent guidance in `SKILL.md`

## Quick start

```bash
make build-cli
make test
make install BIN_DIR=$HOME/.local/bin
```

Then inspect the live CLI help:

```bash
oapi --help
oapi query --help
oapi call --help
```

## Core workflows

### Author a split spec

```bash
oapi init --dir ./api/openapi --title "Experts Backend API" --version 0.1.0
oapi add path --dir ./api/openapi --business workflow --path /workflow-runs
oapi add schema --dir ./api/openapi --business common --file common --name ErrorResponse --kind object
oapi add parameter --dir ./api/openapi --business common --file pagination --name Page --in query
oapi add response --dir ./api/openapi --business common --file errors --name Error
oapi add schema --dir ./api/openapi --business workflow --name WorkflowRun --kind object
oapi fmt --dir ./api/openapi
oapi bundle --dir ./api/openapi --out ./api/openapi/dist/openapi.yaml
oapi doctor --dir ./api/openapi --json
oapi validate --dir ./api/openapi
oapi generate --dir ./api/openapi --lang go --out ./internal/sdk/generated/openapi
```

The split workspace source entry is `./api/openapi/index.yaml`. Bundled artifacts should go to paths like `./api/openapi/dist/openapi.yaml`.

### Inspect a bundled or standalone spec

```bash
oapi query -f ./openapi.json
oapi query -n skill -q workflow -vv
oapi query --name skill-internal -q report -vvv --limit 20
oapi query -f ./openapi.json -q workflow -vv
oapi query -f ./openapi.json -q report -vvv --limit 20
```

`-n` / `--name` resolves `<name>.openapi.yaml` from `${OAPI_SPECS_DIR:-~/.openapi/specs}`. It is mutually exclusive with `-f`. `-q` is optional, and higher verbosity expands more contract detail.

### Call documented endpoints

```bash
# YAML or JSON spec input
oapi call -f ./openapi.yaml -e "GET /users" --base-url https://api.example.com
oapi call -n skill -e "GET /users" --base-url https://api.example.com

# JSON params or params file
oapi call -f ./openapi.json -e "POST /cart/add" --params '{"item_id":"123","quantity":2}'
oapi call -f ./openapi.json -e "POST /goods/list" --params-file params.json

# Path and query parameter injection
oapi call -f ./openapi.json -e "GET /workflow-runs/{runZid}" --params '{"runZid":"QVLR8V8DMVRG2VY2"}' --base-url https://api.example.com
oapi call -f ./openapi.json -e "GET /workflow-runs" --params '{"workflowDefinitionZid":"Z1Z645INZMQLILJ8","workflowVersionZid":"S2CHPKA1PLMLWV33","page":1,"pageSize":10}' --base-url https://api.example.com

# Header and auth injection
oapi call -f ./openapi.json -e "GET /protected" --base-url https://api.example.com --bearer-token "$TOKEN"
oapi call -f ./openapi.json -e "GET /protected" --base-url https://api.example.com --header "X-Trace-Id: debug-123"

# Stream a response body to a file
oapi call -f ./openapi.yaml -e "GET /files/{id}" --base-url https://api.example.com --params '{"id":"file-abc"}' -o ./download.bin

# Opt-in environment headers, filtered by the operation's OpenAPI contract
OAPI_HEADER_X_API_KEY="$TOKEN" oapi call -n kb -e "GET /protected" --auto-headers
OAPI_AUTO_HEADERS=1 OAPI_HEADER_AUTHORIZATION="Bearer $TOKEN" oapi call -n iam -e "GET /protected"
```

`call` supports JSON and YAML specs, `--params` / `--params-file` / `--params-url` are mutually exclusive, and `--strict` upgrades warnings into validation failures. `-o` / `--output` streams the raw response body to a file; stdout stays empty, while verbose metadata goes to stderr. Automatic headers are disabled by default. When enabled, only `OAPI_HEADER_*` candidates allowed by effective OpenAPI security or header parameters are sent; explicit CLI values win.

## Repository map

- `cmd/oapi/`: CLI entrypoint
- `internal/spec/`: spec loading, JSON/YAML decoding, typed structures
- `internal/query/`: endpoint search and ranking
- `internal/validator/`: parameter validation
- `internal/autoheaders/`: opt-in environment parsing, contract selection, and origin checks
- `internal/caller/`: request construction and execution
- `internal/scaffold/`, `internal/edit/`: split-spec authoring and formatting
- `internal/bundle/`, `internal/generate/`, `internal/doctor/`: orchestration helpers
- `HELP.md`: operator-facing quick reference
- `SKILL.md`: AI skill contract

## Notes for contributors

- Keep `SKILL.md`, `README.md`, `HELP.md`, and Cobra help text aligned.
- Keep examples executable and copy-paste ready.
- Add tests first for CLI behavior changes.
- `spec.Load` accepts JSON and YAML.
- `oapi call` supports path-item-level parameter inheritance, explicit auth/session inputs, and opt-in contract-filtered environment headers.
