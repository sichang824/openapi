package output

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"openapi/internal/query"
	"openapi/internal/spec"
)

func Render(w io.Writer, doc *spec.Document, results []query.Endpoint, verbosity int) {
	RenderHeader(w, doc)

	for _, ep := range results {
		fmt.Fprintf(w, "%s %s\n", ep.Method, ep.Path)
		fmt.Fprintf(w, "summary: %s\n", oneLineSummary(ep))

		if verbosity >= 1 {
			renderLevel1(w, ep)
		}
		if verbosity >= 2 {
			renderLevel2(w, ep)
		}
		if verbosity >= 3 {
			renderLevel3(w, doc, ep)
		}
		fmt.Fprintln(w, "---")
	}
}

func RenderHeader(w io.Writer, doc *spec.Document) {
	// OpenAPI version
	if doc.OpenAPI != "" {
		fmt.Fprintf(w, "OpenAPI Version: %s\n", doc.OpenAPI)
	}
	fmt.Fprintln(w, strings.Repeat("─", 60))

	// Info section (compact format)
	fmt.Fprint(w, "Info:")
	if doc.Info.Title != "" {
		fmt.Fprintf(w, " %s", doc.Info.Title)
	}
	if doc.Info.Version != "" {
		fmt.Fprintf(w, " (%s)", doc.Info.Version)
	}
	fmt.Fprintln(w)

	if doc.Info.Description != "" {
		// Word wrap description at 75 characters
		wrappedDesc := wrapText(doc.Info.Description, 75)
		for _, line := range wrappedDesc {
			fmt.Fprintf(w, "  %s\n", line)
		}
	}
	fmt.Fprintln(w)

	// Servers section (deduplicated, compact)
	if len(doc.Servers) > 0 {
		fmt.Fprint(w, "Servers:")
		seenServers := make(map[string]bool)
		for _, server := range doc.Servers {
			if !seenServers[server.URL] {
				seenServers[server.URL] = true
				fmt.Fprintf(w, " %s", server.URL)
			}
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w)
	}

	// Tags section (compact)
	if len(doc.Tags) > 0 {
		fmt.Fprint(w, "Tags:")
		for _, tag := range doc.Tags {
			fmt.Fprintf(w, " %s", tag.Name)
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, strings.Repeat("─", 60))
	fmt.Fprintln(w)
}

func oneLineSummary(ep query.Endpoint) string {
	if strings.TrimSpace(ep.Operation.Summary) != "" {
		return ep.Operation.Summary
	}
	return ep.Operation.Description
}

func renderLevel1(w io.Writer, ep query.Endpoint) {
	if ep.Operation.OperationID != "" {
		fmt.Fprintf(w, "operationId: %s\n", ep.Operation.OperationID)
	}
	if len(ep.Operation.Tags) > 0 {
		fmt.Fprintf(w, "tags: %s\n", strings.Join(ep.Operation.Tags, ", "))
	}
	if desc := strings.TrimSpace(ep.Operation.Description); desc != "" && desc != strings.TrimSpace(ep.Operation.Summary) {
		fmt.Fprintf(w, "description: %s\n", desc)
	}
}

func renderLevel2(w io.Writer, ep query.Endpoint) {
	if len(ep.Operation.Parameters) > 0 {
		parts := make([]string, 0, len(ep.Operation.Parameters))
		for _, p := range ep.Operation.Parameters {
			if p.Ref != "" {
				parts = append(parts, fmt.Sprintf("ref:%s", p.Ref))
				continue
			}
			suffix := ""
			if p.Required {
				suffix = "(required)"
			}
			parts = append(parts, fmt.Sprintf("%s:%s%s", p.In, p.Name, suffix))
		}
		fmt.Fprintf(w, "params: %s\n", strings.Join(parts, ", "))
	}
	if ep.Operation.RequestBody != nil && len(ep.Operation.RequestBody.Content) > 0 {
		fmt.Fprintf(w, "requestBody: %s\n", joinKeys(ep.Operation.RequestBody.Content))
	}
	if len(ep.Operation.Responses) > 0 {
		codes := make([]string, 0, len(ep.Operation.Responses))
		for code := range ep.Operation.Responses {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		fmt.Fprintf(w, "responses: %s\n", strings.Join(codes, ", "))
	}
}

func renderLevel3(w io.Writer, doc *spec.Document, ep query.Endpoint) {
	for _, p := range ep.Operation.Parameters {
		effective := p
		if resolvedParam, ok := doc.ResolveParameter(p); ok {
			effective = resolvedParam
		}
		resolved, ok := doc.ResolveSchema(effective.Schema)
		if !ok {
			resolved = effective.Schema
		}
		fmt.Fprintf(
			w,
			"param detail: %s %s required=%t type=%s\n",
			effective.In, effective.Name, effective.Required, doc.DisplayType(effective.Schema),
		)
		renderParameterMetadata(w, doc, p, effective, "  ")
		renderSchemaMetadata(w, resolved, "  ")
	}
	if ep.Operation.RequestBody != nil {
		if ep.Operation.RequestBody.Description != "" {
			fmt.Fprintf(w, "request body description: %s\n", ep.Operation.RequestBody.Description)
		}
		for ct, media := range ep.Operation.RequestBody.Content {
			fmt.Fprintf(w, "request body: content=%s schema=%s\n", ct, schemaRefOrType(media.Schema))
			renderSchemaTree(w, doc, media.Schema, "  ")
		}
	}
	respCodes := make([]string, 0, len(ep.Operation.Responses))
	for code := range ep.Operation.Responses {
		respCodes = append(respCodes, code)
	}
	sort.Strings(respCodes)
	for _, code := range respCodes {
		resp := ep.Operation.Responses[code]
		if len(resp.Content) == 0 && (resp.Schema.Ref != "" || resp.Schema.Type != "" || len(resp.Schema.Properties) > 0 || resp.Schema.Items != nil) {
			contentType := "application/json"
			if len(ep.Operation.Produces) > 0 {
				contentType = ep.Operation.Produces[0]
			}
			fmt.Fprintf(
				w,
				"response: %s content=%s schema=%s\n",
				code, contentType, schemaRefOrType(resp.Schema),
			)
			if resp.Description != "" {
				fmt.Fprintf(w, "response description: %s\n", resp.Description)
			}
			renderSchemaTree(w, doc, resp.Schema, "  ")
			continue
		}
		if len(resp.Content) == 0 {
			if resp.Description != "" {
				fmt.Fprintf(w, "response description: %s\n", resp.Description)
			}
			fmt.Fprintf(w, "response: %s content=none schema=none\n", code)
			continue
		}
		for ct, media := range resp.Content {
			fmt.Fprintf(
				w,
				"response: %s content=%s schema=%s\n",
				code, ct, schemaRefOrType(media.Schema),
			)
			if resp.Description != "" {
				fmt.Fprintf(w, "response description: %s\n", resp.Description)
			}
			renderSchemaTree(w, doc, media.Schema, "  ")
		}
	}
}

func renderParameterMetadata(w io.Writer, doc *spec.Document, original spec.Parameter, effective spec.Parameter, indent string) {
	if original.Ref != "" {
		fmt.Fprintf(w, "%sref: %s\n", indent, original.Ref)
	}
	if effective.Description != "" {
		fmt.Fprintf(w, "%sdescription: %s\n", indent, effective.Description)
	}
	if effective.Example != nil {
		fmt.Fprintf(w, "%sexample: %s\n", indent, spec.FormatValue(effective.Example))
	}
	renderNamedExamples(w, effective.Examples, indent)
	if effective.Deprecated {
		fmt.Fprintf(w, "%sdeprecated: true\n", indent)
	}
	if effective.AllowEmptyValue {
		fmt.Fprintf(w, "%sallowEmptyValue: true\n", indent)
	}
	if effective.Style != "" {
		fmt.Fprintf(w, "%sstyle: %s\n", indent, effective.Style)
	}
	if effective.Explode != nil {
		fmt.Fprintf(w, "%sexplode: %t\n", indent, *effective.Explode)
	}
	if effective.AllowReserved {
		fmt.Fprintf(w, "%sallowReserved: true\n", indent)
	}
	if len(effective.Content) > 0 {
		contentTypes := make([]string, 0, len(effective.Content))
		for ct := range effective.Content {
			contentTypes = append(contentTypes, ct)
		}
		sort.Strings(contentTypes)
		for _, ct := range contentTypes {
			media := effective.Content[ct]
			fmt.Fprintf(w, "%sparameter content: %s schema=%s\n", indent, ct, schemaRefOrType(media.Schema))
			if media.Example != nil {
				fmt.Fprintf(w, "%smedia example: %s\n", indent+"  ", spec.FormatValue(media.Example))
			}
			renderNamedExamples(w, media.Examples, indent+"  ")
			renderSchemaTree(w, doc, media.Schema, indent+"  ")
		}
	}
}

func renderNamedExamples(w io.Writer, examples map[string]spec.Example, indent string) {
	if len(examples) == 0 {
		return
	}
	names := make([]string, 0, len(examples))
	for name := range examples {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		ex := examples[name]
		if ex.Summary != "" {
			fmt.Fprintf(w, "%sexample[%s].summary: %s\n", indent, name, ex.Summary)
		}
		if ex.Description != "" {
			fmt.Fprintf(w, "%sexample[%s].description: %s\n", indent, name, ex.Description)
		}
		if ex.Value != nil {
			fmt.Fprintf(w, "%sexample[%s]: %s\n", indent, name, spec.FormatValue(ex.Value))
			continue
		}
		if ex.ExternalValue != "" {
			fmt.Fprintf(w, "%sexample[%s].externalValue: %s\n", indent, name, ex.ExternalValue)
		}
	}
}

func schemaRefOrType(s spec.Schema) string {
	if s.Ref != "" {
		return s.Ref
	}
	if s.Type != "" {
		return s.Type
	}
	return "unknown"
}

func renderSchemaTree(w io.Writer, doc *spec.Document, schema spec.Schema, indent string) {
	resolved, ok := doc.ResolveSchema(schema)
	if !ok {
		resolved = schema
	}

	renderSchemaHeader(w, resolved, indent)
	renderSchemaChildren(w, doc, resolved, indent)
}

func renderSchemaHeader(w io.Writer, schema spec.Schema, indent string) {
	if len(schema.Required) > 0 {
		fmt.Fprintf(w, "%sschema required: %s\n", indent, strings.Join(schema.Required, ", "))
	}

	renderSchemaMetadata(w, schema, indent)
}

func renderSchemaChildren(w io.Writer, doc *spec.Document, schema spec.Schema, indent string) {
	if len(schema.Properties) == 0 {
		if schema.Items != nil {
			fmt.Fprintf(w, "%sitems:\n", indent)
			renderSchemaTree(w, doc, *schema.Items, indent+"  ")
		}
		return
	}

	names := make([]string, 0, len(schema.Properties))
	for name := range schema.Properties {
		names = append(names, name)
	}
	sort.Strings(names)

	required := make(map[string]bool, len(schema.Required))
	for _, name := range schema.Required {
		required[name] = true
	}

	for _, name := range names {
		prop := schema.Properties[name]
		fmt.Fprintf(
			w,
			"%sproperty: %s type=%s required=%t",
			indent,
			name,
			doc.DisplayType(prop),
			required[name],
		)
		if prop.Ref != "" {
			fmt.Fprintf(w, " ref=%s", prop.Ref)
		}
		fmt.Fprintln(w)
		propResolved, ok := doc.ResolveSchema(prop)
		if !ok {
			propResolved = prop
		}
		renderSchemaMetadata(w, propResolved, indent+"  ")
		if len(propResolved.Properties) > 0 || propResolved.Items != nil {
			renderSchemaChildren(w, doc, propResolved, indent+"  ")
		}
	}
}

func renderSchemaMetadata(w io.Writer, schema spec.Schema, indent string) {
	if schema.Description != "" {
		fmt.Fprintf(w, "%sdescription: %s\n", indent, schema.Description)
	}
	if schema.Example != nil {
		fmt.Fprintf(w, "%sexample: %s\n", indent, spec.FormatValue(schema.Example))
	}
	if len(schema.Enum) > 0 {
		fmt.Fprintf(w, "%senum: %s\n", indent, spec.FormatValue(schema.Enum))
	}
	if schema.Default != nil {
		fmt.Fprintf(w, "%sdefault: %s\n", indent, spec.FormatValue(schema.Default))
	}
	if schema.Maximum != nil {
		fmt.Fprintf(w, "%smaximum: %s\n", indent, spec.FormatValue(*schema.Maximum))
	}
	if schema.Nullable {
		fmt.Fprintf(w, "%snullable: true\n", indent)
	}
	if schema.AdditionalProperties != nil {
		fmt.Fprintf(w, "%sadditionalProperties: %s\n", indent, spec.FormatValue(schema.AdditionalProperties))
	}
}

func joinKeys[M ~map[string]T, T any](m M) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

func wrapText(text string, maxWidth int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{}
	}

	var lines []string
	currentLine := words[0]

	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) <= maxWidth {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}
	lines = append(lines, currentLine)

	return lines
}
