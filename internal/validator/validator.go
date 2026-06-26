package validator

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"openapi/internal/spec"
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type ValidationResult struct {
	Errors   []ValidationError
	Warnings []string
}

func (r *ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

func ValidateParams(params map[string]interface{}, operation spec.Operation, doc *spec.Document, strict bool) *ValidationResult {
	result := &ValidationResult{
		Errors:   make([]ValidationError, 0),
		Warnings: make([]string, 0),
	}

	requiredParams := make(map[string]bool)
	paramSchemas := make(map[string]spec.Parameter)

	for _, p := range operation.Parameters {
		if p.In == "header" {
			continue
		}
		paramSchemas[p.Name] = p
		if p.Required {
			requiredParams[p.Name] = true
		}
	}

	mergeRequestBodyParams(requiredParams, paramSchemas, operation, doc)

	for name := range requiredParams {
		if _, exists := params[name]; !exists {
			result.Errors = append(result.Errors, ValidationError{
				Field:   name,
				Message: "required parameter is missing",
			})
		}
	}

	for name, value := range params {
		param, exists := paramSchemas[name]
		if !exists {
			if strict {
				result.Errors = append(result.Errors, ValidationError{
					Field:   name,
					Message: "unknown parameter",
				})
			} else {
				result.Warnings = append(result.Warnings, fmt.Sprintf("unknown parameter: %s", name))
			}
			continue
		}
		if param.In == "header" {
			continue
		}

		if err := validateType(name, value, param.Schema); err != nil {
			if strict {
				result.Errors = append(result.Errors, ValidationError{
					Field:   name,
					Message: err.Error(),
				})
			} else {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %s", name, err.Error()))
			}
		}
	}

	return result
}

func validateType(field string, value interface{}, schema spec.Schema) error {
	if schema.Type == "" {
		return nil
	}

	switch schema.Type {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
	case "integer":
		if _, ok := value.(float64); !ok {
			return fmt.Errorf("expected integer, got %T", value)
		}
	case "number":
		if _, ok := value.(float64); !ok {
			return fmt.Errorf("expected number, got %T", value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", value)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("expected array, got %T", value)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("expected object, got %T", value)
		}
	}

	return nil
}

// BuildRequestBodyParams returns only parameters that belong in the request body,
// excluding path, query, and header parameters defined on the operation.
func BuildRequestBodyParams(params map[string]interface{}, operation spec.Operation, doc *spec.Document) map[string]interface{} {
	nonBody := nonBodyParamNames(operation, doc)
	bodyParams := make(map[string]interface{}, len(params))
	for key, value := range params {
		if nonBody[key] {
			continue
		}
		bodyParams[key] = value
	}
	return bodyParams
}

func nonBodyParamNames(operation spec.Operation, doc *spec.Document) map[string]bool {
	names := make(map[string]bool)
	for _, parameter := range operation.Parameters {
		effective := parameter
		if doc != nil {
			if resolved, ok := doc.ResolveParameter(parameter); ok {
				effective = resolved
			}
		}
		switch effective.In {
		case "path", "query", "header":
			names[effective.Name] = true
		}
	}
	return names
}

func BuildFormBody(params map[string]interface{}, operation spec.Operation, doc *spec.Document) (string, error) {
	if operation.RequestBody == nil || len(operation.RequestBody.Content) == 0 {
		return "", nil
	}

	contentType := PreferredContentType(operation.RequestBody.Content)
	if !strings.Contains(contentType, "application/x-www-form-urlencoded") {
		return "", nil
	}

	bodyParams := BuildRequestBodyParams(params, operation, doc)
	values := url.Values{}
	for key, value := range bodyParams {
		values.Set(key, fmt.Sprintf("%v", value))
	}

	return values.Encode(), nil
}

func PreferredContentType(contents map[string]spec.MediaType) string {
	if _, ok := contents["application/x-www-form-urlencoded"]; ok {
		return "application/x-www-form-urlencoded"
	}

	if _, ok := contents["application/json"]; ok {
		return "application/json"
	}

	for ct := range contents {
		return ct
	}

	return ""
}

func ParseParams(paramsStr string, paramsFile string, paramsURL string) (map[string]interface{}, error) {
	sourceCount := 0
	if strings.TrimSpace(paramsStr) != "" {
		sourceCount++
	}
	if strings.TrimSpace(paramsFile) != "" {
		sourceCount++
	}
	if strings.TrimSpace(paramsURL) != "" {
		sourceCount++
	}
	if sourceCount > 1 {
		return nil, fmt.Errorf("use only one of --params, --params-file, or --params-url")
	}

	if paramsStr != "" {
		return parseJSON(paramsStr)
	}

	if paramsFile != "" {
		return parseJSONFile(paramsFile)
	}

	if paramsURL != "" {
		return parseURLParams(paramsURL)
	}

	return nil, nil
}

func parseJSON(data string) (map[string]interface{}, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(data), &params); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return params, nil
}

func parseJSONFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read params file: %w", err)
	}

	var params map[string]interface{}
	if err := json.Unmarshal(data, &params); err != nil {
		return nil, fmt.Errorf("invalid JSON in params file: %w", err)
	}
	return params, nil
}

func parseURLParams(raw string) (map[string]interface{}, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL params: %w", err)
	}

	params := make(map[string]interface{}, len(values))
	for key, list := range values {
		if len(list) == 0 {
			params[key] = ""
			continue
		}
		if len(list) > 1 || strings.HasSuffix(key, "[]") {
			items := make([]interface{}, 0, len(list))
			for _, item := range list {
				items = append(items, item)
			}
			params[key] = items
			continue
		}
		params[key] = list[0]
	}
	return params, nil
}

func mergeRequestBodyParams(requiredParams map[string]bool, paramSchemas map[string]spec.Parameter, operation spec.Operation, doc *spec.Document) {
	if doc == nil || operation.RequestBody == nil || len(operation.RequestBody.Content) == 0 {
		return
	}

	contentType := PreferredContentType(operation.RequestBody.Content)
	media, ok := operation.RequestBody.Content[contentType]
	if !ok {
		return
	}

	schema, ok := doc.ResolveSchema(media.Schema)
	if !ok {
		return
	}

	for _, name := range schema.Required {
		requiredParams[name] = true
	}

	for name, property := range schema.Properties {
		paramSchemas[name] = spec.Parameter{
			Name:     name,
			In:       "body",
			Required: requiredParams[name],
			Schema:   property,
		}
	}
}
