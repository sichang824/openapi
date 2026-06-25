package scaffold

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"openapi/internal/workspace"

	"gopkg.in/yaml.v3"
)

func Init(dir string, title string, version string) error {
	openapiPath := workspace.SourceEntryPath(dir)
	if _, err := os.Stat(openapiPath); err == nil {
		return fmt.Errorf("openapi entry already exists: %s", openapiPath)
	} else if !os.IsNotExist(err) {
		return err
	}

	dirs := []string{
		dir,
		filepath.Join(dir, "common"),
	}
	for _, path := range dirs {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}

	content, err := render(rootDocument(strings.TrimSpace(title), strings.TrimSpace(version)))
	if err != nil {
		return err
	}
	return os.WriteFile(openapiPath, content, 0o644)
}

func SuggestedPathName(path string) string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return "root"
	}

	segments := strings.Split(trimmed, "/")
	parts := make([]string, 0, len(segments)*2)
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			paramName := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
			paramPart := toKebab(paramName)
			if len(parts) > 0 {
				parts[len(parts)-1] = singularize(parts[len(parts)-1])
				paramPart = trimEntityPrefix(parts[len(parts)-1], paramPart)
				parts = append(parts, "by")
			}
			parts = append(parts, paramPart)
			continue
		}
		parts = append(parts, sanitizeSegment(segment))
	}

	name := strings.Join(parts, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return "path-item"
	}
	return name
}

func PathItemTemplate() (*yaml.Node, error) {
	return decodeTemplate(`get:
  summary: TODO
  operationId: todoOperation
  responses:
    '200':
      description: OK
`)
}

func SchemaTemplate(name string, kind string) (*yaml.Node, error) {
	resolvedKind := strings.TrimSpace(kind)
	if resolvedKind == "" {
		resolvedKind = "object"
	}

	var template string
	switch resolvedKind {
	case "object":
		template = fmt.Sprintf(`%s:
  type: object
  properties: {}
`, name)
	case "array":
		template = fmt.Sprintf(`%s:
  type: array
  items: {}
`, name)
	default:
		template = fmt.Sprintf(`%s:
  type: %s
`, name, resolvedKind)
	}
	return decodeTemplate(template)
}

func ParameterTemplate(name string, in string) (*yaml.Node, error) {
	wireName := defaultParameterName(name)
	required := "false"
	typeName := guessParameterType(wireName)
	if in == "path" {
		required = "true"
		typeName = "string"
	}

	template := fmt.Sprintf(`%s:
  name: %s
  in: %s
  required: %s
  schema:
    type: %s
`, name, wireName, in, required, typeName)
	return decodeTemplate(template)
}

func ResponseTemplate(name string) (*yaml.Node, error) {
	template := fmt.Sprintf(`%s:
  description: TODO
`, name)
	return decodeTemplate(template)
}

func render(node *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(node); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func rootDocument(title string, version string) *yaml.Node {
	if title == "" {
		title = "OpenAPI"
	}
	if version == "" {
		version = "0.1.0"
	}

	return &yaml.Node{
		Kind: yaml.DocumentNode,
		Content: []*yaml.Node{
			mappingNode(
				"openapi", scalarNode("3.0.3"),
				"info", mappingNode(
					"title", scalarNode(title),
					"version", scalarNode(version),
				),
				"paths", emptyMappingNode(),
				"components", mappingNode(
					"schemas", emptyMappingNode(),
					"parameters", emptyMappingNode(),
					"responses", emptyMappingNode(),
				),
			),
		},
	}
}

func decodeTemplate(template string) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(template), &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		return nil, fmt.Errorf("empty template")
	}
	return doc.Content[0], nil
}

func mappingNode(pairs ...interface{}) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode}
	for i := 0; i < len(pairs); i += 2 {
		key := pairs[i].(string)
		value := pairs[i+1].(*yaml.Node)
		node.Content = append(node.Content, scalarNode(key), value)
	}
	return node
}

func emptyMappingNode() *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode}
}

func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func defaultParameterName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return trimmed
	}
	runes := []rune(trimmed)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func guessParameterType(name string) string {
	switch strings.ToLower(name) {
	case "page", "pagesize", "page_size", "limit", "offset", "size":
		return "integer"
	default:
		return "string"
	}
}

func singularize(value string) string {
	if strings.HasSuffix(value, "ies") && len(value) > 3 {
		return value[:len(value)-3] + "y"
	}
	if strings.HasSuffix(value, "s") && len(value) > 1 {
		return value[:len(value)-1]
	}
	return value
}

func sanitizeSegment(segment string) string {
	cleaned := toKebab(segment)
	if cleaned == "" {
		return "item"
	}
	return cleaned
}

func trimEntityPrefix(entity string, param string) string {
	entity = strings.TrimSpace(entity)
	param = strings.TrimSpace(param)
	if entity == "" || param == "" {
		return param
	}
	parts := strings.Split(entity, "-")
	last := parts[len(parts)-1]
	prefix := last + "-"
	if strings.HasPrefix(param, prefix) {
		trimmed := strings.TrimPrefix(param, prefix)
		if trimmed != "" {
			return trimmed
		}
	}
	return param
}

func toKebab(value string) string {
	var builder strings.Builder
	lastDash := false
	for index, r := range value {
		switch {
		case r == '-' || r == '_' || r == ' ' || r == '.':
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		case unicode.IsUpper(r):
			if index > 0 && !lastDash {
				builder.WriteByte('-')
			}
			builder.WriteRune(unicode.ToLower(r))
			lastDash = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(unicode.ToLower(r))
			lastDash = false
		}
	}
	return strings.Trim(builder.String(), "-")
}
