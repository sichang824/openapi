package bundle

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"openapi/internal/workspace"
)

type Options struct {
	Dir    string
	Out    string
	Tool   string
	Stdout io.Writer
	Stderr io.Writer
}

type runner struct {
	label string
	name  string
	args  func(input string, output string) []string
}

type ToolSelection struct {
	Key     string
	Label   string
	Command string
}

var toolRegistry = map[string]runner{
	"redocly": {
		label: "redocly",
		name:  "redocly",
		args: func(input string, output string) []string {
			return []string{"bundle", input, "-o", output}
		},
	},
	"swagger-cli": {
		label: "swagger-cli",
		name:  "swagger-cli",
		args: func(input string, output string) []string {
			return []string{"bundle", input, "-o", output, "-t", "yaml"}
		},
	},
	"npx-redocly": {
		label: "npx @redocly/cli",
		name:  "npx",
		args: func(input string, output string) []string {
			return []string{"@redocly/cli", "bundle", input, "-o", output}
		},
	},
}

func Run(opts Options) error {
	input := workspace.SourceEntryPath(opts.Dir)
	if _, err := os.Stat(input); err != nil {
		return fmt.Errorf("bundle input not found: %s", input)
	}
	if strings.TrimSpace(opts.Out) == "" {
		return fmt.Errorf("bundle output is required")
	}
	if err := os.MkdirAll(filepath.Dir(opts.Out), 0o755); err != nil {
		return err
	}

	_, selected, err := resolveRunner(strings.TrimSpace(opts.Tool))
	if err != nil {
		return err
	}

	cmd := exec.Command(selected.name, selected.args(input, opts.Out)...)
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bundle failed with %s: %w", selected.name, err)
	}
	return nil
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
			return "", runner{}, fmt.Errorf("unsupported bundle tool: %s", tool)
		}
		if _, err := exec.LookPath(selected.name); err != nil {
			return "", runner{}, fmt.Errorf("bundle tool not found in PATH: %s", selected.name)
		}
		return tool, selected, nil
	}

	for _, name := range []string{"redocly", "swagger-cli", "npx-redocly"} {
		selected := toolRegistry[name]
		if _, err := exec.LookPath(selected.name); err == nil {
			return name, selected, nil
		}
	}
	return "", runner{}, fmt.Errorf("no supported bundle tool found in PATH (tried: redocly, swagger-cli, npx @redocly/cli)")
}
