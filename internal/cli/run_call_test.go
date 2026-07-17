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

func TestRun_Call_WritesBinaryResponseToOutputFile(t *testing.T) {
	payload := []byte{0x50, 0x4b, 0x03, 0x04, 0x00, 0xff, 0x0a}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	outputPath := filepath.Join(t.TempDir(), "download.bin")
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", "../../testdata/openapi.sample.json",
		"-e", "GET /users",
		"--base-url", server.URL,
		"-o", outputPath,
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error with -o: %v; stderr=%s", err, errOut.String())
	}

	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("output bytes = %v, want %v", got, payload)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty when -o is used", out.String())
	}
}

func TestRun_Call_OutputWithVerboseWritesMetadataToStderr(t *testing.T) {
	payload := []byte{0x00, 0xff, 0x01, 0xfe}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	outputPath := filepath.Join(t.TempDir(), "download.bin")
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-v",
		"-f", "../../testdata/openapi.sample.json",
		"-e", "GET /users",
		"--base-url", server.URL,
		"--output", outputPath,
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error with -v -o: %v; stderr=%s", err, errOut.String())
	}

	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty when -o is used", out.String())
	}
	diagnostics := errOut.String()
	for _, want := range []string{
		"Calling: GET /users",
		"Status: 200 OK",
		"Body written to " + outputPath,
	} {
		if !strings.Contains(diagnostics, want) {
			t.Fatalf("stderr missing %q: %s", want, diagnostics)
		}
	}
	if bytes.Contains(errOut.Bytes(), payload) {
		t.Fatalf("stderr contains response body bytes: %v", errOut.Bytes())
	}
}

func TestRun_Call_InvalidOutputPathFailsBeforeRequest(t *testing.T) {
	requested := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
		_, _ = w.Write([]byte("unexpected"))
	}))
	defer server.Close()

	outputPath := filepath.Join(t.TempDir(), "missing", "download.bin")
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", "../../testdata/openapi.sample.json",
		"-e", "GET /users",
		"--base-url", server.URL,
		"-o", outputPath,
	}, &out, &errOut)
	if err == nil {
		t.Fatal("Run returned nil error for an invalid output path")
	}
	if requested {
		t.Fatal("HTTP request was sent before the output path was validated")
	}
}

func TestRun_Call_OutputDirectoryFailsBeforeRequest(t *testing.T) {
	requested := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
		_, _ = w.Write([]byte("unexpected"))
	}))
	defer server.Close()

	outputPath := t.TempDir()
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", "../../testdata/openapi.sample.json",
		"-e", "GET /users",
		"--base-url", server.URL,
		"-o", outputPath,
	}, &out, &errOut)
	if err == nil {
		t.Fatal("Run returned nil error when the output path is a directory")
	}
	if requested {
		t.Fatal("HTTP request was sent before the output path was rejected")
	}
}

func TestRun_Call_PartialDownloadDoesNotLeaveOutputFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("short"))
	}))
	defer server.Close()

	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "download.bin")
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", "../../testdata/openapi.sample.json",
		"-e", "GET /users",
		"--base-url", server.URL,
		"-o", outputPath,
	}, &out, &errOut)
	if err == nil {
		t.Fatal("Run returned nil error for a partial response body")
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("output file exists after partial download: %v", err)
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("read output directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("output directory contains temporary files after failure: %v", entries)
	}
}

func TestRun_Call_WritesHTTPErrorResponseBodyToOutputFile(t *testing.T) {
	payload := []byte(`{"error":"not found"}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	outputPath := filepath.Join(t.TempDir(), "error.json")
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{
		"call",
		"-f", "../../testdata/openapi.sample.json",
		"-e", "GET /users",
		"--base-url", server.URL,
		"-o", outputPath,
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error for HTTP 404 response: %v; stderr=%s", err, errOut.String())
	}
	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("output bytes = %q, want %q", got, payload)
	}
}

func TestRun_Call_OutputReplacesExistingFile(t *testing.T) {
	payload := []byte("new response")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	outputPath := filepath.Join(t.TempDir(), "download.txt")
	if err := os.WriteFile(outputPath, []byte("old response"), 0o600); err != nil {
		t.Fatalf("write existing output file: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := Run([]string{
		"call",
		"-f", "../../testdata/openapi.sample.json",
		"-e", "GET /users",
		"--base-url", server.URL,
		"-o", outputPath,
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error replacing output file: %v; stderr=%s", err, errOut.String())
	}

	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read replaced output file: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("output bytes = %q, want %q", got, payload)
	}
}

func TestRun_Call_RejectsEmptyOutputPathBeforeRequest(t *testing.T) {
	requested := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
		_, _ = w.Write([]byte("unexpected"))
	}))
	defer server.Close()

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := Run([]string{
		"call",
		"-f", "../../testdata/openapi.sample.json",
		"-e", "GET /users",
		"--base-url", server.URL,
		"--output", "",
	}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "--output must not be empty") {
		t.Fatalf("error = %v, want empty output path error", err)
	}
	if requested {
		t.Fatal("HTTP request was sent for an empty output path")
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

func TestRun_Call_AutoHeadersOptInAndCLIOverride(t *testing.T) {
	tests := []struct {
		name       string
		env        string
		args       []string
		wantHeader bool
	}{
		{name: "default disabled"},
		{name: "environment enables", env: "1", wantHeader: true},
		{name: "flag enables over disabled environment", env: "off", args: []string{"--auto-headers"}, wantHeader: true},
		{name: "explicit false skips invalid environment", env: "invalid", args: []string{"--auto-headers=false"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const secret = "auto-secret-value"
			var gotHeader string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotHeader = r.Header.Get("X-Api-Key")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer server.Close()

			specPath := writeAutoHeaderSpec(t, server.URL)
			t.Setenv("OAPI_AUTO_HEADERS", tt.env)
			t.Setenv("OAPI_HEADER_X_API_KEY", secret)

			args := []string{"call", "-f", specPath, "-e", "GET /protected"}
			args = append(args, tt.args...)
			var out bytes.Buffer
			var errOut bytes.Buffer
			err := Run(args, &out, &errOut)
			if err != nil {
				t.Fatalf("Run returned error: %v; stderr=%s", err, errOut.String())
			}

			if tt.wantHeader && gotHeader != secret {
				t.Fatalf("server header = %q, want injected value", gotHeader)
			}
			if !tt.wantHeader && gotHeader != "" {
				t.Fatalf("server header = %q, want no automatic header", gotHeader)
			}
			combinedOutput := out.String() + errOut.String()
			if strings.Contains(combinedOutput, secret) {
				t.Fatalf("secret leaked into command output: %s", combinedOutput)
			}
			if tt.wantHeader {
				if !strings.Contains(errOut.String(), "Auto headers enabled: X-Api-Key") || !strings.Contains(errOut.String(), "redacted") {
					t.Fatalf("expected redacted auto-header notice, stderr=%s", errOut.String())
				}
			}
		})
	}
}

func TestRun_Call_AutoHeadersEnforcesSpecOrigin(t *testing.T) {
	t.Run("rejects cross-origin base URL override", func(t *testing.T) {
		var requests int
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			w.WriteHeader(http.StatusOK)
		}))
		defer target.Close()

		specPath := writeAutoHeaderSpec(t, "https://contract.example.com")
		t.Setenv("OAPI_HEADER_X_API_KEY", "must-not-leak")
		var out bytes.Buffer
		var errOut bytes.Buffer
		err := Run([]string{
			"call", "-f", specPath, "-e", "GET /protected",
			"--base-url", target.URL, "--auto-headers",
		}, &out, &errOut)
		if err == nil || !strings.Contains(err.Error(), "origin") {
			t.Fatalf("error = %v, want origin mismatch", err)
		}
		if requests != 0 {
			t.Fatalf("target received %d requests, want none", requests)
		}
		if strings.Contains(err.Error()+errOut.String(), "must-not-leak") {
			t.Fatal("automatic header value leaked in error output")
		}
	})

	t.Run("warns when relative server origin cannot be verified", func(t *testing.T) {
		var gotHeader string
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotHeader = r.Header.Get("X-Api-Key")
			_, _ = w.Write([]byte(`{}`))
		}))
		defer target.Close()

		specPath := writeAutoHeaderSpec(t, "/api")
		t.Setenv("OAPI_HEADER_X_API_KEY", "relative-secret")
		var out bytes.Buffer
		var errOut bytes.Buffer
		err := Run([]string{
			"call", "-f", specPath, "-e", "GET /protected",
			"--base-url", target.URL, "--auto-headers",
		}, &out, &errOut)
		if err != nil {
			t.Fatalf("Run returned error: %v; stderr=%s", err, errOut.String())
		}
		if gotHeader != "relative-secret" {
			t.Fatalf("server header = %q, want automatic header", gotHeader)
		}
		if !strings.Contains(errOut.String(), "cannot verify") {
			t.Fatalf("expected unverifiable-origin warning, stderr=%s", errOut.String())
		}
	})
}

func writeAutoHeaderSpec(t *testing.T, serverURL string) string {
	t.Helper()

	specPath := filepath.Join(t.TempDir(), "openapi.yaml")
	content := `openapi: 3.0.3
info:
  title: Auto Header API
  version: 1.0.0
servers:
  - url: ` + serverURL + `
security:
  - ApiKeyAuth: []
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: X-Api-Key
paths:
  /protected:
    get:
      responses:
        "200":
          description: OK
`
	if err := os.WriteFile(specPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	return specPath
}

func TestRun_Call_AutoHeadersRespectContractAndExplicitValues(t *testing.T) {
	type received struct {
		apiKey        string
		authorization string
		traceID       string
	}
	requests := make(map[string]received)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests[r.URL.Path] = received{
			apiKey:        r.Header.Get("X-Api-Key"),
			authorization: r.Header.Get("Authorization"),
			traceID:       r.Header.Get("X-Trace-Id"),
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	specPath := filepath.Join(t.TempDir(), "openapi.yaml")
	content := `openapi: 3.0.3
info:
  title: Contract Filtering
  version: 1.0.0
servers:
  - url: ` + server.URL + `
security:
  - ApiKeyAuth: []
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: X-Api-Key
paths:
  /protected:
    get:
      parameters:
        - name: X-Trace-Id
          in: header
          schema:
            type: string
      responses:
        "200":
          description: OK
  /public:
    get:
      security: []
      responses:
        "200":
          description: OK
`
	if err := os.WriteFile(specPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	t.Setenv("OAPI_AUTO_HEADERS", "1")
	t.Setenv("OAPI_HEADER_X_API_KEY", "environment-key")
	t.Setenv("OAPI_HEADER_AUTHORIZATION", "Bearer unrelated")
	t.Setenv("OAPI_HEADER_X_TRACE_ID", "trace-from-env")

	for _, call := range [][]string{
		{"call", "-f", specPath, "-e", "GET /protected", "--header", "X-Api-Key: explicit-key"},
		{"call", "-f", specPath, "-e", "GET /public"},
	} {
		var out bytes.Buffer
		var errOut bytes.Buffer
		if err := Run(call, &out, &errOut); err != nil {
			t.Fatalf("Run(%v): %v; stderr=%s", call, err, errOut.String())
		}
	}

	if got := requests["/protected"]; got.apiKey != "explicit-key" || got.traceID != "trace-from-env" || got.authorization != "" {
		t.Fatalf("protected headers = %+v, want explicit API key, automatic trace, no unrelated authorization", got)
	}
	if got := requests["/public"]; got != (received{}) {
		t.Fatalf("public headers = %+v, want no automatic headers", got)
	}
}

func TestRun_Call_AutoHeadersStrictSecurityFailure(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	specPath := writeAutoHeaderSpec(t, server.URL)
	t.Setenv("OAPI_HEADER_X_API_KEY", "")
	var out bytes.Buffer
	var errOut bytes.Buffer
	err := Run([]string{
		"call", "-f", specPath, "-e", "GET /protected", "--auto-headers", "--strict",
	}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "security requirements") {
		t.Fatalf("error = %v, want strict security failure", err)
	}
	if requests != 0 {
		t.Fatalf("server received %d requests, want none", requests)
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

func TestParseCallHeaders_RejectsBearerWithExplicitAuthorization(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"Authorization: explicit", "Authorization:"} {
		if _, err := parseCallHeaders([]string{raw}, "bearer-token"); err == nil {
			t.Fatalf("expected %q to conflict with bearer token", raw)
		}
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
