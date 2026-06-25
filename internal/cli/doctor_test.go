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

func TestRun_Doctor_ReportsHealthyWorkspaceAndTools(t *testing.T) {
	workspace := initAuthoringWorkspace(t)
	if runtime.GOOS == "windows" {
		t.Skip("tool script test is unix-only")
	}

	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	writeExecutable(t, filepath.Join(binDir, "redocly"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "openapi-generator"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := Run([]string{"doctor", "--dir", workspace, "--bundle-tool", "redocly", "--generator-tool", "openapi-generator"}, &out, &errOut)
	if err != nil {
		t.Fatalf("doctor returned error: %v; stderr=%s; stdout=%s", err, errOut.String(), out.String())
	}

	output := out.String()
	checks := []string{
		"workspace: ok",
		"openapi entry: ok",
		"common dir: ok",
		"bundle tool: ok",
		"generator tool: ok",
		"status: ok",
	}
	for _, want := range checks {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in doctor output, got: %s", want, output)
		}
	}
}

func TestRun_Doctor_FailsForMissingWorkspaceBits(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "api", "openapi")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := Run([]string{"doctor", "--dir", workspace, "--bundle-tool", "missing-bundler", "--generator-tool", "missing-generator"}, &out, &errOut)
	if err == nil {
		t.Fatal("expected doctor to fail for missing workspace structure and tools")
	}

	output := out.String()
	checks := []string{
		"openapi entry: missing",
		"common dir: missing",
		"bundle tool: missing",
		"generator tool: missing",
		"status: failed",
	}
	for _, want := range checks {
		if !strings.Contains(output, want) {
			t.Fatalf("expected %q in doctor output, got: %s", want, output)
		}
	}
}

func TestRun_Doctor_JSONOutput(t *testing.T) {
	workspace := initAuthoringWorkspace(t)
	if runtime.GOOS == "windows" {
		t.Skip("tool script test is unix-only")
	}

	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	writeExecutable(t, filepath.Join(binDir, "redocly"), "#!/bin/sh\nexit 0\n")
	writeExecutable(t, filepath.Join(binDir, "openapi-generator"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := Run([]string{"doctor", "--dir", workspace, "--bundle-tool", "redocly", "--generator-tool", "openapi-generator", "--json"}, &out, &errOut)
	if err != nil {
		t.Fatalf("doctor returned error: %v; stderr=%s; stdout=%s", err, errOut.String(), out.String())
	}

	var report struct {
		Status string `json:"status"`
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
			Detail string `json:"detail"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal doctor json: %v\noutput=%s", err, out.String())
	}
	if report.Status != "ok" {
		t.Fatalf("expected ok status, got %#v", report)
	}
	if len(report.Checks) != 5 {
		t.Fatalf("expected 5 checks, got %#v", report)
	}
	if report.Checks[0].Name != "workspace" || report.Checks[0].Status != "ok" {
		t.Fatalf("unexpected first check: %#v", report.Checks[0])
	}
}
