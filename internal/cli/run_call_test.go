package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeQuickBIReportSpec(t *testing.T) string {
	t.Helper()

	specPath := filepath.Join(t.TempDir(), "openapi.json")
	specContent := `{
	"openapi": "3.0.3",
	"info": {
		"title": "Quick BI API",
		"version": "1.0.0"
	},
	"paths": {
		"/Manager/Report/quickBiReportData": {
			"post": {
				"requestBody": {
					"required": true,
					"content": {
						"application/x-www-form-urlencoded": {
							"schema": {
								"type": "object",
								"properties": {
									"source": {"type": "string"},
									"report": {"type": "string"},
									"dhb_skey": {"type": "string"},
									"data_source": {"type": "integer"},
									"pageSize": {"type": "integer"},
									"searchFields": {"type": "string"},
									"searchCondition": {"type": "string"}
								},
								"required": ["source", "report"]
							}
						}
					}
				},
				"responses": {
					"200": {
						"description": "OK"
					}
				}
			}
		}
	}
}`
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("failed to write temp quick bi spec: %v", err)
	}
	return specPath
}

func TestRun_CallResolvesSpecNameFromConfiguredDirectory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users" {
			t.Fatalf("expected request to /users, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	specsDir := t.TempDir()
	specPath := filepath.Join(specsDir, "skill.openapi.yaml")
	specContent := `openapi: 3.0.3
info:
  title: Named API
  version: 1.0.0
paths:
  /users:
    get:
      responses:
        "200":
          description: OK
`
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("write named spec: %v", err)
	}
	t.Setenv("OAPI_SPECS_DIR", specsDir)

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := Run([]string{
		"call",
		"-n", "skill",
		"-e", "GET /users",
		"--base-url", server.URL,
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v; stderr=%s", err, errOut.String())
	}
	if !strings.Contains(out.String(), `"ok": true`) {
		t.Fatalf("expected successful response, got: %s", out.String())
	}
}

func TestRun_Call_UsesCurlStyleFormRequestWithoutUnknownParameterWarning(t *testing.T) {
	var gotQuery string
	var gotBody string
	var gotContentType string
	specPath := writeQuickBIReportSpec(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		gotContentType = r.Header.Get("Content-Type")

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		gotBody = string(body)

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"ok"}`))
	}))
	defer server.Close()

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", specPath,
		"-e", "POST /Manager/Report/quickBiReportData",
		"--base-url", server.URL,
		"--params", `{"source":"ai_analysis","report":"clientDev","dhb_skey":"token","data_source":2,"pageSize":1000,"searchFields":"sum_date,new_clients_total","searchCondition":"{\"start_date\":\"2026-02-09 00:00:00\",\"count\":\"1\"}"}`,
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v; stderr=%s", err, errOut.String())
	}

	if strings.Contains(errOut.String(), "unknown parameter: dhb_skey") {
		t.Fatalf("did not expect unknown parameter warning, got stderr=%s", errOut.String())
	}

	if gotQuery != "" {
		t.Fatalf("expected no query string for curl-style form request, got %q", gotQuery)
	}

	if !strings.Contains(gotContentType, "application/x-www-form-urlencoded") {
		t.Fatalf("expected form content type, got %q", gotContentType)
	}

	if !strings.Contains(gotBody, "searchCondition=%7B%22start_date%22%3A%222026-02-09+00%3A00%3A00%22%2C%22count%22%3A%221%22%7D") {
		t.Fatalf("expected encoded searchCondition in form body, got %q", gotBody)
	}

	if !strings.Contains(gotBody, "dhb_skey=token") {
		t.Fatalf("expected dhb_skey in form body, got %q", gotBody)
	}

	output := out.String()
	if strings.Contains(output, "Calling:") {
		t.Fatalf("did not expect request preamble at default verbosity, got: %s", output)
	}
	if strings.Contains(output, "Headers:") {
		t.Fatalf("did not expect headers at default verbosity, got: %s", output)
	}
	if !strings.Contains(output, `"code": 200`) {
		t.Fatalf("expected response body in output, got: %s", output)
	}
}

func TestRun_Call_AcceptsVerboseFlag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"ok"}`))
	}))
	defer server.Close()

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-v",
		"-f", "../../testdata/openapi.sample.json",
		"-e", "GET /users",
		"--base-url", server.URL,
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error with -v: %v; stderr=%s", err, errOut.String())
	}

	output := out.String()
	if !strings.Contains(output, "Calling: GET /users") {
		t.Fatalf("expected request preamble in output, got: %s", output)
	}
	if !strings.Contains(output, "Status: 200 OK") {
		t.Fatalf("expected response status in output, got: %s", output)
	}
	if strings.Contains(output, "Headers:") {
		t.Fatalf("did not expect headers at -v, got: %s", output)
	}
}

func TestRun_Call_VerboseLevel2ShowsHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Test", "ok")
		_, _ = w.Write([]byte(`{"code":200,"message":"ok"}`))
	}))
	defer server.Close()

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-vv",
		"-f", "../../testdata/openapi.sample.json",
		"-e", "GET /users",
		"--base-url", server.URL,
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error with -vv: %v; stderr=%s", err, errOut.String())
	}

	output := out.String()
	if !strings.Contains(output, "Headers:") {
		t.Fatalf("expected headers at -vv, got: %s", output)
	}
	if !strings.Contains(output, "X-Test: ok") {
		t.Fatalf("expected response headers at -vv, got: %s", output)
	}
}

func TestRun_Call_DefaultsBaseURLToFirstServerInSpec(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users" {
			t.Fatalf("expected request to /users, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"ok"}`))
	}))
	defer server.Close()

	specContent := `{
  "openapi": "3.0.3",
  "info": {
    "title": "Sample API",
    "version": "1.0.0"
  },
  "servers": [
    {
      "url": "` + server.URL + `"
    }
  ],
  "paths": {
    "/users": {
      "get": {
        "summary": "List users",
        "responses": {
          "200": {
            "description": "OK"
          }
        }
      }
    }
  }
}`

	specPath := filepath.Join(t.TempDir(), "openapi.json")
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("failed to write temp spec: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-v",
		"-f", specPath,
		"-e", "GET /users",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error without --base-url: %v; stderr=%s", err, errOut.String())
	}

	output := out.String()
	if !strings.Contains(output, "Base URL: "+server.URL) {
		t.Fatalf("expected output to show spec server as base URL, got: %s", output)
	}
	if !strings.Contains(output, "Status: 200 OK") {
		t.Fatalf("expected successful call using spec base URL, got: %s", output)
	}
}

func TestRun_Call_MatchesConcretePathAgainstTemplatedSpecPath(t *testing.T) {
	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"ok"}`))
	}))
	defer server.Close()

	specContent := `{
  "openapi": "3.0.3",
  "info": {
    "title": "Sample API",
    "version": "1.0.0"
  },
  "paths": {
    "/orders/{id}": {
      "get": {
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {
              "type": "integer"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "OK"
          }
        }
      }
    }
  }
}`

	specPath := filepath.Join(t.TempDir(), "openapi.json")
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("failed to write temp spec: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", specPath,
		"-e", "GET /orders/7080644",
		"--base-url", server.URL,
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v; stderr=%s", err, errOut.String())
	}

	if gotPath != "/orders/7080644" {
		t.Fatalf("expected request path to preserve concrete segment, got %q", gotPath)
	}
	if strings.Contains(errOut.String(), "required parameter is missing") {
		t.Fatalf("did not expect missing path parameter validation, got stderr=%s", errOut.String())
	}
	if !strings.Contains(out.String(), `"code": 200`) {
		t.Fatalf("expected response body in output, got: %s", out.String())
	}
}

func TestRun_Call_RequiresExplicitBaseURLWhenSpecHasNoServers(t *testing.T) {
	specContent := `{
  "openapi": "3.0.3",
  "info": {
    "title": "Sample API",
    "version": "1.0.0"
  },
  "paths": {
    "/users": {
      "get": {
        "summary": "List users",
        "responses": {
          "200": {
            "description": "OK"
          }
        }
      }
    }
  }
}`

	specPath := filepath.Join(t.TempDir(), "openapi.json")
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("failed to write temp spec: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", specPath,
		"-e", "GET /users",
	}, &out, &errOut)
	if err == nil {
		t.Fatalf("expected error when neither --base-url nor spec servers are available")
	}

	if !strings.Contains(err.Error(), "base-url") {
		t.Fatalf("expected base-url guidance in error, got: %v", err)
	}
}

func TestRun_Call_SendsCookieFromFlag(t *testing.T) {
	var gotCookie string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	specPath := filepath.Join(t.TempDir(), "openapi.json")
	specContent := `{
  "openapi": "3.0.3",
  "info": {"title": "T", "version": "1"},
  "paths": {
    "/ping": {
      "get": {
        "responses": {"200": {"description": "OK"}}
      }
    }
  }
}`
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", specPath,
		"-e", "GET /ping",
		"--base-url", server.URL,
		"--cookie", "a=1; b=two",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run: %v; stderr=%s", err, errOut.String())
	}

	if gotCookie != "a=1; b=two" {
		t.Fatalf("expected Cookie header on server, got %q", gotCookie)
	}
}

func TestRun_Call_SendsCookieFromPath(t *testing.T) {
	var gotCookie string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	specPath := filepath.Join(t.TempDir(), "openapi.json")
	if err := os.WriteFile(specPath, []byte(`{
  "openapi": "3.0.3",
  "info": {"title": "T", "version": "1"},
  "paths": {"/ping": {"get": {"responses": {"200": {"description": "OK"}}}}}
}`), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	cookiePath := filepath.Join(t.TempDir(), "cookie.txt")
	if err := os.WriteFile(cookiePath, []byte("# Netscape HTTP Cookie File\nadmin.dhb168.com\tFALSE\t/\tFALSE\t1893456000\tx\tfromfile\n"), 0o644); err != nil {
		t.Fatalf("write cookie file: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", specPath,
		"-e", "GET /ping",
		"--base-url", server.URL,
		"--cookie-path", cookiePath,
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run: %v; stderr=%s", err, errOut.String())
	}

	if gotCookie != "x=fromfile" {
		t.Fatalf("expected cookie parsed from jar file, got %q", gotCookie)
	}
}

func TestRun_Call_CookiePathRejectsRawCookieHeaderText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	specPath := filepath.Join(t.TempDir(), "openapi.json")
	if err := os.WriteFile(specPath, []byte(`{
  "openapi": "3.0.3",
  "info": {"title": "T", "version": "1"},
  "paths": {"/ping": {"get": {"responses": {"200": {"description": "OK"}}}}}
}`), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	cookiePath := filepath.Join(t.TempDir(), "cookie.txt")
	if err := os.WriteFile(cookiePath, []byte("x=fromfile; y=two\n"), 0o644); err != nil {
		t.Fatalf("write cookie file: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", specPath,
		"-e", "GET /ping",
		"--base-url", server.URL,
		"--cookie-path", cookiePath,
	}, &out, &errOut)
	if err == nil {
		t.Fatalf("expected error for raw cookie header text passed to --cookie-path")
	}
	if !strings.Contains(err.Error(), "Netscape cookie jar") {
		t.Fatalf("expected Netscape cookie jar guidance, got: %v", err)
	}
}

func TestRun_Call_CookieAndCookiePathMutuallyExclusive(t *testing.T) {
	specPath := filepath.Join(t.TempDir(), "openapi.json")
	if err := os.WriteFile(specPath, []byte(`{
  "openapi": "3.0.3",
  "info": {"title": "T", "version": "1"},
  "paths": {"/ping": {"get": {"responses": {"200": {"description": "OK"}}}}}
}`), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", specPath,
		"-e", "GET /ping",
		"--base-url", "http://127.0.0.1:9",
		"--cookie", "a=1",
		"--cookie-path", filepath.Join(t.TempDir(), "c.txt"),
	}, &out, &errOut)
	if err == nil {
		t.Fatalf("expected error when both --cookie and --cookie-path are set")
	}
	if !strings.Contains(err.Error(), "only one") {
		t.Fatalf("expected mutual exclusion error, got: %v", err)
	}
}

func TestRun_Call_AcceptsParamsURL(t *testing.T) {
	var gotBody string
	var gotContentType string
	specPath := writeQuickBIReportSpec(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", specPath,
		"-e", "POST /Manager/Report/quickBiReportData",
		"--base-url", server.URL,
		"--params-url", "source=ai_analysis&report=clientDev&dhb_skey=token&data_source=2&pageSize=1000",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v; stderr=%s", err, errOut.String())
	}

	if !strings.Contains(gotContentType, "application/x-www-form-urlencoded") {
		t.Fatalf("expected form content type, got %q", gotContentType)
	}
	for _, expected := range []string{
		"source=ai_analysis",
		"report=clientDev",
		"dhb_skey=token",
		"data_source=2",
		"pageSize=1000",
	} {
		if !strings.Contains(gotBody, expected) {
			t.Fatalf("expected %q in form body, got %q", expected, gotBody)
		}
	}
}

func TestRun_Call_ParamsURLRepeatsBracketArrayQueryParams(t *testing.T) {
	specPath := filepath.Join(t.TempDir(), "openapi.json")
	specContent := `{
  "openapi": "3.0.3",
  "info": {"title": "T", "version": "1"},
  "servers": [{"url": "https://example.com"}],
  "paths": {
    "/Api/v1/search/conditions": {
      "get": {
        "parameters": [
          {
            "name": "order[]",
            "in": "query",
            "schema": {
              "type": "array",
              "items": {"type": "string"}
            },
            "style": "form",
            "explode": true
          },
          {
            "name": "client[]",
            "in": "query",
            "schema": {
              "type": "array",
              "items": {"type": "string"}
            },
            "style": "form",
            "explode": true
          }
        ],
        "responses": {
          "200": {"description": "OK"}
        }
      }
    }
  }
}`
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("failed to write temp spec: %v", err)
	}

	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", specPath,
		"-e", "GET /Api/v1/search/conditions",
		"--base-url", server.URL,
		"--params-url", "order%5B%5D=status&order%5B%5D=admin_order&client%5B%5D=type",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v; stderr=%s", err, errOut.String())
	}

	if strings.Contains(errOut.String(), "expected array") {
		t.Fatalf("did not expect array validation warnings, got stderr=%s", errOut.String())
	}

	for _, expected := range []string{
		"order%5B%5D=status",
		"order%5B%5D=admin_order",
		"client%5B%5D=type",
	} {
		if !strings.Contains(gotQuery, expected) {
			t.Fatalf("expected %q in query string, got %q", expected, gotQuery)
		}
	}
}

func TestRun_Call_LoadsYAMLSpec(t *testing.T) {
	var gotMethod string
	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	specPath := filepath.Join(t.TempDir(), "openapi.yaml")
	specContent := "openapi: 3.0.3\ninfo:\n  title: Sample API\n  version: 1.0.0\npaths:\n  /users:\n    get:\n      responses:\n        '200':\n          description: OK\n"
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("failed to write temp spec: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", specPath,
		"-e", "GET /users",
		"--base-url", server.URL,
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error for YAML spec: %v; stderr=%s", err, errOut.String())
	}

	if gotMethod != http.MethodGet {
		t.Fatalf("expected GET method, got %q", gotMethod)
	}
	if gotPath != "/users" {
		t.Fatalf("expected /users path, got %q", gotPath)
	}
	if !strings.Contains(out.String(), `"ok": true`) {
		t.Fatalf("expected response body in output, got: %s", out.String())
	}
}

func TestRun_Call_InjectsPathParameterDefinedAtPathLevel(t *testing.T) {
	var gotPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	specPath := filepath.Join(t.TempDir(), "openapi.json")
	specContent := `{
  "openapi": "3.0.3",
  "info": {"title": "T", "version": "1"},
  "paths": {
    "/workflow-runs/{runZid}": {
      "parameters": [
        {
          "name": "runZid",
          "in": "path",
          "required": true,
          "schema": {"type": "string"}
        }
      ],
      "get": {
        "responses": {
          "200": {"description": "OK"}
        }
      }
    }
  }
}`
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("failed to write temp spec: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", specPath,
		"-e", "GET /workflow-runs/{runZid}",
		"--base-url", server.URL,
		"--params", `{"runZid":"QVLR8V8DMVRG2VY2"}`,
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v; stderr=%s", err, errOut.String())
	}

	if gotPath != "/workflow-runs/QVLR8V8DMVRG2VY2" {
		t.Fatalf("expected substituted path, got %q", gotPath)
	}
	if strings.Contains(errOut.String(), "unknown parameter: runZid") {
		t.Fatalf("did not expect unknown parameter warning, got stderr=%s", errOut.String())
	}
}

func TestRun_Call_InjectsQueryParametersDefinedAtPathLevel(t *testing.T) {
	var gotQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	specPath := filepath.Join(t.TempDir(), "openapi.json")
	specContent := `{
  "openapi": "3.0.3",
  "info": {"title": "T", "version": "1"},
  "paths": {
    "/workflow-runs": {
      "parameters": [
        {
          "name": "workflowDefinitionZid",
          "in": "query",
          "schema": {"type": "string"}
        },
        {
          "name": "workflowVersionZid",
          "in": "query",
          "schema": {"type": "string"}
        },
        {
          "name": "page",
          "in": "query",
          "schema": {"type": "integer"}
        },
        {
          "name": "pageSize",
          "in": "query",
          "schema": {"type": "integer"}
        }
      ],
      "get": {
        "responses": {
          "200": {"description": "OK"}
        }
      }
    }
  }
}`
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("failed to write temp spec: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", specPath,
		"-e", "GET /workflow-runs",
		"--base-url", server.URL,
		"--params", `{"workflowDefinitionZid":"Z1Z645INZMQLILJ8","workflowVersionZid":"S2CHPKA1PLMLWV33","page":1,"pageSize":10}`,
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v; stderr=%s", err, errOut.String())
	}

	for _, expected := range []string{
		"workflowDefinitionZid=Z1Z645INZMQLILJ8",
		"workflowVersionZid=S2CHPKA1PLMLWV33",
		"page=1",
		"pageSize=10",
	} {
		if !strings.Contains(gotQuery, expected) {
			t.Fatalf("expected %q in query string, got %q", expected, gotQuery)
		}
	}
	if strings.Contains(errOut.String(), "unknown parameter") {
		t.Fatalf("did not expect unknown parameter warning, got stderr=%s", errOut.String())
	}
}

func TestRun_Call_SendsBearerTokenFromFlag(t *testing.T) {
	var gotAuthorization string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	specPath := filepath.Join(t.TempDir(), "openapi.json")
	specContent := `{
  "openapi": "3.0.3",
  "info": {"title": "T", "version": "1"},
  "paths": {
    "/protected": {
      "get": {
        "responses": {
          "200": {"description": "OK"}
        }
      }
    }
  }
}`
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("failed to write temp spec: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", specPath,
		"-e", "GET /protected",
		"--base-url", server.URL,
		"--bearer-token", "secret-token",
		"-vv",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v; stderr=%s", err, errOut.String())
	}

	if gotAuthorization != "Bearer secret-token" {
		t.Fatalf("expected Authorization header, got %q", gotAuthorization)
	}
	if strings.Contains(out.String(), "secret-token") {
		t.Fatalf("did not expect bearer token to be printed in verbose output, got: %s", out.String())
	}
}

func TestRun_Call_ExcludesPathParamsFromJSONBody(t *testing.T) {
	var gotPath string
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Fatalf("failed to parse request body as JSON: %v; body=%q", err, string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	specPath := filepath.Join(t.TempDir(), "openapi.json")
	specContent := `{
  "openapi": "3.0.3",
  "info": {"title": "T", "version": "1"},
  "paths": {
    "/app-runtime/apps/{appID}/api/uploads": {
      "post": {
        "parameters": [
          {
            "name": "appID",
            "in": "path",
            "required": true,
            "schema": {"type": "string"}
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "title": {"type": "string"}
                },
                "required": ["title"]
              }
            }
          }
        },
        "responses": {
          "200": {"description": "OK"}
        }
      }
    }
  }
}`
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("failed to write temp spec: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", specPath,
		"-e", "POST /app-runtime/apps/{appID}/api/uploads",
		"--base-url", server.URL,
		"--params", `{"appID":"file-share-demo","title":"Upload test"}`,
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v; stderr=%s", err, errOut.String())
	}

	if gotPath != "/app-runtime/apps/file-share-demo/api/uploads" {
		t.Fatalf("expected substituted path, got %q", gotPath)
	}
	if _, ok := gotBody["appID"]; ok {
		t.Fatalf("expected appID to be excluded from JSON body, got %#v", gotBody)
	}
	if gotBody["title"] != "Upload test" {
		t.Fatalf("expected title in JSON body, got %#v", gotBody)
	}
}

func TestRun_Call_ExcludesInferredPathParamsFromJSONBody(t *testing.T) {
	var gotBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Fatalf("failed to parse request body as JSON: %v; body=%q", err, string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	specPath := filepath.Join(t.TempDir(), "openapi.json")
	specContent := `{
  "openapi": "3.0.3",
  "info": {"title": "T", "version": "1"},
  "paths": {
    "/app-runtime/apps/{appID}/api/uploads": {
      "post": {
        "parameters": [
          {
            "name": "appID",
            "in": "path",
            "required": true,
            "schema": {"type": "string"}
          }
        ],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "title": {"type": "string"}
                },
                "required": ["title"]
              }
            }
          }
        },
        "responses": {
          "200": {"description": "OK"}
        }
      }
    }
  }
}`
	if err := os.WriteFile(specPath, []byte(specContent), 0o644); err != nil {
		t.Fatalf("failed to write temp spec: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", specPath,
		"-e", "POST /app-runtime/apps/file-share-demo/api/uploads",
		"--base-url", server.URL,
		"--params", `{"title":"Upload test"}`,
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v; stderr=%s", err, errOut.String())
	}

	if _, ok := gotBody["appID"]; ok {
		t.Fatalf("expected inferred appID to be excluded from JSON body, got %#v", gotBody)
	}
	if gotBody["title"] != "Upload test" {
		t.Fatalf("expected title in JSON body, got %#v", gotBody)
	}
}
