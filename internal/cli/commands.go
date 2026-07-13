package cli

import (
	"errors"
	"io"
	"strings"
	"sync"
	"text/template"

	"github.com/spf13/cobra"
)

var configureHelpOnce sync.Once

func Run(args []string, stdout io.Writer, stderr io.Writer) error {
	root := newRootCommand(stdout, stderr)
	root.SetArgs(args)
	return root.Execute()
}

func newRootCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	configureHelpTemplates()

	root := &cobra.Command{
		Use:   "oapi",
		Short: "Split OpenAPI authoring, validation, generation, and inspection CLI",
		Long: `oapi helps maintain a split OpenAPI workspace without hand-editing
aggregator refs, and provides query/call workflows for large specs.

Use it to scaffold split-spec files, bundle/validate/generate through
external tooling, inspect endpoint contracts, and call documented APIs.`,
		Example: `  oapi init --dir ./api/openapi --title "Experts Backend API" --version 0.1.0
  oapi add path --dir ./api/openapi --path /workflow-runs
	oapi fmt --dir ./api/openapi
	oapi doctor --dir ./api/openapi --json
  oapi validate --dir ./api/openapi
  oapi generate --dir ./api/openapi --lang go --out ./internal/sdk/generated/openapi
  oapi query -f ./openapi.json -q workflow -vv
  oapi call -f ./openapi.json -e "GET /users" --base-url https://api.example.com`,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetHelpTemplate(commandHelpTemplate)
	root.AddGroup(
		&cobra.Group{ID: "authoring", Title: "Authoring Commands:"},
		&cobra.Group{ID: "orchestration", Title: "Orchestration Commands:"},
		&cobra.Group{ID: "inspection", Title: "Inspection Commands:"},
	)

	root.AddCommand(
		newInitCommand(stdout, stderr),
		newAddCommand(stdout, stderr),
		newFmtCommand(stdout, stderr),
		newBundleCommand(stdout, stderr),
		newDoctorCommand(stdout, stderr),
		newValidateCommand(stdout, stderr),
		newGenerateCommand(stdout, stderr),
		newQueryCommand(stdout, stderr),
		newCallCommand(stdout, stderr),
	)
	return root
}

func newQueryCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	opts := queryOptions{
		Limit:  20,
		Offset: 0,
	}
	cmd := &cobra.Command{
		Use:   "query",
		Short: "Search endpoints by keyword",
		Long: `Search a large OpenAPI document without opening it by hand.

Use query when you need to find candidate endpoints quickly, inspect them at
progressive detail levels, or emit structured JSON for tooling.

Common boundaries and pitfalls:
- query reads a single bundled or standalone spec file; it does not traverse a split directory
- use -n/--name for <name>.openapi.yaml under $OAPI_SPECS_DIR (default ~/.openapi/specs), or -f for a path
- -q is optional, so "oapi query -f ./openapi.json" is the full inventory mode
- --limit and --offset page both list mode and search mode
- use -vvv only when you actually need expanded contract detail`,
		Example: "  oapi query -f ./openapi.json\n  oapi query -n skill -q workflow -vv\n  oapi query --name skill-internal -q order --json",
		GroupID: "inspection",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("name") && strings.TrimSpace(opts.Name) == "" {
				return errors.New("--name must not be empty")
			}
			return executeQuery(opts, stdout, stderr)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&opts.File, "f", "f", "", "OpenAPI spec file (default openapi.json)")
	flags.StringVarP(&opts.Name, "name", "n", "", "spec name resolved from $"+specsDirEnv+" (default ~/.openapi/specs)")
	cmd.MarkFlagsMutuallyExclusive("f", "name")
	flags.StringVarP(&opts.Keyword, "q", "q", "", "keyword")
	flags.IntVar(&opts.Limit, "limit", opts.Limit, "result limit")
	flags.IntVar(&opts.Offset, "offset", opts.Offset, "result offset")
	flags.BoolVar(&opts.JSONOutput, "json", false, "output query results as JSON")
	flags.CountVarP(&opts.Verbose, "verbose", "v", "verbosity level, repeat for details")
	return cmd
}

func newCallCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	opts := callOptions{}
	cmd := &cobra.Command{
		Use:   "call",
		Short: "Call an API endpoint with parameters",
		Long: `Call a documented endpoint directly from the CLI with spec-aware
parameter validation and request construction.

Use call when you want a fast contract-to-request loop without writing ad hoc
curl commands.

Common boundaries and pitfalls:
- call expects a single spec file, not a split OpenAPI directory
- the spec file may be JSON or YAML
- use -n/--name for <name>.openapi.yaml under $OAPI_SPECS_DIR (default ~/.openapi/specs), or -f for a path
- pass exactly one of --params, --params-file, or --params-url
- if the spec has no servers[], you must provide --base-url
- strict mode blocks on validation errors; default mode still sends requests when only warnings exist
- POST/PUT/PATCH/DELETE can have real side effects, so treat call as a live operation tool`,
		Example: "  oapi call -f ./openapi.yaml -e \"GET /users\" --base-url https://api.example.com\n  oapi call -n skill -e \"POST /cart/add\" --params '{\"item_id\":\"123\",\"quantity\":2}'\n  oapi call --name skill-internal -e \"GET /protected\" --bearer-token '<token>'",
		GroupID: "inspection",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("name") && strings.TrimSpace(opts.Name) == "" {
				return errors.New("--name must not be empty")
			}
			return executeCall(opts, stdout, stderr)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&opts.File, "f", "f", "", "OpenAPI spec file (JSON or YAML; default openapi.json)")
	flags.StringVarP(&opts.Name, "name", "n", "", "spec name resolved from $"+specsDirEnv+" (default ~/.openapi/specs)")
	cmd.MarkFlagsMutuallyExclusive("f", "name")
	flags.StringVarP(&opts.Endpoint, "e", "e", "", "endpoint (e.g., POST /users)")
	flags.StringVar(&opts.BaseURL, "base-url", "", "base URL of the API server")
	flags.StringVar(&opts.Params, "params", "", "JSON string of parameters")
	flags.StringVar(&opts.ParamsFile, "params-file", "", "JSON file containing parameters")
	flags.StringVar(&opts.ParamsURL, "params-url", "", "URL query string parameters (supports repeated keys/arrays, e.g., key1=val1&key1=val2 or order[]=status&order[]=admin_order)")
	flags.StringVar(&opts.BodyFile, "body-file", "", "raw request body file (curl --data-binary style)")
	flags.StringVar(&opts.ContentType, "content-type", "", "override request Content-Type header")
	flags.StringArrayVar(&opts.Headers, "header", nil, "custom request header in 'Name: Value' form; repeatable")
	flags.StringVar(&opts.BearerToken, "bearer-token", "", "set Authorization: Bearer <token> on the request")
	flags.StringVar(&opts.Cookie, "cookie", "", "raw Cookie header value (semicolon-separated; same idea as curl -H 'Cookie: ...')")
	flags.StringVar(&opts.CookiePath, "cookie-path", "", "read cookies from a Netscape cookie jar file (curl -b file style)")
	flags.BoolVar(&opts.Strict, "strict", false, "strict parameter validation")
	flags.CountVarP(&opts.Verbose, "verbose", "v", "verbosity level, repeat for details")
	return cmd
}

func newInitCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	opts := initOptions{Dir: ".", Title: "OpenAPI", Version: "0.1.0"}
	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Initialize a split OpenAPI workspace",
		Example: "  oapi init --dir ./api/openapi --title \"Experts Backend API\" --version 0.1.0",
		GroupID: "authoring",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeInit(opts, stdout, stderr)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.Dir, "dir", opts.Dir, "target OpenAPI directory")
	flags.StringVar(&opts.Title, "title", opts.Title, "spec title")
	flags.StringVar(&opts.Version, "version", opts.Version, "spec version")
	return cmd
}

func newAddCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add path/schema/parameter/response templates",
		Long: `Add creates minimal split-spec files and updates the root aggregator
for you.

Use it to automate the repetitive parts of authoring without trying to infer
business fields.

The subcommands are grouped by file type so path item creation stays separate
from reusable component creation.

	Layout rule:
	- --business common writes shared component files under common/*.yaml
	- --business <name> writes business files under <name>/paths.yaml, <name>/schemas.yaml, <name>/parameters.yaml, or <name>/responses.yaml

Common boundaries and pitfalls:
- generated templates are intentionally skeletal and must be edited afterward
- existing files or refs fail unless you pass --force
- add updates the split tree and index.yaml refs; it does not bundle or validate`,
		Example: "  oapi add path --help\n  oapi add schema --help\n  oapi add parameter --help\n  oapi add response --help",
		GroupID: "authoring",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return errAddTargetRequired
		},
	}
	cmd.AddGroup(
		&cobra.Group{ID: "paths", Title: "Path Commands:"},
		&cobra.Group{ID: "components", Title: "Component Commands:"},
	)
	cmd.AddCommand(
		newAddPathCommand(stdout, stderr),
		newAddSchemaCommand(stdout, stderr),
		newAddParameterCommand(stdout, stderr),
		newAddResponseCommand(stdout, stderr),
	)
	return cmd
}

func newBundleCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	opts := bundleOptions{Dir: ".", Tool: "auto"}
	cmd := &cobra.Command{
		Use:     "bundle",
		Short:   "Bundle split files into one spec",
		Example: "  oapi bundle --dir ./api/openapi --out ./api/openapi/dist/openapi.yaml",
		GroupID: "orchestration",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeBundle(opts, stdout, stderr)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.Dir, "dir", opts.Dir, "target OpenAPI directory")
	flags.StringVar(&opts.Out, "out", "", "bundle output file (required)")
	flags.StringVar(&opts.Tool, "tool", opts.Tool, "bundle tool: auto, redocly, swagger-cli, npx-redocly")
	return cmd
}

func newFmtCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	opts := fmtOptions{Dir: "."}
	cmd := &cobra.Command{
		Use:   "fmt",
		Short: "Format split-spec YAML files deterministically",
		Long: `Format rewrites the split OpenAPI workspace into a stable YAML layout.

Use fmt when you want repeatable ordering and indentation before review, CI,
or bundling.

Common boundaries and pitfalls:
- fmt only rewrites index.yaml and the tracked split YAML files under common/ plus business directories like workflow/
- fmt sorts mappings and normalizes YAML layout; it does not rename files or rewrite spec semantics
- generated dist/openapi.yaml artifacts are out of scope and are not touched`,
		Example: "  oapi fmt --dir ./api/openapi",
		GroupID: "authoring",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeFmt(opts, stdout, stderr)
		},
	}
	cmd.Flags().StringVar(&opts.Dir, "dir", opts.Dir, "target OpenAPI directory")
	return cmd
}

func newDoctorCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	opts := doctorOptions{Dir: ".", BundleTool: "auto", GeneratorTool: "auto"}
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check split workspace health and required tools",
		Long: `Doctor inspects the split OpenAPI workspace and the external tools that
oapi depends on for bundle, validate, and generate workflows.

Use doctor before validate/generate when you want a fast readiness check for
directory structure and tool availability.

Common boundaries and pitfalls:
- doctor checks workspace shape and tool resolution, not semantic correctness of the spec
- auto tool mode follows the same discovery order as bundle and generate
- a missing index.yaml or common/ directory is reported directly as a workspace issue`,
		Example: "  oapi doctor --dir ./api/openapi\n  oapi doctor --dir ./api/openapi --json\n  oapi doctor --dir ./api/openapi --bundle-tool redocly --generator-tool openapi-generator",
		GroupID: "orchestration",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeDoctor(opts, stdout, stderr)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.Dir, "dir", opts.Dir, "target OpenAPI directory")
	flags.StringVar(&opts.BundleTool, "bundle-tool", opts.BundleTool, "bundle tool: auto, redocly, swagger-cli, npx-redocly")
	flags.StringVar(&opts.GeneratorTool, "generator-tool", opts.GeneratorTool, "generator tool: auto, openapi-generator, npx-openapi-generator")
	flags.BoolVar(&opts.JSONOutput, "json", false, "output doctor report as JSON")
	return cmd
}

func newValidateCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	opts := validateOptions{Dir: ".", Tool: "auto", BundleTool: "auto"}
	cmd := &cobra.Command{
		Use:     "validate",
		Short:   "Bundle then validate a split spec",
		Example: "  oapi validate --dir ./api/openapi\n  oapi validate --dir ./api/openapi --tool openapi-generator --bundle-tool redocly",
		GroupID: "orchestration",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeValidate(opts, stdout, stderr)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.Dir, "dir", opts.Dir, "target OpenAPI directory")
	flags.StringVar(&opts.Tool, "tool", opts.Tool, "generator tool: auto, openapi-generator, npx-openapi-generator")
	flags.StringVar(&opts.BundleTool, "bundle-tool", opts.BundleTool, "bundle tool: auto, redocly, swagger-cli, npx-redocly")
	return cmd
}

func newGenerateCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	opts := generateOptions{Dir: ".", Tool: "auto", BundleTool: "auto"}
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Bundle then run openapi-generator",
		Long: `Generate code from a split OpenAPI workspace by bundling to a temporary
single-file spec and then delegating to openapi-generator.

Use generate when you want a stable split-spec authoring flow but still need a
single generated SDK or client artifact.

Common boundaries and pitfalls:
- generate does not point openapi-generator directly at the split tree; it always bundles first
- --lang and --out are required
- generator-specific tuning is intentionally narrow here: use --config, --additional-property, and --global-property
- generated output is tool-owned code, so expect overwrites in the target directory`,
		Example: "  oapi generate --dir ./api/openapi --lang go --out ./internal/sdk/generated/openapi\n  oapi generate --dir ./api/openapi --lang typescript-fetch --out ../experts-web/src/generated/api\n  oapi generate --dir ./api/openapi --lang go --out ./internal/sdk/generated/openapi --additional-property packageName=experts",
		GroupID: "orchestration",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeGenerate(opts, stdout, stderr)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.Dir, "dir", opts.Dir, "target OpenAPI directory")
	flags.StringVar(&opts.Lang, "lang", "", "generator language (required)")
	flags.StringVar(&opts.Out, "out", "", "generator output directory (required)")
	flags.StringVar(&opts.Tool, "tool", opts.Tool, "generator tool: auto, openapi-generator, npx-openapi-generator")
	flags.StringVar(&opts.BundleTool, "bundle-tool", opts.BundleTool, "bundle tool: auto, redocly, swagger-cli, npx-redocly")
	flags.StringVar(&opts.Config, "config", "", "optional openapi-generator config file")
	flags.StringArrayVar(&opts.AdditionalProperties, "additional-property", nil, "additional property passed to openapi-generator (repeatable, e.g., packageName=experts)")
	flags.StringArrayVar(&opts.GlobalProperties, "global-property", nil, "global property passed to openapi-generator (repeatable, e.g., models)")
	return cmd
}

func newAddPathCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	opts := addPathOptions{Dir: "."}
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Add a path item template and root ref",
		Long: `Add a path item under <business>/paths.yaml and wire the matching
root paths ref in index.yaml.

Use --business to choose which business file owns the path item.`,
		Example: "  oapi add path --dir ./api/openapi --business workflow --path /workflow-runs\n  oapi add path --dir ./api/openapi --business workflow --path /workflow-runs/{runZid}\n  oapi add path --dir ./api/openapi --business workflow --path /workflow-runs/{runZid} --force",
		GroupID: "paths",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeAddPath(opts, stdout, stderr)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.Dir, "dir", opts.Dir, "target OpenAPI directory")
	flags.StringVar(&opts.Business, "business", "", "business directory that owns this path file")
	flags.StringVar(&opts.Path, "path", "", "OpenAPI path key (required)")
	flags.StringVar(&opts.Name, "name", "", "reserved for future path naming strategies")
	flags.BoolVar(&opts.Force, "force", false, "overwrite existing file/ref")
	return cmd
}

func newAddSchemaCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	opts := addSchemaOptions{addNamedOptions: addNamedOptions{Dir: "."}, Kind: "object"}
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Add a schema template and root ref",
		Long: `Add a schema definition and wire the matching root components.schemas ref.

Use --business common for shared files like common/common.yaml.
Use --business <name> for business-local definitions in <name>/schemas.yaml.`,
		Example: "  oapi add schema --dir ./api/openapi --business common --file common --name ErrorResponse --kind object\n  oapi add schema --dir ./api/openapi --business workflow --name CreateWorkflowRunRequest --kind object",
		GroupID: "components",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeAddSchema(opts, stdout, stderr)
		},
	}
	bindAddNamedFlags(cmd, &opts.addNamedOptions, "schema")
	cmd.Flags().StringVar(&opts.Kind, "kind", opts.Kind, "schema kind")
	return cmd
}

func newAddParameterCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	opts := addParameterOptions{addNamedOptions: addNamedOptions{Dir: "."}}
	cmd := &cobra.Command{
		Use:   "parameter",
		Short: "Add a parameter template and root ref",
		Long: `Add a parameter definition and wire the matching root components.parameters ref.

Use --business common for shared files like common/pagination.yaml.
Use --business <name> for business-local definitions in <name>/parameters.yaml.`,
		Example: "  oapi add parameter --dir ./api/openapi --business common --file pagination --name Page --in query\n  oapi add parameter --dir ./api/openapi --business workflow --name RunZid --in path",
		GroupID: "components",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeAddParameter(opts, stdout, stderr)
		},
	}
	bindAddNamedFlags(cmd, &opts.addNamedOptions, "parameter")
	cmd.Flags().StringVar(&opts.In, "in", "", "parameter location (query, path, header, cookie)")
	return cmd
}

func newAddResponseCommand(stdout io.Writer, stderr io.Writer) *cobra.Command {
	opts := addNamedOptions{Dir: "."}
	cmd := &cobra.Command{
		Use:   "response",
		Short: "Add a response template and root ref",
		Long: `Add a response definition and wire the matching root components.responses ref.

Use --business common for shared files like common/errors.yaml.
Use --business <name> for business-local definitions in <name>/responses.yaml.`,
		Example: "  oapi add response --dir ./api/openapi --business common --file errors --name Error\n  oapi add response --dir ./api/openapi --business workflow --name WorkflowRunCreated",
		GroupID: "components",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeAddResponse(opts, stdout, stderr)
		},
	}
	bindAddNamedFlags(cmd, &opts, "response")
	return cmd
}

func bindAddNamedFlags(cmd *cobra.Command, opts *addNamedOptions, target string) {
	flags := cmd.Flags()
	flags.StringVar(&opts.Dir, "dir", opts.Dir, "target OpenAPI directory")
	flags.StringVar(&opts.Business, "business", "", "common for shared files, or a business directory like workflow")
	flags.StringVar(&opts.File, "file", "", target+" file name under common/; ignored for business-local files")
	flags.StringVar(&opts.Name, "name", "", target+" name")
	flags.BoolVar(&opts.Force, "force", false, "overwrite existing definition/ref")
}

func configureHelpTemplates() {
	configureHelpOnce.Do(func() {
		cobra.AddTemplateFuncs(template.FuncMap{
			"groupDescription": groupDescription,
			"groupCommands":    groupCommands,
			"otherCommands":    otherCommands,
			"indentBlock":      indentBlock,
		})
	})
}

func groupDescription(groupID string) string {
	switch groupID {
	case "authoring":
		return "Scaffold and maintain split-spec files."
	case "orchestration":
		return "Bundle and delegate validation or code generation."
	case "inspection":
		return "Inspect contracts and call documented endpoints."
	case "paths":
		return "Create path-item files and wire path refs."
	case "components":
		return "Create reusable schema, parameter, and response components."
	default:
		return ""
	}
}

func groupCommands(commands []*cobra.Command, groupID string) []*cobra.Command {
	filtered := make([]*cobra.Command, 0, len(commands))
	for _, cmd := range commands {
		if !cmd.IsAvailableCommand() || cmd.IsAdditionalHelpTopicCommand() {
			continue
		}
		if cmd.GroupID == groupID {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
}

func otherCommands(commands []*cobra.Command) []*cobra.Command {
	filtered := make([]*cobra.Command, 0, len(commands))
	for _, cmd := range commands {
		if !cmd.IsAvailableCommand() || cmd.IsAdditionalHelpTopicCommand() {
			continue
		}
		if cmd.GroupID == "" {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
}

func indentBlock(value string, prefix string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	lines := strings.Split(strings.TrimRight(value, "\n"), "\n")
	for index, line := range lines {
		lines[index] = prefix + line
	}
	return strings.Join(lines, "\n")
}

const commandHelpTemplate = `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}Usage:
  {{if .Runnable}}{{.UseLine}}{{else}}{{.CommandPath}} [command]{{end}}

{{if eq .CommandPath "oapi"}}Quick Start:
  oapi --help
  oapi add path --help
  oapi generate --help

Common Paths:
  split spec dir  ./api/openapi
  bundled spec    ./api/openapi/dist/openapi.yaml
  go sdk output   ./internal/sdk/generated/openapi

{{end}}{{if .Example}}Examples:
{{indentBlock .Example "  "}}

{{end}}{{if .HasAvailableSubCommands}}{{if .Groups}}{{range .Groups}}{{ $cmds := groupCommands $.Commands .ID }}{{if $cmds}}{{.Title}} {{groupDescription .ID}}
{{range $cmds}}  {{rpad .Name 12}}{{.Short}}
{{end}}
{{end}}{{end}}{{ $other := otherCommands .Commands }}{{if $other}}Other Commands:
{{range $other}}  {{rpad .Name 12}}{{.Short}}
{{end}}
{{end}}{{else}}Commands:
{{range .Commands}}{{if (and .IsAvailableCommand (not .IsAdditionalHelpTopicCommand))}}  {{rpad .Name 12}}{{.Short}}
{{end}}{{end}}
{{end}}{{end}}{{if .HasAvailableLocalFlags}}Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}

{{end}}{{if .HasAvailableInheritedFlags}}Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}

{{end}}More:
  Use "{{.CommandPath}} [command] --help" for more information about a command.
`
