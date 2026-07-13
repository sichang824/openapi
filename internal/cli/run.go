package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"openapi/internal/autoheaders"
	"openapi/internal/caller"
	"openapi/internal/output"
	"openapi/internal/query"
	"openapi/internal/spec"
	"openapi/internal/validator"
)

type queryCallExample struct {
	Method  string
	Path    string
	Summary string
	Command string
}

type queryOptions struct {
	File       string
	Name       string
	Keyword    string
	Limit      int
	Offset     int
	JSONOutput bool
	Verbose    int
}

const specsDirEnv = "OAPI_SPECS_DIR"

type specInput struct {
	Path  string
	Flag  string
	Value string
}

func resolveSpecInput(file, name string) (specInput, error) {
	file = strings.TrimSpace(file)
	name = strings.TrimSpace(name)
	if file != "" && name != "" {
		return specInput{}, errors.New("use only one of -f or --name")
	}
	if name == "" {
		if file == "" {
			file = "openapi.json"
		}
		return specInput{Path: file, Flag: "-f", Value: file}, nil
	}
	if name == "." || name == ".." || strings.ContainsAny(name, `/\\`) {
		return specInput{}, fmt.Errorf("invalid spec name %q: use a file name without path separators", name)
	}

	dir := strings.TrimSpace(os.Getenv(specsDirEnv))
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return specInput{}, fmt.Errorf("resolve home directory: %w", err)
		}
		dir = filepath.Join(home, ".openapi", "specs")
	}
	return specInput{
		Path:  filepath.Join(dir, name+".openapi.yaml"),
		Flag:  "-n",
		Value: name,
	}, nil
}

func resolveSpecFile(file, name string) (string, error) {
	input, err := resolveSpecInput(file, name)
	return input.Path, err
}

func executeQuery(opts queryOptions, stdout io.Writer, stderr io.Writer) error {
	if opts.Limit < 1 {
		return errors.New("limit must be greater than 0")
	}
	if opts.Offset < 0 {
		return errors.New("offset must be greater than or equal to 0")
	}

	input, err := resolveSpecInput(opts.File, opts.Name)
	if err != nil {
		return err
	}
	doc, err := spec.Load(input.Path)
	if err != nil {
		return err
	}
	printSpecWarnings(stderr, doc.Warnings)

	var allResults []query.Endpoint
	if strings.TrimSpace(opts.Keyword) == "" {
		allResults = query.ListAll(doc)
	} else {
		allResults = query.Search(doc, opts.Keyword, 0)
	}

	results, page := paginateEndpoints(allResults, opts.Limit, opts.Offset)
	level := opts.Verbose
	if level < 0 {
		level = 0
	}
	if level > 3 {
		level = 3
	}

	if opts.JSONOutput {
		return writeQueryJSON(stdout, doc, input, opts.Keyword, results, allResults, page, level)
	}

	if len(allResults) == 0 {
		output.RenderHeader(stdout, doc)
		_, _ = fmt.Fprintln(stdout, "no matched endpoints")
		_, _ = fmt.Fprintln(stdout)
		_, _ = fmt.Fprintln(stdout, "available endpoints:")
		allEndpoints := query.ListAll(doc)
		for _, ep := range allEndpoints {
			_, _ = fmt.Fprintf(stdout, "  %s %s\n", ep.Method, ep.Path)
		}
		return nil
	}
	if len(results) == 0 {
		output.RenderHeader(stdout, doc)
		_, _ = fmt.Fprintf(stdout, "no endpoints in current window (offset=%d, limit=%d, total=%d)\n", opts.Offset, opts.Limit, len(allResults))
		_, _ = fmt.Fprintln(stdout)
		renderQueryPaginationHint(stdout, input, opts.Keyword, page)
		return nil
	}
	output.Render(stdout, doc, results, level)
	renderQueryPaginationHint(stdout, input, opts.Keyword, page)
	renderQueryCallExamples(stdout, doc, input, results)
	return nil
}

type callOptions struct {
	File           string
	Name           string
	Endpoint       string
	BaseURL        string
	Params         string
	ParamsFile     string
	ParamsURL      string
	BodyFile       string
	ContentType    string
	Cookie         string
	CookiePath     string
	Headers        []string
	BearerToken    string
	AutoHeaders    bool
	AutoHeadersSet bool
	Strict         bool
	Verbose        int
}

func executeCall(opts callOptions, stdout io.Writer, stderr io.Writer) error {
	autoHeadersEnabled, err := autoheaders.ResolveEnabled(opts.AutoHeaders, opts.AutoHeadersSet, os.LookupEnv)
	if err != nil {
		return err
	}

	resolvedCookiePath := strings.TrimSpace(opts.CookiePath)

	if strings.TrimSpace(opts.Cookie) != "" && resolvedCookiePath != "" {
		return errors.New("use only one of --cookie or --cookie-path")
	}

	headers, err := parseCallHeaders(opts.Headers, opts.BearerToken)
	if err != nil {
		return err
	}

	cookieHeader := strings.TrimSpace(opts.Cookie)
	if resolvedCookiePath != "" {
		cookieHeader, err = readNetscapeCookieJar(resolvedCookiePath)
		if err != nil {
			return err
		}
	}

	if strings.TrimSpace(opts.Endpoint) == "" {
		return errors.New("endpoint is required, use -e")
	}

	file, err := resolveSpecFile(opts.File, opts.Name)
	if err != nil {
		return err
	}
	doc, err := spec.Load(file)
	if err != nil {
		return err
	}
	printSpecWarnings(stderr, doc.Warnings)

	resolvedBaseURL := strings.TrimSpace(opts.BaseURL)
	if resolvedBaseURL == "" && len(doc.Servers) > 0 {
		resolvedBaseURL = strings.TrimSpace(doc.Servers[0].URL)
	}
	if resolvedBaseURL == "" {
		return errors.New("base URL is required: pass --base-url or define at least one server URL in the OpenAPI spec")
	}

	method, path := parseEndpoint(opts.Endpoint)
	operation, pathParams, found := findOperation(doc, method, path)
	if !found {
		return fmt.Errorf("endpoint not found: %s %s", method, path)
	}

	resolvedAutoHeaders := make([]caller.ResolvedHeader, 0)
	if autoHeadersEnabled {
		candidates, err := autoheaders.ScanEnvironment(os.Environ())
		if err != nil {
			return err
		}
		resolution := autoheaders.Resolve(doc, operation, candidates, headers)
		if resolution.SecurityRequired && !resolution.SecuritySatisfied {
			message := "automatic headers could not satisfy the operation's OpenAPI security requirements"
			if opts.Strict {
				return errors.New(message)
			}
			_, _ = fmt.Fprintf(stderr, "Warning: %s; continuing without automatic security headers\n", message)
		}
		for _, header := range resolution.Headers {
			resolvedAutoHeaders = append(resolvedAutoHeaders, caller.ResolvedHeader{
				Name: header.Name, Value: header.Value, Source: header.Source, Secret: header.Secret,
			})
		}
	}
	if len(resolvedAutoHeaders) > 0 {
		specServer := ""
		if len(doc.Servers) > 0 {
			specServer = strings.TrimSpace(doc.Servers[0].URL)
		}
		originWarning, err := autoheaders.ValidateOrigin(specServer, resolvedBaseURL)
		if err != nil {
			return err
		}
		if originWarning != "" {
			_, _ = fmt.Fprintf(stderr, "Warning: %s; automatic headers will be sent to %s\n", originWarning, requestOrigin(resolvedBaseURL))
		}
	}

	params, err := validator.ParseParams(opts.Params, opts.ParamsFile, opts.ParamsURL)
	if err != nil {
		return err
	}

	if params == nil {
		params = make(map[string]interface{})
	}
	mergeResolvedPathParams(params, pathParams)

	validationResult := validator.ValidateParams(params, operation, doc, opts.Strict)
	if validationResult.HasErrors() {
		_, _ = fmt.Fprintln(stderr, "Parameter validation failed:")
		for _, e := range validationResult.Errors {
			fmt.Fprintf(stderr, "  - %s: %s\n", e.Field, e.Message)
		}
		return errors.New("validation failed")
	}

	if len(validationResult.Warnings) > 0 {
		_, _ = fmt.Fprintln(stderr, "Parameter validation warnings:")
		for _, w := range validationResult.Warnings {
			fmt.Fprintf(stderr, "  - %s\n", w)
		}
	}

	contentType := strings.TrimSpace(opts.ContentType)
	var body []byte
	resolvedBodyFile := strings.TrimSpace(opts.BodyFile)

	if resolvedBodyFile != "" {
		body, err = os.ReadFile(resolvedBodyFile)
		if err != nil {
			return fmt.Errorf("read body-file: %w", err)
		}
		if contentType == "" && operation.RequestBody != nil && len(operation.RequestBody.Content) > 0 {
			contentType = validator.PreferredContentType(operation.RequestBody.Content)
		}
	} else if operation.RequestBody != nil && len(operation.RequestBody.Content) > 0 {
		if contentType == "" {
			contentType = validator.PreferredContentType(operation.RequestBody.Content)
		}
		if strings.Contains(contentType, "application/x-www-form-urlencoded") {
			formBody, err := validator.BuildFormBody(params, operation, doc)
			if err != nil {
				return err
			}
			body = []byte(formBody)
		} else if strings.Contains(contentType, "application/json") {
			bodyParams := validator.BuildRequestBodyParams(params, operation, doc)
			bodyData, err := json.Marshal(bodyParams)
			if err != nil {
				return err
			}
			body = bodyData
		}
	}

	callReq := &caller.CallRequest{
		BaseURL:     resolvedBaseURL,
		Method:      method,
		Path:        path,
		Params:      params,
		Operation:   operation,
		ContentType: contentType,
		Body:        body,
		Cookie:      cookieHeader,
		Headers:     headers,
		AutoHeaders: resolvedAutoHeaders,
	}

	if len(resolvedAutoHeaders) > 0 {
		names := make([]string, 0, len(resolvedAutoHeaders))
		for _, header := range resolvedAutoHeaders {
			names = append(names, header.Name)
		}
		_, _ = fmt.Fprintf(stderr, "Auto headers enabled: %s (environment, redacted) -> %s\n", strings.Join(names, ", "), requestOrigin(resolvedBaseURL))
	}

	callResp, err := caller.Call(callReq)
	if err != nil {
		return err
	}

	if opts.Verbose >= 1 {
		_, _ = fmt.Fprintf(stdout, "Calling: %s %s\n", method, path)
		fmt.Fprintf(stdout, "Base URL: %s\n", resolvedBaseURL)
		if opts.Verbose >= 3 && callResp.URL != "" {
			fmt.Fprintf(stdout, "URL: %s\n", callResp.URL)
		}
		if opts.Verbose >= 3 && contentType != "" {
			fmt.Fprintf(stdout, "Content-Type: %s\n", contentType)
		}
		if opts.Verbose >= 3 && len(body) > 0 {
			fmt.Fprintf(stdout, "Body: %s\n", formatRequestBodyForVerbose(body))
		}
		if opts.Verbose >= 3 && cookieHeader != "" {
			fmt.Fprintf(stdout, "Cookie: <redacted; length=%d>\n", len(cookieHeader))
		}
		_, _ = fmt.Fprintln(stdout)
	}

	_, _ = fmt.Fprintln(stdout, caller.FormatResponse(callResp, opts.Verbose))
	return nil
}

func requestOrigin(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return rawURL
	}
	return parsed.Scheme + "://" + parsed.Host
}

func parseEndpoint(endpoint string) (string, string) {
	parts := strings.Fields(endpoint)
	if len(parts) != 2 {
		return "", ""
	}
	return strings.ToUpper(parts[0]), parts[1]
}

func findOperation(doc *spec.Document, method, path string) (spec.Operation, map[string]interface{}, bool) {
	pathItem, pathExists := doc.Paths[path]
	if !pathExists {
		return findTemplatedOperation(doc, method, path)
	}

	methodLower := strings.ToLower(method)
	operation, exists := pathItem[methodLower]
	return operation, nil, exists
}

func findTemplatedOperation(doc *spec.Document, method, path string) (spec.Operation, map[string]interface{}, bool) {
	methodLower := strings.ToLower(method)
	paths := make([]string, 0, len(doc.Paths))
	for template := range doc.Paths {
		paths = append(paths, template)
	}
	sort.Strings(paths)

	bestScore := -1
	var bestOperation spec.Operation
	var bestParams map[string]interface{}

	for _, template := range paths {
		pathItem := doc.Paths[template]
		operation, exists := pathItem[methodLower]
		if !exists {
			continue
		}

		params, score, matched := matchPathTemplate(template, path, operation)
		if !matched {
			continue
		}
		if score > bestScore {
			bestScore = score
			bestOperation = operation
			bestParams = params
		}
	}

	if bestScore < 0 {
		return spec.Operation{}, nil, false
	}

	return bestOperation, bestParams, true
}

func matchPathTemplate(templatePath, actualPath string, operation spec.Operation) (map[string]interface{}, int, bool) {
	templateSegments := splitPathSegments(templatePath)
	actualSegments := splitPathSegments(actualPath)
	if len(templateSegments) != len(actualSegments) {
		return nil, 0, false
	}

	pathParameters := make(map[string]spec.Parameter)
	for _, parameter := range operation.Parameters {
		if parameter.In == "path" {
			pathParameters[parameter.Name] = parameter
		}
	}

	params := make(map[string]interface{})
	score := 0
	for index, templateSegment := range templateSegments {
		actualSegment := actualSegments[index]
		parameterName, isParameter := pathParameterName(templateSegment)
		if isParameter {
			params[parameterName] = coercePathParamValue(actualSegment, pathParameters[parameterName])
			continue
		}
		if templateSegment != actualSegment {
			return nil, 0, false
		}
		score += len(templateSegment) + 1
	}

	return params, score, true
}

func splitPathSegments(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func pathParameterName(segment string) (string, bool) {
	if len(segment) < 3 || segment[0] != '{' || segment[len(segment)-1] != '}' {
		return "", false
	}
	name := strings.TrimSpace(segment[1 : len(segment)-1])
	if name == "" {
		return "", false
	}
	return name, true
}

func coercePathParamValue(raw string, parameter spec.Parameter) interface{} {
	switch parameter.Schema.Type {
	case "integer", "number":
		if num, err := strconv.ParseFloat(raw, 64); err == nil {
			return num
		}
	case "boolean":
		if value, err := strconv.ParseBool(raw); err == nil {
			return value
		}
	}
	return raw
}

func mergeResolvedPathParams(params map[string]interface{}, pathParams map[string]interface{}) {
	for key, value := range pathParams {
		params[key] = value
	}
}

func parseCallHeaders(rawHeaders []string, bearerToken string) (http.Header, error) {
	headers := make(http.Header)
	for _, rawHeader := range rawHeaders {
		name, value, ok := strings.Cut(rawHeader, ":")
		if !ok {
			return nil, fmt.Errorf("invalid header %q: expected 'Name: Value'", rawHeader)
		}

		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if name == "" {
			return nil, fmt.Errorf("invalid header %q: header name is empty", rawHeader)
		}

		headers.Add(name, value)
	}

	trimmedToken := strings.TrimSpace(bearerToken)
	if trimmedToken == "" {
		return headers, nil
	}
	if callHeadersContain(headers, "Authorization") {
		return nil, errors.New("use only one of --bearer-token or --header 'Authorization: ...'")
	}
	headers.Set("Authorization", "Bearer "+trimmedToken)
	return headers, nil
}

func callHeadersContain(headers http.Header, name string) bool {
	for key := range headers {
		if strings.EqualFold(key, name) {
			return true
		}
	}
	return false
}

func readNetscapeCookieJar(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read cookie-path: %w", err)
	}
	defer file.Close()

	parts := make([]string, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#HttpOnly_") {
			line = strings.TrimPrefix(line, "#HttpOnly_")
		} else if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 7 {
			return "", fmt.Errorf("read cookie-path: invalid Netscape cookie jar line %q", line)
		}
		name := strings.TrimSpace(fields[5])
		value := strings.TrimSpace(fields[6])
		if name == "" || value == "" {
			continue
		}
		parts = append(parts, name+"="+value)
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read cookie-path: %w", err)
	}
	if len(parts) == 0 {
		return "", errors.New("read cookie-path: no cookies found in Netscape cookie jar; use --cookie for raw Cookie header strings")
	}
	return strings.Join(parts, "; "), nil
}

func printSpecWarnings(w io.Writer, warnings []string) {
	if len(warnings) == 0 {
		return
	}

	const maxWarnings = 10
	_, _ = fmt.Fprintf(w, "Spec compatibility warnings (%d):\n", len(warnings))
	limit := len(warnings)
	if limit > maxWarnings {
		limit = maxWarnings
	}
	for i := 0; i < limit; i++ {
		_, _ = fmt.Fprintf(w, "  - %s\n", warnings[i])
	}
	if len(warnings) > maxWarnings {
		_, _ = fmt.Fprintf(w, "  - ... %d more warnings omitted\n", len(warnings)-maxWarnings)
	}
	_, _ = fmt.Fprintln(w)
}

type queryPage struct {
	Total  int
	Limit  int
	Offset int
	Shown  int
	Next   int
	Prev   int
}

func paginateEndpoints(endpoints []query.Endpoint, limit, offset int) ([]query.Endpoint, queryPage) {
	page := queryPage{
		Total:  len(endpoints),
		Limit:  limit,
		Offset: offset,
		Next:   -1,
		Prev:   -1,
	}
	if offset >= len(endpoints) {
		if offset > 0 {
			page.Prev = max(offset-limit, 0)
		}
		return nil, page
	}

	end := offset + limit
	if end > len(endpoints) {
		end = len(endpoints)
	}
	page.Shown = end - offset
	if end < len(endpoints) {
		page.Next = end
	}
	if offset > 0 {
		page.Prev = max(offset-limit, 0)
	}
	return endpoints[offset:end], page
}

func renderQueryPaginationHint(w io.Writer, input specInput, keyword string, page queryPage) {
	if page.Total == 0 {
		return
	}
	quote := func(value string) string { return shellQuoteForGOOS(value, queryCallGOOS()) }
	start := 0
	if page.Shown > 0 {
		start = page.Offset + 1
	}
	end := page.Offset + page.Shown
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "Showing %d-%d of %d endpoints (limit=%d, offset=%d)\n", start, end, page.Total, page.Limit, page.Offset)
	if page.Next >= 0 {
		_, _ = fmt.Fprintf(w, "Next page: oapi query %s %s --limit %d --offset %d", input.Flag, quote(input.Value), page.Limit, page.Next)
		if strings.TrimSpace(keyword) != "" {
			_, _ = fmt.Fprintf(w, " -q %s", quote(keyword))
		}
		_, _ = fmt.Fprintln(w)
	}
	if page.Prev >= 0 {
		_, _ = fmt.Fprintf(w, "Previous page: oapi query %s %s --limit %d --offset %d", input.Flag, quote(input.Value), page.Limit, page.Prev)
		if strings.TrimSpace(keyword) != "" {
			_, _ = fmt.Fprintf(w, " -q %s", quote(keyword))
		}
		_, _ = fmt.Fprintln(w)
	}
	if strings.TrimSpace(keyword) == "" {
		_, _ = fmt.Fprintln(w, "Tip: use -q <keyword> to narrow results, for example: -q order")
	} else {
		_, _ = fmt.Fprintf(w, "Tip: refine further with -q, current keyword=%s\n", quote(keyword))
	}
}

func renderQueryCallExamples(w io.Writer, doc *spec.Document, input specInput, results []query.Endpoint) {
	if len(results) == 0 {
		return
	}

	examples := buildQueryCallExamples(doc, input, results)
	if len(examples) == 0 {
		return
	}

	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Call examples:")
	for _, example := range examples {
		if strings.TrimSpace(example.Summary) != "" {
			_, _ = fmt.Fprintf(w, "# %s\n", example.Summary)
		}
		_, _ = fmt.Fprintf(w, "%s\n", example.Command)
	}
}

func buildQueryCallExamples(doc *spec.Document, input specInput, results []query.Endpoint) []queryCallExample {
	examples := make([]queryCallExample, 0, len(results))
	for _, endpoint := range results {
		command := buildQueryCallCommand(doc, input, endpoint)
		examples = append(examples, queryCallExample{
			Method:  endpoint.Method,
			Path:    endpoint.Path,
			Summary: querySummary(endpoint),
			Command: command,
		})
	}
	return examples
}

func buildQueryCallCommand(doc *spec.Document, input specInput, endpoint query.Endpoint) string {
	goos := queryCallGOOS()
	quote := func(value string) string { return shellQuoteForGOOS(value, goos) }
	command := fmt.Sprintf("oapi call %s %s -e %s", input.Flag, quote(input.Value), quote(endpoint.Method+" "+endpoint.Path))
	params := buildExampleParams(doc, endpoint.Operation)
	if len(params) == 0 {
		return command
	}
	if queryCallParamFlag(goos) == "--params-url" {
		payload := marshalShellQuery(params)
		if payload == "" {
			return command
		}
		return fmt.Sprintf("%s --params-url %s", command, quote(payload))
	}

	payload, err := marshalShellJSON(params)
	if err != nil {
		return command
	}
	return fmt.Sprintf("%s --params %s", command, quote(string(payload)))
}

func marshalShellJSON(value interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func queryCallParamFlag(goos string) string {
	if goos == "windows" {
		return "--params-url"
	}
	return "--params"
}

func queryCallGOOS() string {
	if override := strings.TrimSpace(os.Getenv("OAPI_QUERY_CALL_GOOS")); override != "" {
		return strings.ToLower(override)
	}
	return runtime.GOOS
}

func marshalShellQuery(params map[string]interface{}) string {
	values := url.Values{}
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		appendQueryParam(values, key, params[key])
	}
	return values.Encode()
}

func appendQueryParam(values url.Values, key string, value interface{}) {
	switch typed := value.(type) {
	case []interface{}:
		for _, item := range typed {
			values.Add(key, queryParamValue(item))
		}
	case []string:
		for _, item := range typed {
			values.Add(key, item)
		}
	default:
		values.Add(key, queryParamValue(value))
	}
}

func queryParamValue(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case int16:
		return strconv.FormatInt(int64(typed), 10)
	case int8:
		return strconv.FormatInt(int64(typed), 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case uint32:
		return strconv.FormatUint(uint64(typed), 10)
	case uint16:
		return strconv.FormatUint(uint64(typed), 10)
	case uint8:
		return strconv.FormatUint(uint64(typed), 10)
	default:
		payload, err := marshalShellJSON(value)
		if err != nil {
			return fmt.Sprintf("%v", value)
		}
		return string(payload)
	}
}

func buildExampleParams(doc *spec.Document, operation spec.Operation) map[string]interface{} {
	params := make(map[string]interface{})
	for _, parameter := range operation.Parameters {
		effective := parameter
		if resolved, ok := doc.ResolveParameter(parameter); ok {
			effective = resolved
		}
		if effective.In != "path" && effective.In != "query" {
			continue
		}
		if !shouldIncludeParameterExample(effective, doc) {
			continue
		}
		value, ok := exampleValueForParameter(effective, doc)
		if !ok {
			continue
		}
		params[effective.Name] = value
	}

	mergeRequestBodyExampleParams(params, doc, operation)
	return params
}

func shouldIncludeParameterExample(parameter spec.Parameter, doc *spec.Document) bool {
	if parameter.Required {
		return true
	}
	if _, ok := exampleValueForParameter(parameter, doc); ok {
		return true
	}
	return false
}

func exampleValueForParameter(parameter spec.Parameter, doc *spec.Document) (interface{}, bool) {
	if parameter.Example != nil {
		return parameter.Example, true
	}
	if value, ok := firstNamedExampleValue(parameter.Examples); ok {
		return value, true
	}
	if value, ok := firstMediaExampleValue(parameter.Content); ok {
		return value, true
	}
	return exampleValueForSchema(doc, parameter.Schema, parameter.Name, parameter.Required)
}

func mergeRequestBodyExampleParams(params map[string]interface{}, doc *spec.Document, operation spec.Operation) {
	if operation.RequestBody == nil || len(operation.RequestBody.Content) == 0 {
		return
	}

	contentType := validator.PreferredContentType(operation.RequestBody.Content)
	media, ok := operation.RequestBody.Content[contentType]
	if !ok {
		return
	}

	value, ok := exampleValueForSchema(doc, media.Schema, "body", true)
	if !ok {
		return
	}

	bodyMap, ok := value.(map[string]interface{})
	if !ok {
		return
	}
	for key, bodyValue := range bodyMap {
		params[key] = bodyValue
	}
}

func exampleValueForSchema(doc *spec.Document, schema spec.Schema, name string, includePlaceholders bool) (interface{}, bool) {
	resolved, ok := doc.ResolveSchema(schema)
	if ok {
		schema = resolved
	}

	if schema.Default != nil {
		return schema.Default, true
	}
	if schema.Example != nil {
		return schema.Example, true
	}
	if len(schema.Enum) > 0 {
		return schema.Enum[0], true
	}

	switch schema.Type {
	case "object":
		result := make(map[string]interface{})
		required := make(map[string]bool, len(schema.Required))
		for _, propertyName := range schema.Required {
			required[propertyName] = true
		}
		propertyNames := make([]string, 0, len(schema.Properties))
		for propertyName := range schema.Properties {
			propertyNames = append(propertyNames, propertyName)
		}
		sort.Strings(propertyNames)
		for _, propertyName := range propertyNames {
			propertySchema := schema.Properties[propertyName]
			value, valueOK := exampleValueForSchema(doc, propertySchema, propertyName, required[propertyName])
			if !valueOK {
				continue
			}
			if required[propertyName] || hasSchemaDefaultOrExample(doc, propertySchema) {
				result[propertyName] = value
			}
		}
		if len(result) > 0 {
			return result, true
		}
		if includePlaceholders {
			return map[string]interface{}{}, true
		}
		return nil, false
	case "array":
		if schema.Items != nil {
			itemValue, itemOK := exampleValueForSchema(doc, *schema.Items, name+"Item", false)
			if itemOK {
				return []interface{}{itemValue}, true
			}
		}
		if includePlaceholders {
			return []interface{}{}, true
		}
		return nil, false
	case "integer":
		if includePlaceholders {
			return 1, true
		}
	case "number":
		if includePlaceholders {
			return 1, true
		}
	case "boolean":
		if includePlaceholders {
			return true, true
		}
	case "string":
		if includePlaceholders {
			return fmt.Sprintf("<%s>", name), true
		}
	default:
		if includePlaceholders {
			return fmt.Sprintf("<%s>", name), true
		}
	}

	return nil, false
}

func hasSchemaDefaultOrExample(doc *spec.Document, schema spec.Schema) bool {
	resolved, ok := doc.ResolveSchema(schema)
	if ok {
		schema = resolved
	}
	if schema.Default != nil || schema.Example != nil || len(schema.Enum) > 0 {
		return true
	}
	if schema.Type == "object" {
		for _, propertySchema := range schema.Properties {
			if hasSchemaDefaultOrExample(doc, propertySchema) {
				return true
			}
		}
	}
	return false
}

func firstNamedExampleValue(examples map[string]spec.Example) (interface{}, bool) {
	if len(examples) == 0 {
		return nil, false
	}
	names := make([]string, 0, len(examples))
	for name := range examples {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if examples[name].Value != nil {
			return examples[name].Value, true
		}
	}
	return nil, false
}

func firstMediaExampleValue(contents map[string]spec.MediaType) (interface{}, bool) {
	if len(contents) == 0 {
		return nil, false
	}
	contentTypes := make([]string, 0, len(contents))
	for contentType := range contents {
		contentTypes = append(contentTypes, contentType)
	}
	sort.Strings(contentTypes)
	for _, contentType := range contentTypes {
		media := contents[contentType]
		if media.Example != nil {
			return media.Example, true
		}
		if value, ok := firstNamedExampleValue(media.Examples); ok {
			return value, true
		}
	}
	return nil, false
}

func shellQuote(value string) string {
	return shellQuoteForGOOS(value, "")
}

func shellQuoteForGOOS(value, goos string) string {
	if strings.EqualFold(goos, "windows") {
		return powerShellQuote(value)
	}
	return posixShellQuote(value)
}

func posixShellQuote(value string) string {
	if value == "" {
		return "''"
	}
	escaped := strings.ReplaceAll(value, "'", "'\"'\"'")
	return "'" + escaped + "'"
}

func powerShellQuote(value string) string {
	if value == "" {
		return "''"
	}
	escaped := strings.ReplaceAll(value, "'", "''")
	return "'" + escaped + "'"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type queryJSONOutput struct {
	File      string            `json:"file"`
	Keyword   string            `json:"keyword,omitempty"`
	Verbosity int               `json:"verbosity"`
	Page      queryJSONPage     `json:"page"`
	Hints     queryJSONHints    `json:"hints"`
	Results   []queryJSONResult `json:"results"`
	Servers   []string          `json:"servers,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
	Info      queryJSONInfo     `json:"info"`
}

type queryJSONInfo struct {
	Title          string `json:"title,omitempty"`
	Version        string `json:"version,omitempty"`
	Description    string `json:"description,omitempty"`
	OpenAPIVersion string `json:"openapiVersion,omitempty"`
}

type queryJSONPage struct {
	Total      int  `json:"total"`
	Count      int  `json:"count"`
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
	HasNext    bool `json:"hasNext"`
	HasPrev    bool `json:"hasPrev"`
	NextOffset int  `json:"nextOffset,omitempty"`
	PrevOffset int  `json:"prevOffset,omitempty"`
	Start      int  `json:"start,omitempty"`
	End        int  `json:"end,omitempty"`
}

type queryJSONHints struct {
	NextPageCommand     string `json:"nextPageCommand,omitempty"`
	PreviousPageCommand string `json:"previousPageCommand,omitempty"`
	SearchTip           string `json:"searchTip,omitempty"`
}

type queryJSONResult struct {
	Method      string                   `json:"method"`
	Path        string                   `json:"path"`
	Summary     string                   `json:"summary,omitempty"`
	Description string                   `json:"description,omitempty"`
	OperationID string                   `json:"operationId,omitempty"`
	Tags        []string                 `json:"tags,omitempty"`
	Parameters  []spec.Parameter         `json:"parameters,omitempty"`
	RequestBody *spec.RequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]spec.Response `json:"responses,omitempty"`
	Produces    []string                 `json:"produces,omitempty"`
	Consumes    []string                 `json:"consumes,omitempty"`
	Score       int                      `json:"score,omitempty"`
}

func writeQueryJSON(stdout io.Writer, doc *spec.Document, input specInput, keyword string, results []query.Endpoint, allResults []query.Endpoint, page queryPage, verbosity int) error {
	servers := make([]string, 0, len(doc.Servers))
	for _, server := range doc.Servers {
		if strings.TrimSpace(server.URL) != "" {
			servers = append(servers, server.URL)
		}
	}
	tags := make([]string, 0, len(doc.Tags))
	for _, tag := range doc.Tags {
		if strings.TrimSpace(tag.Name) != "" {
			tags = append(tags, tag.Name)
		}
	}
	payload := queryJSONOutput{
		File:      input.Path,
		Keyword:   keyword,
		Verbosity: verbosity,
		Page: queryJSONPage{
			Total:   page.Total,
			Count:   len(results),
			Limit:   page.Limit,
			Offset:  page.Offset,
			HasNext: page.Next >= 0,
			HasPrev: page.Prev >= 0,
		},
		Hints:   buildQueryJSONHints(input, keyword, page),
		Results: make([]queryJSONResult, 0, len(results)),
		Servers: servers,
		Tags:    tags,
		Info: queryJSONInfo{
			Title:          doc.Info.Title,
			Version:        doc.Info.Version,
			Description:    doc.Info.Description,
			OpenAPIVersion: doc.OpenAPI,
		},
	}
	if len(results) > 0 {
		payload.Page.Start = page.Offset + 1
		payload.Page.End = page.Offset + len(results)
	}
	if page.Next >= 0 {
		payload.Page.NextOffset = page.Next
	}
	if page.Prev >= 0 {
		payload.Page.PrevOffset = page.Prev
	}
	for _, endpoint := range results {
		item := queryJSONResult{
			Method:  endpoint.Method,
			Path:    endpoint.Path,
			Summary: querySummary(endpoint),
		}
		if verbosity >= 1 {
			item.Description = endpoint.Operation.Description
			item.OperationID = endpoint.Operation.OperationID
			item.Tags = endpoint.Operation.Tags
			item.Score = endpoint.Score
		}
		if verbosity >= 2 {
			item.Parameters = endpoint.Operation.Parameters
			item.RequestBody = endpoint.Operation.RequestBody
			item.Responses = endpoint.Operation.Responses
			item.Produces = endpoint.Operation.Produces
			item.Consumes = endpoint.Operation.Consumes
		}
		payload.Results = append(payload.Results, item)
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func buildQueryJSONHints(input specInput, keyword string, page queryPage) queryJSONHints {
	hints := queryJSONHints{}
	if page.Next >= 0 {
		hints.NextPageCommand = buildQueryCommand(input, keyword, page.Limit, page.Next)
	}
	if page.Prev >= 0 {
		hints.PreviousPageCommand = buildQueryCommand(input, keyword, page.Limit, page.Prev)
	}
	if strings.TrimSpace(keyword) == "" {
		hints.SearchTip = "use -q <keyword> to narrow results, for example: -q order"
	} else {
		hints.SearchTip = fmt.Sprintf("refine further with -q, current keyword=%s", shellQuoteForGOOS(keyword, queryCallGOOS()))
	}
	return hints
}

func buildQueryCommand(input specInput, keyword string, limit, offset int) string {
	quote := func(value string) string { return shellQuoteForGOOS(value, queryCallGOOS()) }
	command := fmt.Sprintf("oapi query %s %s --limit %d --offset %d", input.Flag, quote(input.Value), limit, offset)
	if strings.TrimSpace(keyword) != "" {
		command += fmt.Sprintf(" -q %s", quote(keyword))
	}
	return command
}

func querySummary(endpoint query.Endpoint) string {
	if strings.TrimSpace(endpoint.Operation.Summary) != "" {
		return endpoint.Operation.Summary
	}
	return endpoint.Operation.Description
}

func formatRequestBodyForVerbose(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	for _, b := range body {
		if b < 0x20 && b != '\t' && b != '\n' && b != '\r' {
			return fmt.Sprintf("<binary; %d bytes>", len(body))
		}
		if b > 0x7e {
			return fmt.Sprintf("<binary; %d bytes>", len(body))
		}
	}
	return string(body)
}
