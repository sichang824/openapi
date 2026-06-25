package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"openapi/internal/bundle"
	"openapi/internal/doctor"
	"openapi/internal/edit"
	"openapi/internal/generate"
	"openapi/internal/scaffold"
	"openapi/internal/workspace"

	"gopkg.in/yaml.v3"
)

var errAddTargetRequired = errors.New("add target is required: path, schema, parameter, response")

type initOptions struct {
	Dir     string
	Title   string
	Version string
}

func executeInit(opts initOptions, stdout io.Writer, stderr io.Writer) error {
	if err := scaffold.Init(filepath.Clean(opts.Dir), opts.Title, opts.Version); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "initialized split OpenAPI scaffold at %s\n", filepath.Clean(opts.Dir))
	return nil
}

type addPathOptions struct {
	Dir      string
	Business string
	Path     string
	Name     string
	Force    bool
}

func executeAddPath(opts addPathOptions, stdout io.Writer, stderr io.Writer) error {
	if strings.TrimSpace(opts.Business) == "" {
		return errors.New("--business is required")
	}
	if strings.TrimSpace(opts.Path) == "" {
		return errors.New("--path is required")
	}

	resolvedBusiness, err := sanitizeRelativePath(opts.Business, "business")
	if err != nil {
		return err
	}

	template, err := scaffold.PathItemTemplate()
	if err != nil {
		return err
	}
	filePath := filepath.Join(filepath.Clean(opts.Dir), resolvedBusiness, "paths.yaml")
	if err := edit.UpsertNamedDefinition(filePath, opts.Path, template, opts.Force); err != nil {
		return err
	}

	openapiPath := workspace.SourceEntryPath(filepath.Clean(opts.Dir))
	ref := "./" + filepath.ToSlash(filepath.Join(resolvedBusiness, "paths.yaml")) + "#/" + jsonPointerEscape(opts.Path)
	if err := edit.UpsertRef(openapiPath, []string{"paths"}, opts.Path, ref, opts.Force); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(stdout, "added path %s -> %s\n", opts.Path, ref)
	return nil
}

type addNamedOptions struct {
	Dir      string
	Business string
	File     string
	Name     string
	Force    bool
}

type addSchemaOptions struct {
	addNamedOptions
	Kind string
}

type addParameterOptions struct {
	addNamedOptions
	In string
}

func executeAddSchema(opts addSchemaOptions, stdout io.Writer, stderr io.Writer) error {
	return executeAddNamedComponent(opts.addNamedOptions, stdout, stderr, namedComponentConfig{
		target:        "schema",
		sectionPath:   []string{"components", "schemas"},
		componentKind: "schemas",
		build: func(flags map[string]string) (*yaml.Node, error) {
			return scaffold.SchemaTemplate(flags["name"], flags["kind"])
		},
		extra: map[string]string{"kind": opts.Kind},
	})
}

func executeAddParameter(opts addParameterOptions, stdout io.Writer, stderr io.Writer) error {
	return executeAddNamedComponent(opts.addNamedOptions, stdout, stderr, namedComponentConfig{
		target:        "parameter",
		sectionPath:   []string{"components", "parameters"},
		componentKind: "parameters",
		build: func(flags map[string]string) (*yaml.Node, error) {
			return scaffold.ParameterTemplate(flags["name"], flags["in"])
		},
		extra: map[string]string{"in": opts.In},
		validate: func(flags map[string]string) error {
			switch flags["in"] {
			case "query", "path", "header", "cookie":
				return nil
			default:
				return errors.New("--in must be one of: query, path, header, cookie")
			}
		},
	})
}

func executeAddResponse(opts addNamedOptions, stdout io.Writer, stderr io.Writer) error {
	return executeAddNamedComponent(opts, stdout, stderr, namedComponentConfig{
		target:        "response",
		sectionPath:   []string{"components", "responses"},
		componentKind: "responses",
		build: func(flags map[string]string) (*yaml.Node, error) {
			return scaffold.ResponseTemplate(flags["name"])
		},
	})
}

type bundleOptions struct {
	Dir  string
	Out  string
	Tool string
}

func executeBundle(opts bundleOptions, stdout io.Writer, stderr io.Writer) error {
	if strings.TrimSpace(opts.Out) == "" {
		return errors.New("--out is required")
	}

	if err := bundle.Run(bundle.Options{
		Dir:    filepath.Clean(opts.Dir),
		Out:    filepath.Clean(opts.Out),
		Tool:   opts.Tool,
		Stdout: stdout,
		Stderr: stderr,
	}); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "bundled %s -> %s\n", workspace.SourceEntryPath(filepath.Clean(opts.Dir)), filepath.Clean(opts.Out))
	return nil
}

type validateOptions struct {
	Dir        string
	Tool       string
	BundleTool string
}

type doctorOptions struct {
	Dir           string
	BundleTool    string
	GeneratorTool string
	JSONOutput    bool
}

type fmtOptions struct {
	Dir string
}

func executeValidate(opts validateOptions, stdout io.Writer, stderr io.Writer) error {
	if err := generate.Validate(generate.ValidateOptions{
		Dir:        filepath.Clean(opts.Dir),
		Tool:       opts.Tool,
		BundleTool: opts.BundleTool,
		Stdout:     stdout,
		Stderr:     stderr,
	}); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "validated %s\n", workspace.SourceEntryPath(filepath.Clean(opts.Dir)))
	return nil
}

func executeDoctor(opts doctorOptions, stdout io.Writer, stderr io.Writer) error {
	report := doctor.Run(doctor.Options{
		Dir:           filepath.Clean(opts.Dir),
		BundleTool:    opts.BundleTool,
		GeneratorTool: opts.GeneratorTool,
	})
	if opts.JSONOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return err
		}
		if report.HasFailures() {
			return errors.New("doctor found problems")
		}
		return nil
	}
	for _, check := range report.Checks {
		if strings.TrimSpace(check.Detail) == "" {
			_, _ = fmt.Fprintf(stdout, "%s: %s\n", check.Name, check.Status)
			continue
		}
		_, _ = fmt.Fprintf(stdout, "%s: %s (%s)\n", check.Name, check.Status, check.Detail)
	}
	_, _ = fmt.Fprintf(stdout, "status: %s\n", report.Status)
	if report.HasFailures() {
		return errors.New("doctor found problems")
	}
	return nil
}

func executeFmt(opts fmtOptions, stdout io.Writer, stderr io.Writer) error {
	formatted, err := edit.FormatSplitWorkspace(filepath.Clean(opts.Dir))
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "formatted %d files under %s\n", len(formatted), filepath.Clean(opts.Dir))
	return nil
}

type generateOptions struct {
	Dir                  string
	Lang                 string
	Out                  string
	Tool                 string
	BundleTool           string
	Config               string
	AdditionalProperties []string
	GlobalProperties     []string
}

func executeGenerate(opts generateOptions, stdout io.Writer, stderr io.Writer) error {
	if strings.TrimSpace(opts.Lang) == "" {
		return errors.New("--lang is required")
	}
	if strings.TrimSpace(opts.Out) == "" {
		return errors.New("--out is required")
	}

	if err := generate.Generate(generate.GenerateOptions{
		Dir:                  filepath.Clean(opts.Dir),
		Lang:                 strings.TrimSpace(opts.Lang),
		Out:                  filepath.Clean(opts.Out),
		Tool:                 opts.Tool,
		BundleTool:           opts.BundleTool,
		Config:               strings.TrimSpace(opts.Config),
		AdditionalProperties: opts.AdditionalProperties,
		GlobalProperties:     opts.GlobalProperties,
		Stdout:               stdout,
		Stderr:               stderr,
	}); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "generated %s client at %s\n", strings.TrimSpace(opts.Lang), filepath.Clean(opts.Out))
	return nil
}

type namedComponentConfig struct {
	target        string
	sectionPath   []string
	componentKind string
	build         func(flags map[string]string) (*yaml.Node, error)
	extra         map[string]string
	validate      func(flags map[string]string) error
}

func executeAddNamedComponent(opts addNamedOptions, stdout io.Writer, stderr io.Writer, config namedComponentConfig) error {
	if strings.TrimSpace(opts.Business) == "" {
		return errors.New("--business is required")
	}
	if strings.TrimSpace(opts.Name) == "" {
		return errors.New("--name is required")
	}

	resolvedBusiness, err := sanitizeRelativePath(opts.Business, "business")
	if err != nil {
		return err
	}
	resolvedFile := config.componentKind
	if strings.EqualFold(resolvedBusiness, "common") {
		if strings.TrimSpace(opts.File) == "" {
			return errors.New("--file is required when --business=common")
		}
		resolvedFile, err = sanitizeRelativePath(opts.File, "file")
		if err != nil {
			return err
		}
	} else if strings.TrimSpace(opts.File) != "" {
		return fmt.Errorf("--file is only supported when --business=common; business-specific %ss are stored in %s/%s.yaml", config.target, resolvedBusiness, config.componentKind)
	}

	resolvedFlags := map[string]string{
		"dir":      filepath.Clean(opts.Dir),
		"business": resolvedBusiness,
		"file":     resolvedFile,
		"name":     strings.TrimSpace(opts.Name),
	}
	for key, value := range config.extra {
		resolvedFlags[key] = strings.TrimSpace(value)
	}
	if config.validate != nil {
		if err := config.validate(resolvedFlags); err != nil {
			return err
		}
	}

	template, err := config.build(resolvedFlags)
	if err != nil {
		return err
	}
	definitionNode, err := extractNamedNode(template, resolvedFlags["name"])
	if err != nil {
		return err
	}

	componentDir := resolvedFlags["business"]
	componentPath := filepath.Join(resolvedFlags["dir"], componentDir, resolvedFlags["file"]+".yaml")
	if err := edit.UpsertNamedDefinition(componentPath, resolvedFlags["name"], definitionNode, opts.Force); err != nil {
		return err
	}

	openapiPath := workspace.SourceEntryPath(resolvedFlags["dir"])
	ref := "./" + filepath.ToSlash(filepath.Join(componentDir, resolvedFlags["file"]+".yaml")) + "#/" + resolvedFlags["name"]
	if err := edit.UpsertRef(openapiPath, config.sectionPath, resolvedFlags["name"], ref, opts.Force); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(stdout, "added %s %s\n", config.target, resolvedFlags["name"])
	return nil
}

func jsonPointerEscape(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}

func sanitizeRelativePath(value string, field string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("--%s is required", field)
	}
	if filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("--%s must be a relative path", field)
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("--%s must not be empty", field)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("--%s must stay within the OpenAPI workspace", field)
	}
	return cleaned, nil
}

func writeYAMLFile(path string, node *yaml.Node, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("file already exists: %s (use --force to overwrite)", path)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(node); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func extractNamedNode(root *yaml.Node, name string) (*yaml.Node, error) {
	if root == nil || root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("template root must be a mapping")
	}
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == name {
			return cloneNode(root.Content[i+1]), nil
		}
	}
	return nil, fmt.Errorf("template missing named node: %s", name)
}

func cloneNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	copyNode := *node
	if len(node.Content) > 0 {
		copyNode.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			copyNode.Content[i] = cloneNode(child)
		}
	}
	return &copyNode
}
