package autoheaders

import (
	"net/http"
	"testing"

	"openapi/internal/spec"
)

func TestResolve_SelectsDeclaredHeaders(t *testing.T) {
	t.Parallel()

	doc := &spec.Document{
		Security: spec.SecurityRequirements{{"Primary": {}}},
		Components: spec.Components{
			SecuritySchemes: map[string]spec.SecurityScheme{
				"Primary": {Type: "apiKey", In: "header", Name: "X-Primary-Key"},
				"Bearer":  {Type: "http", Scheme: "bearer"},
			},
			Parameters: map[string]spec.Parameter{
				"Trace": {Name: "X-Trace-Id", In: "header"},
			},
		},
	}
	candidates := []Candidate{
		{Variable: "OAPI_HEADER_AUTHORIZATION", Name: "Authorization", Value: "Bearer env"},
		{Variable: "OAPI_HEADER_X_PRIMARY_KEY", Name: "X-Primary-Key", Value: "primary-env"},
		{Variable: "OAPI_HEADER_X_TRACE_ID", Name: "X-Trace-Id", Value: "trace-env"},
		{Variable: "OAPI_HEADER_X_UNDECLARED", Name: "X-Undeclared", Value: "nope"},
	}

	tests := []struct {
		name              string
		operation         spec.Operation
		explicit          http.Header
		wantNames         []string
		securityRequired  bool
		securitySatisfied bool
	}{
		{
			name:              "inherits root security",
			operation:         spec.Operation{},
			wantNames:         []string{"X-Primary-Key"},
			securityRequired:  true,
			securitySatisfied: true,
		},
		{
			name:      "explicit empty security makes operation public",
			operation: spec.Operation{Security: securityPtr()},
		},
		{
			name: "OR selects first satisfiable requirement",
			operation: spec.Operation{Security: securityPtr(
				spec.SecurityRequirement{"Missing": {}},
				spec.SecurityRequirement{"Bearer": {}},
			)},
			wantNames:         []string{"Authorization"},
			securityRequired:  true,
			securitySatisfied: true,
		},
		{
			name: "incomplete AND does not send partial credentials",
			operation: spec.Operation{Security: securityPtr(
				spec.SecurityRequirement{"Primary": {}, "Missing": {}},
			)},
			securityRequired: true,
		},
		{
			name: "explicit header satisfies AND and overrides environment",
			operation: spec.Operation{Security: securityPtr(
				spec.SecurityRequirement{"Primary": {}, "Bearer": {}},
			)},
			explicit:          http.Header{"Authorization": {"Bearer explicit"}},
			wantNames:         []string{"X-Primary-Key"},
			securityRequired:  true,
			securitySatisfied: true,
		},
		{
			name: "resolved header parameter is allowed",
			operation: spec.Operation{
				Security:   securityPtr(),
				Parameters: []spec.Parameter{{Ref: "#/components/parameters/Trace"}},
			},
			wantNames: []string{"X-Trace-Id"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Resolve(doc, tt.operation, candidates, tt.explicit)
			if got.SecurityRequired != tt.securityRequired || got.SecuritySatisfied != tt.securitySatisfied {
				t.Fatalf("security state = required:%t satisfied:%t, want required:%t satisfied:%t", got.SecurityRequired, got.SecuritySatisfied, tt.securityRequired, tt.securitySatisfied)
			}
			if len(got.Headers) != len(tt.wantNames) {
				t.Fatalf("headers = %+v, want names %v", got.Headers, tt.wantNames)
			}
			for i, wantName := range tt.wantNames {
				if got.Headers[i].Name != wantName || !got.Headers[i].Secret || got.Headers[i].Source != EnvironmentSource {
					t.Fatalf("header %d = %+v, want secret environment header %s", i, got.Headers[i], wantName)
				}
			}
		})
	}
}

func securityPtr(requirements ...spec.SecurityRequirement) *spec.SecurityRequirements {
	value := spec.SecurityRequirements(requirements)
	return &value
}

func TestResolve_UsesAuthorizationForAuthorizationSecuritySchemes(t *testing.T) {
	t.Parallel()

	for _, schemeType := range []string{"http", "oauth2", "openIdConnect", "basic"} {
		schemeType := schemeType
		t.Run(schemeType, func(t *testing.T) {
			t.Parallel()

			doc := &spec.Document{
				Security: spec.SecurityRequirements{{"Auth": {}}},
				Components: spec.Components{SecuritySchemes: map[string]spec.SecurityScheme{
					"Auth": {Type: schemeType},
				}},
			}
			got := Resolve(doc, spec.Operation{}, []Candidate{{
				Variable: "OAPI_HEADER_AUTHORIZATION", Name: "Authorization", Value: "credential",
			}}, nil)
			if !got.SecuritySatisfied || len(got.Headers) != 1 || got.Headers[0].Name != "Authorization" {
				t.Fatalf("resolution = %+v, want Authorization", got)
			}
		})
	}
}
