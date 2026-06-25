package doctor

import (
	"fmt"
	"os"
	"path/filepath"

	"openapi/internal/bundle"
	"openapi/internal/generate"
	"openapi/internal/workspace"
)

type Options struct {
	Dir           string
	BundleTool    string
	GeneratorTool string
}

type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type Report struct {
	Status string  `json:"status"`
	Checks []Check `json:"checks"`
}

func (r Report) HasFailures() bool {
	return r.Status != "ok"
}

func Run(opts Options) Report {
	resolvedDir := filepath.Clean(opts.Dir)
	checks := []Check{
		checkDir("workspace", resolvedDir),
		checkFile("openapi entry", workspace.SourceEntryPath(resolvedDir)),
		checkDir("common dir", filepath.Join(resolvedDir, "common")),
		checkBundleTool(opts.BundleTool),
		checkGeneratorTool(opts.GeneratorTool),
	}
	return Report{Status: summarizeStatus(checks), Checks: checks}
}

func summarizeStatus(checks []Check) string {
	for _, check := range checks {
		if check.Status != "ok" {
			return "failed"
		}
	}
	return "ok"
}

func checkDir(name string, path string) Check {
	info, err := os.Stat(path)
	if err != nil {
		return Check{Name: name, Status: "missing", Detail: path}
	}
	if !info.IsDir() {
		return Check{Name: name, Status: "invalid", Detail: fmt.Sprintf("expected directory: %s", path)}
	}
	return Check{Name: name, Status: "ok", Detail: path}
}

func checkFile(name string, path string) Check {
	info, err := os.Stat(path)
	if err != nil {
		return Check{Name: name, Status: "missing", Detail: path}
	}
	if info.IsDir() {
		return Check{Name: name, Status: "invalid", Detail: fmt.Sprintf("expected file: %s", path)}
	}
	return Check{Name: name, Status: "ok", Detail: path}
}

func checkBundleTool(tool string) Check {
	selection, err := bundle.DetectTool(tool)
	if err != nil {
		return Check{Name: "bundle tool", Status: "missing", Detail: err.Error()}
	}
	return Check{Name: "bundle tool", Status: "ok", Detail: selection.Label}
}

func checkGeneratorTool(tool string) Check {
	selection, err := generate.DetectTool(tool)
	if err != nil {
		return Check{Name: "generator tool", Status: "missing", Detail: err.Error()}
	}
	return Check{Name: "generator tool", Status: "ok", Detail: selection.Label}
}
