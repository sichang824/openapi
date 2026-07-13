package caller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"openapi/internal/spec"
)

type CallRequest struct {
	BaseURL     string
	Method      string
	Path        string
	Params      map[string]interface{}
	Operation   spec.Operation
	ContentType string
	Body        []byte
	Headers     http.Header
	AutoHeaders []ResolvedHeader
	// Cookie is the raw value for the HTTP Cookie header (e.g. "a=1; b=2").
	Cookie string
}

type ResolvedHeader struct {
	Name   string
	Value  string
	Source string
	Secret bool
}

type CallResponse struct {
	StatusCode int
	Headers    map[string][]string
	Body       string
	URL        string
}

func Call(req *CallRequest) (*CallResponse, error) {
	url := ""
	if shouldSendQuery(req) {
		url = buildURL(req.BaseURL, req.Path, req.Params, req.Operation, true)
	} else {
		url = buildURL(req.BaseURL, req.Path, req.Params, req.Operation, false)
	}
	var bodyReader io.Reader
	if len(req.Body) > 0 {
		bodyReader = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequest(req.Method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Accept", "application/json")
	for _, header := range req.AutoHeaders {
		httpReq.Header.Set(header.Name, header.Value)
	}
	for key, values := range req.Headers {
		httpReq.Header.Del(key)
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}
	if req.ContentType != "" {
		httpReq.Header.Set("Content-Type", req.ContentType)
	}
	if c := strings.TrimSpace(req.Cookie); c != "" {
		httpReq.Header.Set("Cookie", c)
	}

	client := &http.Client{CheckRedirect: rejectCrossOriginAutoHeaderRedirects(req.AutoHeaders)}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return &CallResponse{
		StatusCode: resp.StatusCode,
		Headers:    map[string][]string(resp.Header),
		Body:       string(respBody),
		URL:        url,
	}, nil
}

func rejectCrossOriginAutoHeaderRedirects(headers []ResolvedHeader) func(*http.Request, []*http.Request) error {
	if len(headers) == 0 {
		return nil
	}
	return func(next *http.Request, via []*http.Request) error {
		if len(via) == 0 || sameOrigin(via[0].URL, next.URL) {
			return nil
		}
		return fmt.Errorf(
			"blocked cross-origin redirect while automatic environment headers are active: %s -> %s",
			originString(via[0].URL),
			originString(next.URL),
		)
	}
}

func sameOrigin(left, right *url.URL) bool {
	return strings.EqualFold(left.Scheme, right.Scheme) &&
		strings.EqualFold(left.Hostname(), right.Hostname()) &&
		effectivePort(left) == effectivePort(right)
}

func effectivePort(value *url.URL) string {
	if port := value.Port(); port != "" {
		return port
	}
	switch strings.ToLower(value.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func originString(value *url.URL) string {
	if value == nil {
		return "<unknown>"
	}
	return value.Scheme + "://" + value.Host
}

func buildURL(baseURL, path string, params map[string]interface{}, operation spec.Operation, includeQuery bool) string {
	finalPath := path

	for _, p := range operation.Parameters {
		if p.In == "path" {
			if val, ok := params[p.Name]; ok {
				finalPath = strings.ReplaceAll(finalPath, "{"+p.Name+"}", url.PathEscape(fmt.Sprintf("%v", val)))
			}
		}
	}

	if !includeQuery {
		return fmt.Sprintf("%s%s", baseURL, finalPath)
	}

	queryValues := url.Values{}
	for _, p := range operation.Parameters {
		if p.In != "query" {
			continue
		}
		if val, ok := params[p.Name]; ok {
			addQueryParam(queryValues, p.Name, val)
		}
	}

	encodedQuery := queryValues.Encode()
	if encodedQuery != "" {
		return fmt.Sprintf("%s%s?%s", baseURL, finalPath, encodedQuery)
	}

	return fmt.Sprintf("%s%s", baseURL, finalPath)
}

func addQueryParam(queryValues url.Values, key string, value interface{}) {
	if value == nil {
		queryValues.Add(key, "")
		return
	}

	rv := reflect.ValueOf(value)
	if rv.IsValid() {
		switch rv.Kind() {
		case reflect.Slice, reflect.Array:
			for i := 0; i < rv.Len(); i++ {
				queryValues.Add(key, fmt.Sprintf("%v", rv.Index(i).Interface()))
			}
			return
		}
	}

	queryValues.Add(key, fmt.Sprintf("%v", value))
}

func shouldSendQuery(req *CallRequest) bool {
	return !(req.ContentType == "application/x-www-form-urlencoded" && len(req.Body) > 0)
}

func FormatResponse(resp *CallResponse, verbosity int) string {
	var buf bytes.Buffer

	if verbosity >= 1 {
		fmt.Fprintf(&buf, "Status: %d %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	if verbosity >= 2 {
		fmt.Fprintf(&buf, "Headers:\n")
		for key, values := range resp.Headers {
			for _, val := range values {
				fmt.Fprintf(&buf, "  %s: %s\n", key, val)
			}
		}
	}

	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, []byte(resp.Body), "", "  "); err == nil {
		if verbosity >= 1 {
			fmt.Fprintf(&buf, "Body:\n%s\n", prettyJSON.String())
		} else {
			fmt.Fprintf(&buf, "%s\n", prettyJSON.String())
		}
	} else {
		if verbosity >= 1 {
			fmt.Fprintf(&buf, "Body:\n%s\n", resp.Body)
		} else {
			fmt.Fprintf(&buf, "%s\n", resp.Body)
		}
	}

	return buf.String()
}
