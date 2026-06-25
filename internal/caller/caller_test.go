package caller

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"openapi/internal/spec"
)

func TestCall_FormRequestDoesNotDuplicateQueryParams(t *testing.T) {
	operation := spec.Operation{
		Parameters: []spec.Parameter{
			{Name: "source", In: "query"},
			{Name: "searchCondition", In: "query"},
		},
	}

	var gotQuery string
	var gotBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		gotBody = string(body)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := Call(&CallRequest{
		BaseURL: server.URL,
		Method:  http.MethodPost,
		Path:    "/Manager/Report/quickBiReportData",
		Params: map[string]interface{}{
			"source":          "ai_analysis",
			"searchCondition": "count=1",
		},
		Operation:   operation,
		ContentType: "application/x-www-form-urlencoded",
		Body:        []byte("source=ai_analysis&searchCondition=count%3D1"),
	})
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}

	if gotQuery != "" {
		t.Fatalf("expected no query string for form request, got %q", gotQuery)
	}

	if gotBody != "source=ai_analysis&searchCondition=count%3D1" {
		t.Fatalf("expected form body to be preserved, got %q", gotBody)
	}
}

func TestCall_SendsCookieHeader(t *testing.T) {
	var gotCookie string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	_, err := Call(&CallRequest{
		BaseURL:   server.URL,
		Method:    http.MethodGet,
		Path:      "/ping",
		Params:    map[string]interface{}{},
		Operation: spec.Operation{},
		Cookie:    "session_id=abc; access_token=xyz",
	})
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}

	if gotCookie != "session_id=abc; access_token=xyz" {
		t.Fatalf("expected Cookie header, got %q", gotCookie)
	}
}

func TestCall_SendsBinaryBody(t *testing.T) {
	var gotBody []byte
	var gotContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		gotBody = body
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	payload := []byte{0x00, 0x01, 0x02, 0xff, 0xfe}

	_, err := Call(&CallRequest{
		BaseURL:     server.URL,
		Method:      http.MethodPut,
		Path:        "/upload",
		Params:      map[string]interface{}{},
		Operation:   spec.Operation{},
		ContentType: "application/octet-stream",
		Body:        payload,
	})
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}

	if gotContentType != "application/octet-stream" {
		t.Fatalf("expected octet-stream content type, got %q", gotContentType)
	}
	if !bytes.Equal(gotBody, payload) {
		t.Fatalf("expected binary body %v, got %v", payload, gotBody)
	}
}

func TestCall_RepeatsArrayQueryParams(t *testing.T) {
	operation := spec.Operation{
		Parameters: []spec.Parameter{
			{Name: "order[]", In: "query"},
			{Name: "client[]", In: "query"},
		},
	}

	var gotQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	_, err := Call(&CallRequest{
		BaseURL: server.URL,
		Method:  http.MethodGet,
		Path:    "/Api/v1/search/conditions",
		Params: map[string]interface{}{
			"order[]":  []interface{}{"status", "admin_order"},
			"client[]": []interface{}{"type", "pay_type"},
		},
		Operation: operation,
	})
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}

	for _, expected := range []string{
		"order%5B%5D=status",
		"order%5B%5D=admin_order",
		"client%5B%5D=type",
		"client%5B%5D=pay_type",
	} {
		if !strings.Contains(gotQuery, expected) {
			t.Fatalf("expected repeated query param %q in %q", expected, gotQuery)
		}
	}
}
