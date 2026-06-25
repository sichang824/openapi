package caller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var sensitiveHeaderNames = map[string]struct{}{
	"authorization": {},
	"accesstoken":   {},
	"cookie":        {},
	"x-api-key":     {},
	"api-key":       {},
}

// ParseHeaderLine parses a curl-style header value: "Name: value".
func ParseHeaderLine(raw string) (string, string, error) {
	trimmed := strings.TrimSpace(raw)
	colon := strings.Index(trimmed, ":")
	if colon <= 0 {
		return "", "", errors.New("expected header format 'Name: value'")
	}

	name := strings.TrimSpace(trimmed[:colon])
	value := strings.TrimSpace(trimmed[colon+1:])
	if name == "" {
		return "", "", errors.New("header name is required")
	}
	if value == "" {
		return "", "", errors.New("header value is required")
	}

	return name, value, nil
}

// IsSensitiveHeader reports whether a header name should be redacted in logs.
func IsSensitiveHeader(name string) bool {
	_, ok := sensitiveHeaderNames[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// FormatHeaderForVerbose formats a request header for verbose CLI output.
func FormatHeaderForVerbose(name, value string) string {
	if IsSensitiveHeader(name) {
		return fmt.Sprintf("%s: <redacted; length=%d>", name, len(value))
	}
	return fmt.Sprintf("%s: %s", name, value)
}

func applyRequestHeaders(httpReq *http.Request, headers http.Header, cookie string) {
	for key, values := range headers {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	if c := strings.TrimSpace(cookie); c != "" {
		httpReq.Header.Set("Cookie", c)
	}
}
