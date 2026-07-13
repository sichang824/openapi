package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Document struct {
	Swagger             string                    `json:"swagger"`
	OpenAPI             string                    `json:"openapi"`
	Info                Info                      `json:"info"`
	Host                string                    `json:"host"`
	BasePath            string                    `json:"basePath"`
	Schemes             []string                  `json:"schemes"`
	Servers             []Server                  `json:"servers"`
	Security            SecurityRequirements      `json:"security"`
	Tags                []Tag                     `json:"tags"`
	Paths               map[string]PathItem       `json:"paths"`
	Components          Components                `json:"components"`
	SecurityDefinitions map[string]SecurityScheme `json:"securityDefinitions"`
	Definitions         map[string]Schema         `json:"definitions"`
	Warnings            []string                  `json:"-"`
}

type Tag struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Info struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

type Server struct {
	URL string `json:"url"`
}

type PathItem map[string]Operation

var httpOperationKeys = map[string]struct{}{
	"get":     {},
	"put":     {},
	"post":    {},
	"delete":  {},
	"options": {},
	"head":    {},
	"patch":   {},
	"trace":   {},
}

type Operation struct {
	Summary     string                `json:"summary"`
	Description string                `json:"description"`
	OperationID string                `json:"operationId"`
	Tags        []string              `json:"tags"`
	Consumes    []string              `json:"consumes"`
	Produces    []string              `json:"produces"`
	Parameters  []Parameter           `json:"parameters"`
	RequestBody *RequestBody          `json:"requestBody"`
	Responses   map[string]Response   `json:"responses"`
	Security    *SecurityRequirements `json:"security"`
}

type SecurityRequirement map[string][]string
type SecurityRequirements []SecurityRequirement

type SecurityScheme struct {
	Type         string `json:"type"`
	In           string `json:"in"`
	Name         string `json:"name"`
	Scheme       string `json:"scheme"`
	BearerFormat string `json:"bearerFormat"`
}

type Parameter struct {
	Ref             string               `json:"$ref"`
	Name            string               `json:"name"`
	In              string               `json:"in"`
	Required        bool                 `json:"required"`
	Description     string               `json:"description"`
	Example         any                  `json:"example"`
	Examples        map[string]Example   `json:"examples"`
	Deprecated      bool                 `json:"deprecated"`
	AllowEmptyValue bool                 `json:"allowEmptyValue"`
	Style           string               `json:"style"`
	Explode         *bool                `json:"explode"`
	AllowReserved   bool                 `json:"allowReserved"`
	Content         map[string]MediaType `json:"content"`
	Schema          Schema               `json:"schema"`
}

type Example struct {
	Summary       string `json:"summary"`
	Description   string `json:"description"`
	Value         any    `json:"value"`
	ExternalValue string `json:"externalValue"`
}

type RequestBody struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content"`
}

type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content"`
	Schema      Schema               `json:"schema"`
}

type MediaType struct {
	Schema   Schema             `json:"schema"`
	Example  any                `json:"example"`
	Examples map[string]Example `json:"examples"`
}

type Components struct {
	Schemas         map[string]Schema         `json:"schemas"`
	Parameters      map[string]Parameter      `json:"parameters"`
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes"`
}

type Schema struct {
	Type                 string            `json:"type"`
	Ref                  string            `json:"$ref"`
	Required             []string          `json:"required"`
	Properties           map[string]Schema `json:"properties"`
	Items                *Schema           `json:"items"`
	AdditionalProperties any               `json:"additionalProperties"`
	Description          string            `json:"description"`
	Example              any               `json:"example"`
	Enum                 []any             `json:"enum"`
	Default              any               `json:"default"`
	Maximum              *float64          `json:"maximum"`
	Nullable             bool              `json:"nullable"`
}

func Load(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	raw, err := parseSpecData(data, path)
	if err != nil {
		return nil, err
	}

	warnings := sanitizeDocument(raw)

	jsonData, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}

	var doc Document
	if err := json.Unmarshal(jsonData, &doc); err != nil {
		return nil, err
	}
	if doc.OpenAPI == "" && doc.Swagger != "" {
		doc.OpenAPI = doc.Swagger
	}
	if len(doc.Servers) == 0 && doc.Host != "" {
		schemes := doc.Schemes
		if len(schemes) == 0 {
			schemes = []string{"https"}
		}
		for _, scheme := range schemes {
			baseURL := strings.TrimRight(fmt.Sprintf("%s://%s%s", scheme, doc.Host, doc.BasePath), "/")
			doc.Servers = append(doc.Servers, Server{URL: baseURL})
		}
	}
	normalizeResolvedParameters(&doc)
	doc.Warnings = warnings
	return &doc, nil
}

func parseSpecData(data []byte, path string) (any, error) {
	var raw any

	if isYAMLPath(path) {
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("invalid YAML: %w", err)
		}
		return raw, nil
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		if yamlErr := yaml.Unmarshal(data, &raw); yamlErr == nil {
			return raw, nil
		}
		return nil, fmt.Errorf("invalid OpenAPI spec (expected JSON or YAML): %w", err)
	}

	return raw, nil
}

func isYAMLPath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

func (p *PathItem) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	pathParameters := make([]Parameter, 0)
	if rawParameters, ok := raw["parameters"]; ok {
		if err := json.Unmarshal(rawParameters, &pathParameters); err != nil {
			return err
		}
	}

	result := make(PathItem)
	for key, rawValue := range raw {
		method := strings.ToLower(key)
		if _, ok := httpOperationKeys[method]; !ok {
			continue
		}

		var operation Operation
		if err := json.Unmarshal(rawValue, &operation); err != nil {
			return err
		}
		operation.Parameters = mergeParameters(pathParameters, operation.Parameters)
		result[method] = operation
	}

	*p = result
	return nil
}

func (d *Document) ResolveSchema(schema Schema) (Schema, bool) {
	if schema.Ref == "" {
		return schema, true
	}

	return d.ResolveSchemaRef(schema.Ref)
}

func (d *Document) ResolveParameter(parameter Parameter) (Parameter, bool) {
	if parameter.Ref == "" {
		return parameter, true
	}
	return d.ResolveParameterRef(parameter.Ref)
}

func (d *Document) ResolveParameterRef(ref string) (Parameter, bool) {
	const prefix = "#/components/parameters/"
	if !strings.HasPrefix(ref, prefix) {
		return Parameter{}, false
	}
	key := strings.TrimPrefix(ref, prefix)
	if key == "" {
		return Parameter{}, false
	}
	p, ok := d.Components.Parameters[key]
	if !ok {
		return Parameter{}, false
	}
	// Nested parameter $ref is unusual but valid to guard against.
	if p.Ref != "" && p.Ref != ref {
		return d.ResolveParameter(p)
	}
	return p, true
}

func (d *Document) SecurityScheme(name string) (SecurityScheme, bool) {
	if scheme, ok := d.Components.SecuritySchemes[name]; ok {
		return scheme, true
	}
	scheme, ok := d.SecurityDefinitions[name]
	return scheme, ok
}

func (d *Document) ResolveSchemaRef(ref string) (Schema, bool) {
	path := ""
	switch {
	case strings.HasPrefix(ref, "#/components/schemas/"):
		path = strings.TrimPrefix(ref, "#/components/schemas/")
	case strings.HasPrefix(ref, "#/definitions/"):
		path = strings.TrimPrefix(ref, "#/definitions/")
	default:
		return Schema{}, false
	}
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		return Schema{}, false
	}

	current, ok := d.Components.Schemas[parts[0]]
	if !ok {
		current, ok = d.Definitions[parts[0]]
		if !ok {
			return Schema{}, false
		}
	}

	for i := 1; i < len(parts); i++ {
		switch parts[i] {
		case "properties":
			i++
			if i >= len(parts) {
				return Schema{}, false
			}

			next, ok := current.Properties[parts[i]]
			if !ok {
				return Schema{}, false
			}
			current = next
		default:
			return Schema{}, false
		}
	}

	if current.Ref != "" {
		return d.ResolveSchema(current)
	}

	return current, true
}

func (d *Document) DisplayType(schema Schema) string {
	if resolved, ok := d.ResolveSchema(schema); ok && resolved.Type != "" {
		return resolved.Type
	}
	if schema.Type != "" {
		return schema.Type
	}
	if schema.Ref != "" {
		return "object"
	}
	return "unknown"
}

func FormatValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%g", x)
	case []any:
		parts := make([]string, 0, len(x))
		for _, item := range x {
			parts = append(parts, FormatValue(item))
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprintf("%v", x)
	}
}

func unmarshalRawDocument(data []byte) (any, error) {
	var raw any
	jsonErr := json.Unmarshal(data, &raw)
	if jsonErr == nil {
		return raw, nil
	}

	yamlErr := yaml.Unmarshal(data, &raw)
	if yamlErr == nil {
		return raw, nil
	}

	return nil, fmt.Errorf("unsupported OpenAPI file format: expected JSON or YAML (json: %v; yaml: %v)", jsonErr, yamlErr)
}

func mergeParameters(pathParameters []Parameter, operationParameters []Parameter) []Parameter {
	merged := make([]Parameter, 0, len(pathParameters)+len(operationParameters))
	indexes := make(map[string]int, len(pathParameters)+len(operationParameters))

	for _, parameter := range pathParameters {
		key := parameterMergeKey(parameter)
		indexes[key] = len(merged)
		merged = append(merged, parameter)
	}

	for _, parameter := range operationParameters {
		key := parameterMergeKey(parameter)
		if index, ok := indexes[key]; ok {
			merged[index] = parameter
			continue
		}
		indexes[key] = len(merged)
		merged = append(merged, parameter)
	}

	return merged
}

func parameterMergeKey(parameter Parameter) string {
	if parameter.Ref != "" {
		return "ref:" + parameter.Ref
	}
	return parameter.In + "\x00" + parameter.Name
}

func normalizeResolvedParameters(doc *Document) {
	for path, item := range doc.Paths {
		normalized := make(PathItem, len(item))
		for method, operation := range item {
			operation.Parameters = resolveParameters(doc, operation.Parameters)
			normalized[method] = operation
		}
		doc.Paths[path] = normalized
	}
}

func resolveParameters(doc *Document, parameters []Parameter) []Parameter {
	if len(parameters) == 0 {
		return parameters
	}

	resolved := make([]Parameter, 0, len(parameters))
	for _, parameter := range parameters {
		if next, ok := doc.ResolveParameter(parameter); ok {
			if parameter.Ref != "" {
				next.Ref = parameter.Ref
			}
			resolved = append(resolved, next)
			continue
		}
		resolved = append(resolved, parameter)
	}
	return resolved
}

func sanitizeDocument(root any) []string {
	warnings := make([]string, 0)
	sanitizeNode(root, "$", &warnings)
	return dedupeStrings(warnings)
}

func sanitizeNode(node any, path string, warnings *[]string) {
	switch value := node.(type) {
	case map[string]any:
		promoteMalformedRequired(value, path, warnings)
		normalizeMalformedItems(value, path, warnings)
		normalizeMalformedEnum(value, path, warnings)
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			sanitizeNode(value[key], path+"."+key, warnings)
		}
	case []any:
		for index, item := range value {
			sanitizeNode(item, fmt.Sprintf("%s[%d]", path, index), warnings)
		}
	}
}

func promoteMalformedRequired(node map[string]any, path string, warnings *[]string) {
	propertiesRaw, ok := node["properties"]
	if !ok {
		return
	}

	properties, ok := propertiesRaw.(map[string]any)
	if !ok {
		return
	}

	requiredList, requiredSet := collectRequiredNames(node["required"])
	for propertyName, propertyValue := range properties {
		propertySchema, ok := propertyValue.(map[string]any)
		if !ok {
			continue
		}

		requiredRaw, ok := propertySchema["required"]
		if !ok {
			continue
		}

		requiredBool, ok := requiredRaw.(bool)
		if !ok {
			continue
		}

		delete(propertySchema, "required")
		if requiredBool && !requiredSet[propertyName] {
			requiredList = append(requiredList, propertyName)
			requiredSet[propertyName] = true
		}

		*warnings = append(
			*warnings,
			fmt.Sprintf(
				"%s.properties.%s.required is boolean; schema-level required must be an array of property names. Promoted %q to %s.required and ignored the boolean value %t",
				path,
				propertyName,
				propertyName,
				path,
				requiredBool,
			),
		)
	}

	if len(requiredList) > 0 {
		node["required"] = requiredList
	}
}

func normalizeMalformedItems(node map[string]any, path string, warnings *[]string) {
	itemsRaw, ok := node["items"]
	if !ok {
		return
	}
	if _, ok := itemsRaw.(map[string]any); ok {
		return
	}

	replacement := map[string]any{}
	if typeName, ok := itemsRaw.(string); ok {
		switch typeName {
		case "string", "integer", "number", "boolean", "object", "array":
			replacement["type"] = typeName
		}
	}
	node["items"] = replacement
	*warnings = append(
		*warnings,
		fmt.Sprintf(
			"%s.items is %T; schema items must be an object. Replaced it with %s",
			path,
			itemsRaw,
			formatReplacementSchema(replacement),
		),
	)
}

func normalizeMalformedEnum(node map[string]any, path string, warnings *[]string) {
	enumRaw, ok := node["enum"]
	if !ok {
		return
	}
	if _, ok := enumRaw.([]any); ok {
		return
	}

	enumMap, ok := enumRaw.(map[string]any)
	if !ok {
		return
	}

	keys := make([]string, 0, len(enumMap))
	for key := range enumMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	normalized := make([]any, 0, len(keys))
	for _, key := range keys {
		normalized = append(normalized, key)
	}
	node["enum"] = normalized
	*warnings = append(
		*warnings,
		fmt.Sprintf(
			"%s.enum is an object; schema enum must be an array. Replaced it with the object keys [%s]",
			path,
			strings.Join(keys, ", "),
		),
	)
}

func collectRequiredNames(raw any) ([]any, map[string]bool) {
	requiredList := make([]any, 0)
	requiredSet := make(map[string]bool)
	items, ok := raw.([]any)
	if !ok {
		return requiredList, requiredSet
	}
	for _, item := range items {
		name, ok := item.(string)
		if !ok || name == "" || requiredSet[name] {
			continue
		}
		requiredList = append(requiredList, name)
		requiredSet[name] = true
	}
	return requiredList, requiredSet
}

func dedupeStrings(items []string) []string {
	if len(items) < 2 {
		return items
	}
	seen := make(map[string]bool, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}

func formatReplacementSchema(schema map[string]any) string {
	if len(schema) == 0 {
		return "an empty schema"
	}
	if schemaType, ok := schema["type"].(string); ok {
		return fmt.Sprintf("a schema with type=%q", schemaType)
	}
	return "a normalized schema object"
}
