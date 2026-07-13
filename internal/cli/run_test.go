package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func expectedQueryCallFlag() string {
	return queryCallParamFlag(runtime.GOOS)
}

func TestRun_QueryResolvesSpecNameFromConfiguredDirectory(t *testing.T) {
	specsDir := t.TempDir()
	specPath := filepath.Join(specsDir, "skill-internal.openapi.yaml")
	data, err := os.ReadFile("../../testdata/openapi.sample.yaml")
	if err != nil {
		t.Fatalf("read sample spec: %v", err)
	}
	if err := os.WriteFile(specPath, data, 0o644); err != nil {
		t.Fatalf("write named spec: %v", err)
	}
	t.Setenv("OAPI_SPECS_DIR", specsDir)

	var out bytes.Buffer
	var errOut bytes.Buffer
	err = Run([]string{
		"query",
		"--name", "skill-internal",
		"-q", "ping",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "GET /ping") {
		t.Fatalf("expected named spec to be queried, got: %s", output)
	}
	if !strings.Contains(output, "oapi call -n 'skill-internal' -e 'GET /ping'") {
		t.Fatalf("expected generated call example to preserve name selector, got: %s", output)
	}
}

func TestRun_QueryRejectsEmptyName(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	err := Run([]string{"query", "-n", ""}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "--name must not be empty") {
		t.Fatalf("expected empty name error, got: %v", err)
	}
}

func TestRun_QueryRejectsFileAndNameTogether(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	err := Run([]string{
		"query",
		"-f", "../../testdata/openapi.sample.json",
		"-n", "skill",
	}, &out, &errOut)
	if err == nil {
		t.Fatal("expected -f and -n to be mutually exclusive")
	}
	if !strings.Contains(err.Error(), "if any flags in the group") {
		t.Fatalf("expected mutual exclusion error, got: %v", err)
	}
}

func TestResolveSpecFile_DefaultDirectory(t *testing.T) {
	t.Setenv("OAPI_SPECS_DIR", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("resolve home directory: %v", err)
	}

	got, err := resolveSpecFile("", "skill")
	if err != nil {
		t.Fatalf("resolveSpecFile returned error: %v", err)
	}
	want := filepath.Join(home, ".openapi", "specs", "skill.openapi.yaml")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveSpecFile_RejectsPathLikeName(t *testing.T) {
	if _, err := resolveSpecFile("", "../skill"); err == nil {
		t.Fatal("expected path-like spec name to be rejected")
	}
}

func TestRun_DefaultVerbosityShowsOnlySummary(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"query",
		"-f", "../../testdata/openapi.sample.json",
		"-q", "user",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "GET /users") {
		t.Fatalf("expected path line in output, got: %s", output)
	}
	if !strings.Contains(output, "summary: List users") {
		t.Fatalf("expected summary in output, got: %s", output)
	}
	if !strings.Contains(output, "Call examples:") {
		t.Fatalf("expected call examples section in output, got: %s", output)
	}
	if !strings.Contains(output, "oapi call -f '../../testdata/openapi.sample.json' -e 'GET /users'") {
		t.Fatalf("expected oapi call command in output, got: %s", output)
	}
	for _, unwanted := range []string{
		"operationId:",
		"tags:",
		"params:",
		"requestBody:",
		"response:",
	} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("did not expect %q in default output, got: %s", unwanted, output)
		}
	}
}

func TestRun_VerboseLevel1ShowsOperationMetadata(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"query",
		"-f", "../../testdata/openapi.sample.json",
		"-q", "user",
		"-v",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "GET /users") {
		t.Fatalf("expected path line in output, got: %s", output)
	}
	if !strings.Contains(output, "List users") {
		t.Fatalf("expected summary in output, got: %s", output)
	}
	if !strings.Contains(output, "operationId: listUsers") {
		t.Fatalf("expected operationId at -v, got: %s", output)
	}
	if !strings.Contains(output, "tags: users") {
		t.Fatalf("expected tags at -v, got: %s", output)
	}
	if !strings.Contains(output, "description: Return paginated users") {
		t.Fatalf("expected description at -v, got: %s", output)
	}
	if strings.Contains(output, "params:") {
		t.Fatalf("did not expect params summary at -v, got: %s", output)
	}
}

func TestRun_VerboseLevel2ShowsContractOverview(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"query",
		"-f", "../../testdata/openapi.sample.json",
		"-q", "order",
		"-vv",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "operationId: getOrderById") {
		t.Fatalf("expected operationId at -vv, got: %s", output)
	}
	if !strings.Contains(output, "tags: orders") {
		t.Fatalf("expected tags at -vv, got: %s", output)
	}
	if !strings.Contains(output, "params: path:id(required)") {
		t.Fatalf("expected params summary at -vv, got: %s", output)
	}
	if !strings.Contains(output, "responses: 200, 404") {
		t.Fatalf("expected response summary at -vv, got: %s", output)
	}
	if expectedQueryCallFlag() == "--params" && !strings.Contains(output, `oapi call -f '../../testdata/openapi.sample.json' -e 'GET /orders/{id}' --params '{"id":"<id>"}'`) {
		t.Fatalf("expected call example with required path param, got: %s", output)
	}
	if expectedQueryCallFlag() == "--params-url" && !strings.Contains(output, `oapi call -f '../../testdata/openapi.sample.json' -e 'GET /orders/{id}' --params-url 'id=%3Cid%3E'`) {
		t.Fatalf("expected Windows-style call example with query params, got: %s", output)
	}
	if strings.Contains(output, "param detail:") {
		t.Fatalf("did not expect deep parameter detail at -vv, got: %s", output)
	}
}

func TestRun_VerboseLevel3ShowsDeepDetails(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"query",
		"-f", "../../testdata/openapi.sample.json",
		"-q", "users",
		"-vvv",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "param detail: query page required=false type=integer") {
		t.Fatalf("expected parameter detail at -vvv, got: %s", output)
	}
	if !strings.Contains(output, "response: 200 content=application/json schema=#/components/schemas/UserList") {
		t.Fatalf("expected response detail at -vvv, got: %s", output)
	}
}

func TestRun_VerboseLevel3ExpandsResolvedSchemaDetails(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"query",
		"-f", "../../testdata/openapi.verbose.sample.json",
		"-q", "report",
		"-vvv",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	output := out.String()
	checks := []string{
		"param detail: query source required=false type=string",
		"description: Source filter in query parameter",
		"example: scheduled",
		"deprecated: true",
		"allowEmptyValue: true",
		"style: form",
		"explode: false",
		"allowReserved: true",
		"param detail: query filters required=false type=unknown",
		"description: JSON filters encoded in query",
		"example[basic].summary: Basic filter",
		"example[basic].description: Single region selector",
		"example[basic]: {\"region\":\"north\"}",
		"example[empty]: {}",
		"example[remote].externalValue: https://example.com/examples/filters.json",
		"parameter content: application/json schema=object",
		"media example: map[region:north]",
		"example[inline]: map[region:north status:active]",
		"description: Filter payload",
		"param detail: header X-Trace-Id required=false type=string",
		"ref: #/components/parameters/TraceID",
		"description: Correlation id passed by gateway",
		"example: trace-123",
		"description: Report source",
		"example: manual",
		"enum: manual, scheduled",
		"request body: content=application/json schema=#/components/schemas/ReportQuery",
		"request body description: Request payload for report creation.",
		"schema required: source, limit",
		"property: limit type=integer required=true",
		"default: 100",
		"maximum: 1000",
		"response: 200 content=application/json schema=#/components/schemas/ReportResponse",
		"response description: Report created",
		"property: data type=object required=true ref=#/components/schemas/ReportData",
		"property: items type=array required=false",
	}

	for _, want := range checks {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in output, got: %s", want, output)
		}
	}

	if expectedQueryCallFlag() == "--params" && !strings.Contains(output, `oapi call -f '../../testdata/openapi.verbose.sample.json' -e 'POST /reports' --params '{"filters":"{\"region\":\"north\"}","limit":100,"source":"manual"}'`) {
		t.Fatalf("expected call example with defaults/examples for body and query params, got: %s", output)
	}
	if expectedQueryCallFlag() == "--params-url" && !strings.Contains(output, `oapi call -f '../../testdata/openapi.verbose.sample.json' -e 'POST /reports' --params-url 'filters=%7B%22region%22%3A%22north%22%7D&limit=100&source=manual'`) {
		t.Fatalf("expected Windows-style call example with defaults/examples for body and query params, got: %s", output)
	}
	if strings.Contains(output, `--params '{"X-Trace-Id"`) {
		t.Fatalf("did not expect unsupported header param in call example, got: %s", output)
	}
}

func TestQueryCallParamFlag_WindowsUsesParamsURL(t *testing.T) {
	if got := queryCallParamFlag("windows"); got != "--params-url" {
		t.Fatalf("expected --params-url for windows, got %q", got)
	}
	if got := queryCallParamFlag("darwin"); got != "--params" {
		t.Fatalf("expected --params for non-windows, got %q", got)
	}
}

func TestQueryCallGOOS_UsesEnvOverride(t *testing.T) {
	t.Setenv("OAPI_QUERY_CALL_GOOS", "windows")
	if got := queryCallGOOS(); got != "windows" {
		t.Fatalf("expected env override to force windows, got %q", got)
	}
}

func TestRun_QueryWindowsOverrideUsesWindowsQuoting(t *testing.T) {
	t.Setenv("OAPI_QUERY_CALL_GOOS", "windows")

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"query",
		"-f", "../../testdata/openapi.sample.json",
		"-q", "order",
		"-vv",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, `oapi call -f '../../testdata/openapi.sample.json' -e 'GET /orders/{id}' --params-url 'id=%3Cid%3E'`) {
		t.Fatalf("expected PowerShell-safe quoted call example, got: %s", output)
	}
	if !strings.Contains(output, `current keyword='order'`) {
		t.Fatalf("expected PowerShell-safe quoted keyword hint, got: %s", output)
	}
}

func TestPowerShellQuote_EscapesSingleQuotes(t *testing.T) {
	if got := powerShellQuote(`C:\Users\ann\O'Reilly\spec.json`); got != `'C:\Users\ann\O''Reilly\spec.json'` {
		t.Fatalf("expected PowerShell single-quote escaping, got %q", got)
	}
}

func TestRun_Limit(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"query",
		"-f", "../../testdata/openapi.sample.json",
		"-q", "get",
		"-v",
		"--limit", "1",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	output := out.String()
	if strings.Count(output, "\nGET /") != 1 {
		t.Fatalf("expected exactly one result with limit=1, got: %s", output)
	}
	if !strings.Contains(output, "Showing 1-1 of 2 endpoints (limit=1, offset=0)") {
		t.Fatalf("expected pagination summary, got: %s", output)
	}
	if !strings.Contains(output, "Next page: oapi query -f '../../testdata/openapi.sample.json' --limit 1 --offset 1 -q 'get'") {
		t.Fatalf("expected next page hint, got: %s", output)
	}
}

func TestRun_Offset(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"query",
		"-f", "../../testdata/openapi.sample.json",
		"--limit", "1",
		"--offset", "1",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "GET /users") {
		t.Fatalf("expected /users on second page, got: %s", output)
	}
	if strings.Contains(output, "GET /orders/{id}") {
		t.Fatalf("did not expect first page result on second page, got: %s", output)
	}
	if !strings.Contains(output, "Showing 2-2 of 2 endpoints (limit=1, offset=1)") {
		t.Fatalf("expected second-page summary, got: %s", output)
	}
	if !strings.Contains(output, "Previous page: oapi query -f '../../testdata/openapi.sample.json' --limit 1 --offset 0") {
		t.Fatalf("expected previous page hint, got: %s", output)
	}
	if !strings.Contains(output, "Tip: use -q <keyword> to narrow results") {
		t.Fatalf("expected search hint, got: %s", output)
	}
}

func TestRun_OffsetOutOfRangeShowsHint(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"query",
		"-f", "../../testdata/openapi.sample.json",
		"--limit", "1",
		"--offset", "10",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "no endpoints in current window (offset=10, limit=1, total=2)") {
		t.Fatalf("expected empty-window message, got: %s", output)
	}
	if !strings.Contains(output, "Previous page: oapi query -f '../../testdata/openapi.sample.json' --limit 1 --offset 9") {
		t.Fatalf("expected previous page hint for out-of-range window, got: %s", output)
	}
}

func TestRun_QueryJSON(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"query",
		"-f", "../../testdata/openapi.sample.json",
		"--json",
		"--limit", "1",
		"--offset", "1",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v; stderr=%s", err, errOut.String())
	}
	if strings.TrimSpace(errOut.String()) != "" {
		t.Fatalf("did not expect stderr output, got: %s", errOut.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v; output=%s", err, out.String())
	}

	page, ok := payload["page"].(map[string]any)
	if !ok {
		t.Fatalf("expected page object, got: %#v", payload["page"])
	}
	if got := int(page["total"].(float64)); got != 2 {
		t.Fatalf("expected total=2, got %d", got)
	}
	if got := int(page["offset"].(float64)); got != 1 {
		t.Fatalf("expected offset=1, got %d", got)
	}
	if got := int(page["count"].(float64)); got != 1 {
		t.Fatalf("expected count=1, got %d", got)
	}

	results, ok := payload["results"].([]any)
	if !ok || len(results) != 1 {
		t.Fatalf("expected one result, got %#v", payload["results"])
	}
	first := results[0].(map[string]any)
	if got := first["path"].(string); got != "/users" {
		t.Fatalf("expected /users on second page, got %s", got)
	}

	hints, ok := payload["hints"].(map[string]any)
	if !ok {
		t.Fatalf("expected hints object, got %#v", payload["hints"])
	}
	if !strings.Contains(hints["previousPageCommand"].(string), "--offset 0") {
		t.Fatalf("expected previous page command, got %#v", hints["previousPageCommand"])
	}
	if !strings.Contains(hints["searchTip"].(string), "use -q <keyword>") {
		t.Fatalf("expected search tip, got %#v", hints["searchTip"])
	}
}

func TestRun_QueryJSONWithKeyword(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"query",
		"-f", "../../testdata/openapi.sample.json",
		"-q", "order",
		"--json",
		"-v",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v; stderr=%s", err, errOut.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v; output=%s", err, out.String())
	}
	if got := payload["keyword"].(string); got != "order" {
		t.Fatalf("expected keyword=order, got %s", got)
	}
	results := payload["results"].([]any)
	first := results[0].(map[string]any)
	if got := first["operationId"].(string); got != "getOrderById" {
		t.Fatalf("expected operationId in verbose json, got %#v", first["operationId"])
	}
	hints := payload["hints"].(map[string]any)
	if !strings.Contains(hints["searchTip"].(string), "current keyword='order'") {
		t.Fatalf("expected keyword refinement hint, got %#v", hints["searchTip"])
	}
}

func TestRun_QueryWithoutKeywordListsAllEndpoints(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"query",
		"-f", "../../testdata/openapi.sample.json",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "GET /orders/{id}") {
		t.Fatalf("expected /orders/{id} in default list output, got: %s", output)
	}
	if !strings.Contains(output, "GET /users") {
		t.Fatalf("expected /users in default list output, got: %s", output)
	}
	if strings.Contains(output, "operationId: listUsers") {
		t.Fatalf("did not expect verbosity level 1 output by default, got: %s", output)
	}
}

func TestRun_Call_RequiresEndpoint(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", "../../testdata/openapi.sample.json",
	}, &out, &errOut)
	if err == nil {
		t.Fatalf("expected error when calling without -e")
	}

	if !strings.Contains(err.Error(), "endpoint is required") {
		t.Fatalf("expected endpoint error, got: %v", err)
	}
}
