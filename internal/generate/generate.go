package generate

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"openapi/internal/bundle"
)

type ValidateOptions struct {
	Dir        string
	Tool       string
	BundleTool string
	Stdout     io.Writer
	Stderr     io.Writer
}

type GenerateOptions struct {
	Dir                  string
	Lang                 string
	Out                  string
	Tool                 string
	BundleTool           string
	Config               string
	AdditionalProperties []string
	GlobalProperties     []string
	Stdout               io.Writer
	Stderr               io.Writer
}

type runner struct {
	label string
	name  string
	args  func(subcommand string, extra []string) []string
}

type ToolSelection struct {
	Key     string
	Label   string
	Command string
}

var toolRegistry = map[string]runner{
	"openapi-generator": {
		label: "openapi-generator",
		name:  "openapi-generator",
		args: func(subcommand string, extra []string) []string {
			return append([]string{subcommand}, extra...)
		},
	},
	"npx-openapi-generator": {
		label: "npx @openapitools/openapi-generator",
		name:  "npx",
		args: func(subcommand string, extra []string) []string {
			return append([]string{"@openapitools/openapi-generator", subcommand}, extra...)
		},
	},
}

func Validate(opts ValidateOptions) error {
	_, run, err := resolveRunner(strings.TrimSpace(opts.Tool))
	if err != nil {
		return err
	}

	return withBundledSpec(opts.Dir, strings.TrimSpace(opts.BundleTool), opts.Stdout, opts.Stderr, func(specPath string) error {
		args := run.args("validate", []string{"-i", specPath})
		return execute(run.name, args, opts.Stdout, opts.Stderr, "validate")
	})
}

func Generate(opts GenerateOptions) error {
	if strings.TrimSpace(opts.Lang) == "" {
		return fmt.Errorf("generate language is required")
	}
	if strings.TrimSpace(opts.Out) == "" {
		return fmt.Errorf("generate output is required")
	}
	_, run, err := resolveRunner(strings.TrimSpace(opts.Tool))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(opts.Out, 0o755); err != nil {
		return err
	}

	return withBundledSpec(opts.Dir, strings.TrimSpace(opts.BundleTool), opts.Stdout, opts.Stderr, func(specPath string) error {
		args := []string{"-g", opts.Lang, "-i", specPath, "-o", opts.Out}
		if strings.TrimSpace(opts.Config) != "" {
			args = append(args, "-c", opts.Config)
		}
		for _, property := range opts.AdditionalProperties {
			if strings.TrimSpace(property) == "" {
				continue
			}
			args = append(args, "--additional-properties", property)
		}
		for _, property := range opts.GlobalProperties {
			if strings.TrimSpace(property) == "" {
				continue
			}
			args = append(args, "--global-property", property)
		}
		return execute(run.name, run.args("generate", args), opts.Stdout, opts.Stderr, "generate")
	})
}

func withBundledSpec(dir string, bundleTool string, stdout io.Writer, stderr io.Writer, fn func(specPath string) error) error {
	resolvedDir := filepath.Clean(dir)
	tempDir, err := os.MkdirTemp("", "oapi-bundle-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	bundledPath := filepath.Join(tempDir, "openapi.yaml")
	if err := bundle.Run(bundle.Options{
		Dir:    resolvedDir,
		Out:    bundledPath,
		Tool:   bundleTool,
		Stdout: stdout,
		Stderr: stderr,
	}); err != nil {
		return err
	}
	return fn(bundledPath)
}

func DetectTool(tool string) (ToolSelection, error) {
	key, selected, err := resolveRunner(strings.TrimSpace(tool))
	if err != nil {
		return ToolSelection{}, err
	}
	return ToolSelection{Key: key, Label: selected.label, Command: selected.name}, nil
}

func resolveRunner(tool string) (string, runner, error) {
	if tool != "" && tool != "auto" {
		selected, ok := toolRegistry[tool]
		if !ok {
			return "", runner{}, fmt.Errorf("unsupported generator tool: %s", tool)
		}
		if _, err := exec.LookPath(selected.name); err != nil {
			return "", runner{}, fmt.Errorf("generator tool not found in PATH: %s", selected.name)
		}
		return tool, selected, nil
	}

	for _, name := range []string{"openapi-generator", "npx-openapi-generator"} {
		selected := toolRegistry[name]
		if _, err := exec.LookPath(selected.name); err == nil {
			return name, selected, nil
		}
	}
	return "", runner{}, fmt.Errorf("no supported generator tool found in PATH (tried: openapi-generator, npx @openapitools/openapi-generator)")
}

func execute(command string, args []string, stdout io.Writer, stderr io.Writer, action string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed with %s: %w", action, command, err)
	}
	return nil
}
