package autoheaders

import (
	"fmt"
	"net/textproto"
	"sort"
	"strings"
)

const headerEnvPrefix = "OAPI_HEADER_"

type Candidate struct {
	Variable string
	Name     string
	Value    string
}

var reservedHeaders = map[string]struct{}{
	"connection":        {},
	"content-length":    {},
	"content-type":      {},
	"cookie":            {},
	"host":              {},
	"proxy-connection":  {},
	"trailer":           {},
	"transfer-encoding": {},
	"upgrade":           {},
}

func ScanEnvironment(environ []string) ([]Candidate, error) {
	candidates := make([]Candidate, 0)
	seen := make(map[string]string)

	for _, entry := range environ {
		variable, value, ok := strings.Cut(entry, "=")
		if !ok || !strings.HasPrefix(variable, headerEnvPrefix) {
			continue
		}

		suffix := strings.TrimPrefix(variable, headerEnvPrefix)
		if suffix == "" {
			return nil, fmt.Errorf("%s has an empty Header suffix", variable)
		}
		if value == "" {
			continue
		}
		if strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("%s contains CR/LF and cannot be used as a Header value", variable)
		}

		name := strings.ReplaceAll(suffix, "_", "-")
		if !validHeaderName(name) {
			return nil, fmt.Errorf("%s maps to invalid Header name %q", variable, name)
		}
		name = textproto.CanonicalMIMEHeaderKey(name)
		key := strings.ToLower(name)
		if _, reserved := reservedHeaders[key]; reserved {
			return nil, fmt.Errorf("%s maps to reserved Header %s, which automatic headers cannot set", variable, name)
		}
		if previous, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf("%s and %s map to the same Header %s", previous, variable, name)
		}
		seen[key] = variable
		candidates = append(candidates, Candidate{Variable: variable, Name: name, Value: value})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Name < candidates[j].Name
	})
	return candidates, nil
}

func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || ('0' <= c && c <= '9') {
			continue
		}
		switch c {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '`', '|', '~':
			continue
		default:
			return false
		}
	}
	return true
}
