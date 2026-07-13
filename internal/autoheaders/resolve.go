package autoheaders

import (
	"net/http"
	"sort"
	"strings"

	"openapi/internal/spec"
)

const EnvironmentSource = "environment"

type ResolvedHeader struct {
	Name   string
	Value  string
	Source string
	Secret bool
}

type Resolution struct {
	Headers           []ResolvedHeader
	SecurityRequired  bool
	SecuritySatisfied bool
}

func Resolve(doc *spec.Document, operation spec.Operation, candidates []Candidate, explicit http.Header) Resolution {
	byName := make(map[string]Candidate, len(candidates))
	for _, candidate := range candidates {
		byName[strings.ToLower(candidate.Name)] = candidate
	}

	result := Resolution{}
	added := make(map[string]struct{})
	addCandidate := func(name string) {
		key := strings.ToLower(name)
		if _, exists := added[key]; exists || headerPresent(explicit, name) {
			return
		}
		candidate, exists := byName[key]
		if !exists {
			return
		}
		added[key] = struct{}{}
		result.Headers = append(result.Headers, ResolvedHeader{
			Name:   candidate.Name,
			Value:  candidate.Value,
			Source: EnvironmentSource,
			Secret: true,
		})
	}

	for _, parameter := range operation.Parameters {
		if resolved, ok := doc.ResolveParameter(parameter); ok {
			parameter = resolved
		}
		if strings.EqualFold(parameter.In, "header") && parameter.Name != "" {
			addCandidate(parameter.Name)
		}
	}

	requirements := doc.Security
	if operation.Security != nil {
		requirements = *operation.Security
	}
	if len(requirements) == 0 {
		sortResolvedHeaders(result.Headers)
		return result
	}

	result.SecurityRequired = true
	for _, requirement := range requirements {
		if len(requirement) == 0 {
			result.SecurityRequired = false
			result.SecuritySatisfied = true
			break
		}

		requiredHeaders, supported := requirementHeaders(doc, requirement)
		if !supported || !headersAvailable(requiredHeaders, byName, explicit) {
			continue
		}

		result.SecuritySatisfied = true
		for _, name := range requiredHeaders {
			addCandidate(name)
		}
		break
	}

	sortResolvedHeaders(result.Headers)
	return result
}

func requirementHeaders(doc *spec.Document, requirement spec.SecurityRequirement) ([]string, bool) {
	headers := make([]string, 0, len(requirement))
	seen := make(map[string]struct{})
	for schemeName := range requirement {
		scheme, ok := doc.SecurityScheme(schemeName)
		if !ok {
			return nil, false
		}

		name := ""
		switch strings.ToLower(scheme.Type) {
		case "apikey":
			if !strings.EqualFold(scheme.In, "header") || strings.TrimSpace(scheme.Name) == "" {
				return nil, false
			}
			name = scheme.Name
		case "http", "oauth2", "openidconnect", "basic":
			name = "Authorization"
		default:
			return nil, false
		}

		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		headers = append(headers, name)
	}
	sort.Strings(headers)
	return headers, true
}

func headersAvailable(names []string, candidates map[string]Candidate, explicit http.Header) bool {
	for _, name := range names {
		if headerHasValue(explicit, name) {
			continue
		}
		candidate, ok := candidates[strings.ToLower(name)]
		if !ok || candidate.Value == "" {
			return false
		}
	}
	return true
}

func headerPresent(headers http.Header, name string) bool {
	for key := range headers {
		if strings.EqualFold(key, name) {
			return true
		}
	}
	return false
}

func headerHasValue(headers http.Header, name string) bool {
	for key, values := range headers {
		if !strings.EqualFold(key, name) {
			continue
		}
		for _, value := range values {
			if value != "" {
				return true
			}
		}
	}
	return false
}

func sortResolvedHeaders(headers []ResolvedHeader) {
	sort.Slice(headers, func(i, j int) bool {
		return headers[i].Name < headers[j].Name
	})
}
