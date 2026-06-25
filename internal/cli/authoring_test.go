package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	workspacepkg "openapi/internal/workspace"
)

func TestRun_Init_CreatesSplitWorkspace(t *testing.T) {
	tempDir := t.TempDir()
	workspace := filepath.Join(tempDir, "api", "openapi")

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := Run([]string{"init", "--dir", workspace, "--title", "Experts Backend API", "--version", "0.1.0"}, &out, &errOut); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	for _, path := range []string{
		filepath.Join(workspace, workspacepkg.SourceEntryName),
		filepath.Join(workspace, "common"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	root := readFile(t, filepath.Join(workspace, workspacepkg.SourceEntryName))
	checks := []string{
		"openapi: 3.0.3",
		"title: Experts Backend API",
		"version: 0.1.0",
		"paths: {}",
		"schemas: {}",
		"parameters: {}",
		"responses: {}",
	}
	for _, want := range checks {
		if !strings.Contains(root, want) {
			t.Fatalf("expected %q in index.yaml, got: %s", want, root)
		}
	}
}

func TestRun_AddPath_CreatesTemplateAndSortedRef(t *testing.T) {
	workspace := initAuthoringWorkspace(t)

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := Run([]string{"add", "path", "--dir", workspace, "--business", "workflow", "--path", "/workflow-runs"}, &out, &errOut); err != nil {
		t.Fatalf("add path returned error: %v", err)
	}
	if err := Run([]string{"add", "path", "--dir", workspace, "--business", "workflow", "--path", "/workflow-runs/{runZid}"}, &out, &errOut); err != nil {
		t.Fatalf("add nested path returned error: %v", err)
	}

	pathFile := readFile(t, filepath.Join(workspace, "workflow", "paths.yaml"))
	if !strings.Contains(pathFile, "summary: TODO") || !strings.Contains(pathFile, "operationId: todoOperation") {
		t.Fatalf("expected path template in generated file, got: %s", pathFile)
	}
	if !strings.Contains(pathFile, "/workflow-runs:") || !strings.Contains(pathFile, "/workflow-runs/{runZid}:") {
		t.Fatalf("expected grouped path keys in generated file, got: %s", pathFile)
	}

	root := readFile(t, filepath.Join(workspace, workspacepkg.SourceEntryName))
	first := strings.Index(root, "/workflow-runs:")
	second := strings.Index(root, "/workflow-runs/{runZid}:")
	if first == -1 || second == -1 || first > second {
		t.Fatalf("expected sorted paths in index.yaml, got: %s", root)
	}
	if !strings.Contains(root, "$ref: ./workflow/paths.yaml#/~1workflow-runs") {
		t.Fatalf("expected root path ref, got: %s", root)
	}
	if !strings.Contains(root, "$ref: ./workflow/paths.yaml#/~1workflow-runs~1{runZid}") {
		t.Fatalf("expected inferred nested path ref, got: %s", root)
	}
}

func TestRun_AddSchemaParameterResponse_UpdatesComponents(t *testing.T) {
	workspace := initAuthoringWorkspace(t)

	var out bytes.Buffer
	var errOut bytes.Buffer
	commands := [][]string{
		{"add", "schema", "--dir", workspace, "--business", "common", "--file", "common", "--name", "ErrorResponse", "--kind", "object"},
		{"add", "parameter", "--dir", workspace, "--business", "common", "--file", "pagination", "--name", "Page", "--in", "query"},
		{"add", "response", "--dir", workspace, "--business", "common", "--file", "errors", "--name", "Error"},
		{"add", "schema", "--dir", workspace, "--business", "workflow", "--name", "WorkflowRun", "--kind", "object"},
		{"add", "parameter", "--dir", workspace, "--business", "workflow", "--name", "RunZid", "--in", "path"},
		{"add", "response", "--dir", workspace, "--business", "workflow", "--name", "WorkflowError"},
	}
	for _, command := range commands {
		if err := Run(command, &out, &errOut); err != nil {
			t.Fatalf("command %v returned error: %v", command, err)
		}
	}

	schemaFile := readFile(t, filepath.Join(workspace, "common", "common.yaml"))
	if !strings.Contains(schemaFile, "ErrorResponse:") || !strings.Contains(schemaFile, "type: object") {
		t.Fatalf("expected schema template, got: %s", schemaFile)
	}
	parameterFile := readFile(t, filepath.Join(workspace, "common", "pagination.yaml"))
	if !strings.Contains(parameterFile, "name: page") || !strings.Contains(parameterFile, "type: integer") {
		t.Fatalf("expected parameter template, got: %s", parameterFile)
	}
	responseFile := readFile(t, filepath.Join(workspace, "common", "errors.yaml"))
	if !strings.Contains(responseFile, "Error:") || !strings.Contains(responseFile, "description: TODO") {
		t.Fatalf("expected response template, got: %s", responseFile)
	}
	workflowSchemaFile := readFile(t, filepath.Join(workspace, "workflow", "schemas.yaml"))
	if !strings.Contains(workflowSchemaFile, "WorkflowRun:") {
		t.Fatalf("expected business schema template, got: %s", workflowSchemaFile)
	}
	workflowParameterFile := readFile(t, filepath.Join(workspace, "workflow", "parameters.yaml"))
	if !strings.Contains(workflowParameterFile, "RunZid:") || !strings.Contains(workflowParameterFile, "required: true") {
		t.Fatalf("expected business parameter template, got: %s", workflowParameterFile)
	}
	workflowResponseFile := readFile(t, filepath.Join(workspace, "workflow", "responses.yaml"))
	if !strings.Contains(workflowResponseFile, "WorkflowError:") {
		t.Fatalf("expected business response template, got: %s", workflowResponseFile)
	}

	root := readFile(t, filepath.Join(workspace, workspacepkg.SourceEntryName))
	checks := []string{
		"ErrorResponse:",
		"$ref: ./common/common.yaml#/ErrorResponse",
		"Page:",
		"$ref: ./common/pagination.yaml#/Page",
		"Error:",
		"$ref: ./common/errors.yaml#/Error",
		"WorkflowRun:",
		"$ref: ./workflow/schemas.yaml#/WorkflowRun",
		"RunZid:",
		"$ref: ./workflow/parameters.yaml#/RunZid",
		"WorkflowError:",
		"$ref: ./workflow/responses.yaml#/WorkflowError",
	}
	for _, want := range checks {
		if !strings.Contains(root, want) {
			t.Fatalf("expected %q in index.yaml, got: %s", want, root)
		}
	}
}

func TestRun_AddPath_RejectsDuplicateWithoutForce(t *testing.T) {
	workspace := initAuthoringWorkspace(t)

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := Run([]string{"add", "path", "--dir", workspace, "--business", "workflow", "--path", "/workflow-runs", "--name", "workflow-runs"}, &out, &errOut); err != nil {
		t.Fatalf("first add path returned error: %v", err)
	}
	if err := Run([]string{"add", "path", "--dir", workspace, "--business", "workflow", "--path", "/workflow-runs", "--name", "workflow-runs"}, &out, &errOut); err == nil {
		t.Fatal("expected duplicate path add to fail without --force")
	}
}

func TestRun_Bundle_UsesExternalTool(t *testing.T) {
	workspace := initAuthoringWorkspace(t)
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	toolPath := filepath.Join(binDir, "redocly")
	script := "#!/bin/sh\nset -eu\ninput=$2\nout=$4\ncp \"$input\" \"$out\"\n"
	if runtime.GOOS == "windows" {
		t.Skip("bundle tool script test is unix-only")
	}
	if err := os.WriteFile(toolPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write tool script: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	outPath := filepath.Join(workspace, "dist", "openapi.yaml")
	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := Run([]string{"bundle", "--dir", workspace, "--out", outPath, "--tool", "redocly"}, &out, &errOut); err != nil {
		t.Fatalf("bundle returned error: %v, stderr=%s", err, errOut.String())
	}

	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected bundled file at %s: %v", outPath, err)
	}
	content := readFile(t, outPath)
	if !strings.Contains(content, "openapi: 3.0.3") {
		t.Fatalf("expected bundled output to copy input, got: %s", content)
	}
}

func TestRun_Validate_BundlesThenRunsOpenAPIGenerator(t *testing.T) {
	workspace := initAuthoringWorkspace(t)
	if runtime.GOOS == "windows" {
		t.Skip("tool script test is unix-only")
	}

	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "validate.log")
	writeExecutable(t, filepath.Join(binDir, "redocly"), "#!/bin/sh\nset -eu\ninput=$2\nout=$4\ncp \"$input\" \"$out\"\n")
	writeExecutable(t, filepath.Join(binDir, "openapi-generator"), "#!/bin/sh\nset -eu\nprintf '%s\n' \"$*\" > \"$OAPI_TEST_LOG\"\nif [ \"$1\" != \"validate\" ]; then\n  echo unexpected command >&2\n  exit 1\nfi\ncase \"$*\" in\n  *'-i '*'.yaml'*) ;;\n  *) echo missing input >&2; exit 1 ;;\nesac\n")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("OAPI_TEST_LOG", logPath)

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := Run([]string{"validate", "--dir", workspace, "--tool", "openapi-generator", "--bundle-tool", "redocly"}, &out, &errOut); err != nil {
		t.Fatalf("validate returned error: %v, stderr=%s", err, errOut.String())
	}

	logged := readFile(t, logPath)
	if !strings.Contains(logged, "validate -i ") {
		t.Fatalf("expected validate command to be logged, got: %s", logged)
	}
	if !strings.Contains(out.String(), "validated ") {
		t.Fatalf("expected success output, got: %s", out.String())
	}
	if strings.Contains(out.String(), workspace+string(os.PathSeparator)+"dist") {
		t.Fatalf("did not expect validate to write to a dist artifact, got: %s", out.String())
	}
}

func TestRun_Generate_BundlesThenRunsOpenAPIGenerator(t *testing.T) {
	workspace := initAuthoringWorkspace(t)
	if runtime.GOOS == "windows" {
		t.Skip("tool script test is unix-only")
	}

	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	logPath := filepath.Join(t.TempDir(), "generate.log")
	writeExecutable(t, filepath.Join(binDir, "redocly"), "#!/bin/sh\nset -eu\ninput=$2\nout=$4\ncp \"$input\" \"$out\"\n")
	writeExecutable(t, filepath.Join(binDir, "openapi-generator"), "#!/bin/sh\nset -eu\nprintf '%s\n' \"$*\" > \"$OAPI_TEST_LOG\"\nout=''\nwhile [ $# -gt 0 ]; do\n  if [ \"$1\" = \"-o\" ]; then\n    out=$2\n    break\n  fi\n  shift\ndone\nif [ -z \"$out\" ]; then\n  echo missing output >&2\n  exit 1\nfi\nmkdir -p \"$out\"\nprintf 'generated' > \"$out/result.txt\"\n")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("OAPI_TEST_LOG", logPath)

	outDir := filepath.Join(t.TempDir(), "generated", "go")
	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := Run([]string{"generate", "--dir", workspace, "--lang", "go", "--out", outDir, "--tool", "openapi-generator", "--bundle-tool", "redocly", "--additional-property", "packageName=experts", "--global-property", "models"}, &out, &errOut); err != nil {
		t.Fatalf("generate returned error: %v, stderr=%s", err, errOut.String())
	}

	logged := readFile(t, logPath)
	for _, want := range []string{
		"generate -g go",
		"-o " + outDir,
		"--additional-properties packageName=experts",
		"--global-property models",
	} {
		if !strings.Contains(logged, want) {
			t.Fatalf("expected %q in logged generator command, got: %s", want, logged)
		}
	}
	if _, err := os.Stat(filepath.Join(outDir, "result.txt")); err != nil {
		t.Fatalf("expected generated output file: %v", err)
	}
	if !strings.Contains(out.String(), "generated go client") {
		t.Fatalf("expected success output, got: %s", out.String())
	}
}

func TestRun_Fmt_SortsAndReformatsSplitWorkspaceYAML(t *testing.T) {
	workspace := initAuthoringWorkspace(t)

	writeFile(t, filepath.Join(workspace, workspacepkg.SourceEntryName), "paths:\n  /zeta:\n    $ref: ./workflow/paths.yaml#/~1zeta\n  /alpha:\n    $ref: ./workflow/paths.yaml#/~1alpha\ncomponents:\n  schemas:\n    Zed: {$ref: ./workflow/schemas.yaml#/Zed}\n    Alpha: {$ref: ./workflow/schemas.yaml#/Alpha}\nopenapi: 3.0.3\ninfo: {version: 1.0.0, title: Test API}\n")
	writeFile(t, filepath.Join(workspace, "workflow", "paths.yaml"), "/zeta:\n  post:\n    responses:\n      \"400\": {description: Bad Request}\n      \"200\": {description: OK}\n    summary: Create user\n    operationId: createUser\n/alpha:\n  get:\n    summary: List users\n    responses:\n      \"404\": {description: Missing}\n      \"200\": {description: OK}\n    operationId: listUsers\n")
	writeFile(t, filepath.Join(workspace, "workflow", "schemas.yaml"), "Zed:\n  properties: {zeta: {type: string}, alpha: {type: string}}\n  type: object\nAlpha:\n  type: object\n  properties: {beta: {type: string}, alpha: {type: string}}\n")

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := Run([]string{"fmt", "--dir", workspace}, &out, &errOut); err != nil {
		t.Fatalf("fmt returned error: %v; stderr=%s; stdout=%s", err, errOut.String(), out.String())
	}

	root := readFile(t, filepath.Join(workspace, workspacepkg.SourceEntryName))
	if strings.Index(root, "Alpha:") > strings.Index(root, "Zed:") {
		t.Fatalf("expected schemas to be sorted in root, got: %s", root)
	}
	if strings.Index(root, "/alpha:") > strings.Index(root, "/zeta:") {
		t.Fatalf("expected paths to be sorted in root, got: %s", root)
	}
	if !strings.Contains(root, "info:\n  title: Test API\n  version: 1.0.0\n") {
		t.Fatalf("expected root info mapping to be reformatted, got: %s", root)
	}

	pathFile := readFile(t, filepath.Join(workspace, "workflow", "paths.yaml"))
	if strings.Index(pathFile, "/alpha:") > strings.Index(pathFile, "/zeta:") {
		t.Fatalf("expected path keys to be sorted, got: %s", pathFile)
	}
	if strings.Index(pathFile, "get:") > strings.Index(pathFile, "post:") {
		t.Fatalf("expected path operations to be sorted, got: %s", pathFile)
	}
	if strings.Index(pathFile, "\"200\":") > strings.Index(pathFile, "\"404\":") {
		t.Fatalf("expected response codes to be sorted, got: %s", pathFile)
	}
	if !strings.Contains(pathFile, "responses:\n      \"200\":\n        description: OK\n") {
		t.Fatalf("expected path file inline mappings to be expanded, got: %s", pathFile)
	}

	schemaFile := readFile(t, filepath.Join(workspace, "workflow", "schemas.yaml"))
	if strings.Index(schemaFile, "Alpha:") > strings.Index(schemaFile, "Zed:") {
		t.Fatalf("expected schema definitions to be sorted, got: %s", schemaFile)
	}
	if strings.Index(schemaFile, "alpha:") > strings.Index(schemaFile, "beta:") {
		t.Fatalf("expected nested properties to be sorted, got: %s", schemaFile)
	}
	if !strings.Contains(out.String(), "formatted ") {
		t.Fatalf("expected fmt success output, got: %s", out.String())
	}
}

func initAuthoringWorkspace(t *testing.T) string {
	t.Helper()
	workspace := filepath.Join(t.TempDir(), "api", "openapi")
	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := Run([]string{"init", "--dir", workspace, "--title", "Test API", "--version", "0.1.0"}, &out, &errOut); err != nil {
		t.Fatalf("init returned error: %v", err)
	}
	return workspace
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func writeExecutable(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
