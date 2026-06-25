package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_Query_WarnsOnMalformedSwaggerButContinues(t *testing.T) {
	t.Parallel()

	specFile := filepath.Join(t.TempDir(), "malformed.json")
	content := `{
		"swagger": "2.0",
		"info": {"title": "Malformed", "version": "1.0.0"},
		"paths": {
			"/orders": {
				"get": {
					"summary": "List orders",
					"responses": {
						"200": {
							"description": "ok",
							"schema": {
								"type": "object",
								"properties": {
									"id": {"type": "string", "required": true}
								}
							}
						}
					}
				}
			}
		}
	}`

	if err := os.WriteFile(specFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp spec: %v", err)
	}

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := Run([]string{"query", "-f", specFile}, &out, &errOut)
	if err != nil {
		t.Fatalf("Run returned error: %v; stderr=%s", err, errOut.String())
	}

	if !strings.Contains(out.String(), "GET /orders") {
		t.Fatalf("expected endpoint output, got: %s", out.String())
	}
	stderr := errOut.String()
	if !strings.Contains(stderr, "Spec compatibility warnings") {
		t.Fatalf("expected warning header, got: %s", stderr)
	}
	if !strings.Contains(stderr, "schema-level required must be an array of property names") {
		t.Fatalf("expected required warning reason, got: %s", stderr)
	}
}
