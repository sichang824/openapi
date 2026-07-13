package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun_Help_ShowsGroupedCobraRootHelp(t *testing.T) {
	root := newRootCommand(ioDiscard{}, ioDiscard{})
	if root.Short != "Split OpenAPI authoring, validation, generation, and inspection CLI" {
		t.Fatalf("unexpected root short help: %q", root.Short)
	}
	if !strings.Contains(root.Example, "oapi fmt --dir ./api/openapi") {
		t.Fatalf("expected fmt example in root metadata, got: %s", root.Example)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer

	if err := Run([]string{"--help"}, &out, &errOut); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	help := out.String()
	checks := []string{
		"oapi helps maintain a split OpenAPI workspace without hand-editing",
		"Quick Start:",
		"Common Paths:",
		"Authoring Commands:",
		"Scaffold and maintain split-spec files.",
		"Format split-spec YAML files deterministically",
		"Orchestration Commands:",
		"Inspection Commands:",
		"oapi add path --help",
		"oapi query -f ./openapi.json -q workflow -vv",
	}
	for _, want := range checks {
		if !strings.Contains(help, want) {
			t.Fatalf("expected %q in help output, got: %s", want, help)
		}
	}
}

func TestRun_Help_ShowsAddPathExamples(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	if err := Run([]string{"add", "path", "--help"}, &out, &errOut); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	help := out.String()
	checks := []string{
		"Add a path item under <business>/paths.yaml and wire the matching",
		"Use --business to choose which business file owns the path item.",
		"Examples:",
		"oapi add path --dir ./api/openapi --business workflow --path /workflow-runs",
		"oapi add path --dir ./api/openapi --business workflow --path /workflow-runs/{runZid}",
		"Flags:",
		"--business string",
		"--path string",
	}
	for _, want := range checks {
		if !strings.Contains(help, want) {
			t.Fatalf("expected %q in add path help output, got: %s", want, help)
		}
	}
}

func TestRun_Help_ShowsAddGroupedHelp(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	if err := Run([]string{"add", "--help"}, &out, &errOut); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	help := out.String()
	checks := []string{
		"Add creates minimal split-spec files and updates the root aggregator",
		"--business common writes shared component files under common/*.yaml",
		"--business <name> writes business files under <name>/paths.yaml, <name>/schemas.yaml, <name>/parameters.yaml, or <name>/responses.yaml",
		"Path Commands:",
		"Create path-item files and wire path refs.",
		"Component Commands:",
		"Create reusable schema, parameter, and response components.",
		"path        Add a path item template and root ref",
		"schema      Add a schema template and root ref",
	}
	for _, want := range checks {
		if !strings.Contains(help, want) {
			t.Fatalf("expected %q in add help output, got: %s", want, help)
		}
	}
}

func TestRun_Help_ShowsAddSchemaLayoutRule(t *testing.T) {
	assertHelpContains(t, []string{"add", "schema", "--help"}, []string{
		"Use --business common for shared files like common/common.yaml.",
		"Use --business <name> for business-local definitions in <name>/schemas.yaml.",
		"common for shared files, or a business directory like workflow",
		"file name under common/; ignored for business-local files",
	})
}

func TestRun_Help_ShowsAddPathLayoutRule(t *testing.T) {
	assertHelpContains(t, []string{"add", "path", "--help"}, []string{
		"Add a path item under <business>/paths.yaml",
		"business directory that owns this path file",
	})
}

func TestRun_Help_ShowsQueryLongBoundaries(t *testing.T) {
	assertHelpContains(t, []string{"query", "--help"}, []string{
		"Search a large OpenAPI document without opening it by hand.",
		"-q is optional",
		"query reads a single bundled or standalone spec file",
	})
}

func TestRun_Help_ShowsQuerySpecFileFlagText(t *testing.T) {
	assertHelpContains(t, []string{"query", "--help"}, []string{
		"OpenAPI spec file",
		"-n, --name string",
		"OAPI_SPECS_DIR",
	})
}

func TestRun_Help_ShowsCallSpecNameFlag(t *testing.T) {
	assertHelpContains(t, []string{"call", "--help"}, []string{
		"-n, --name string",
		"OAPI_SPECS_DIR",
		"--auto-headers",
		"OAPI_HEADER_*",
	})
}

func TestRun_Help_ShowsCallLongBoundaries(t *testing.T) {
	assertHelpContains(t, []string{"call", "--help"}, []string{
		"Call a documented endpoint directly from the CLI with spec-aware",
		"pass exactly one of --params, --params-file, or --params-url",
		"POST/PUT/PATCH/DELETE can have real side effects",
	})
}

func TestRun_Help_ShowsGenerateLongBoundaries(t *testing.T) {
	assertHelpContains(t, []string{"generate", "--help"}, []string{
		"Generate code from a split OpenAPI workspace by bundling to a temporary",
		"generate does not point openapi-generator directly at the split tree",
		"--lang and --out are required",
	})
}

func TestRun_Help_ShowsDoctorLongBoundaries(t *testing.T) {
	assertHelpContains(t, []string{"doctor", "--help"}, []string{
		"Doctor inspects the split OpenAPI workspace and the external tools that",
		"doctor checks workspace shape and tool resolution",
		"oapi doctor --dir ./api/openapi",
		"oapi doctor --dir ./api/openapi --json",
	})
}

func TestRun_Help_ShowsFmtLongBoundaries(t *testing.T) {
	assertHelpContains(t, []string{"fmt", "--help"}, []string{
		"Format rewrites the split OpenAPI workspace into a stable YAML layout.",
		"fmt only rewrites index.yaml and the tracked split YAML files",
		"oapi fmt --dir ./api/openapi",
	})
}

func assertHelpContains(t *testing.T, args []string, checks []string) {
	t.Helper()

	var out bytes.Buffer
	var errOut bytes.Buffer
	if err := Run(args, &out, &errOut); err != nil {
		t.Fatalf("Run returned error for %v: %v", args, err)
	}
	help := out.String()
	for _, want := range checks {
		if !strings.Contains(help, want) {
			t.Fatalf("expected %q in help output for %v, got: %s", want, args, help)
		}
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}
