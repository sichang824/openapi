package caller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"openapi/internal/spec"
)

func TestParseHeaderLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantName  string
		wantValue string
		wantErr   bool
	}{
		{
			name:      "authorization bearer",
			input:     "Authorization: Bearer tok_abc",
			wantName:  "Authorization",
			wantValue: "Bearer tok_abc",
		},
		{
			name:      "coding access token",
			input:     "AccessToken: tok_xyz",
			wantName:  "AccessToken",
			wantValue: "tok_xyz",
		},
		{
			name:      "value contains colon",
			input:     "X-Trace: a:b:c",
			wantName:  "X-Trace",
			wantValue: "a:b:c",
		},
		{
			name:    "missing colon",
			input:   "Authorization Bearer",
			wantErr: true,
		},
		{
			name:    "empty name",
			input:   ": value",
			wantErr: true,
		},
		{
			name:    "empty value",
			input:   "Authorization:",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotName, gotValue, err := ParseHeaderLine(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseHeaderLine returned error: %v", err)
			}
			if gotName != tt.wantName || gotValue != tt.wantValue {
				t.Fatalf("ParseHeaderLine(%q) = (%q, %q), want (%q, %q)", tt.input, gotName, gotValue, tt.wantName, tt.wantValue)
			}
		})
	}
}

func TestCall_SendsCustomHeaders(t *testing.T) {
	var gotAuth string
	var gotAction string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAction = r.Header.Get("Action")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	_, err := Call(&CallRequest{
		BaseURL: server.URL,
		Method:  http.MethodPost,
		Path:    "/open-api",
		Params:  map[string]interface{}{},
		Operation: spec.Operation{},
		Headers: http.Header{
			"Authorization": []string{"Bearer tok_abc"},
			"Action":        []string{"DescribeProjectDepots"},
		},
	})
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}

	if gotAuth != "Bearer tok_abc" {
		t.Fatalf("expected Authorization header, got %q", gotAuth)
	}
	if gotAction != "DescribeProjectDepots" {
		t.Fatalf("expected Action header, got %q", gotAction)
	}
}

func TestCall_CookieOverridesCookieHeader(t *testing.T) {
	var gotCookie string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	_, err := Call(&CallRequest{
		BaseURL: server.URL,
		Method:  http.MethodGet,
		Path:    "/ping",
		Params:  map[string]interface{}{},
		Operation: spec.Operation{},
		Headers: http.Header{
			"Cookie": []string{"from=header"},
		},
		Cookie: "from=cookie-flag",
	})
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}

	if gotCookie != "from=cookie-flag" {
		t.Fatalf("expected --cookie to override Cookie header, got %q", gotCookie)
	}
}

func TestSensitiveHeaderNames(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"Authorization", "authorization", "AccessToken", "Cookie", "X-Api-Key"} {
		if !IsSensitiveHeader(name) {
			t.Fatalf("expected %q to be sensitive", name)
		}
	}

	if IsSensitiveHeader("Content-Type") {
		t.Fatalf("did not expect Content-Type to be sensitive")
	}
}

func TestFormatHeaderForVerbose(t *testing.T) {
	t.Parallel()

	got := FormatHeaderForVerbose("Authorization", "Bearer secret")
	if strings.Contains(got, "secret") {
		t.Fatalf("expected redacted verbose header, got %q", got)
	}
	if !strings.Contains(got, "redacted") {
		t.Fatalf("expected redaction marker, got %q", got)
	}

	gotPlain := FormatHeaderForVerbose("Content-Type", "application/json")
	if gotPlain != "Content-Type: application/json" {
		t.Fatalf("expected plain header formatting, got %q", gotPlain)
	}
}
