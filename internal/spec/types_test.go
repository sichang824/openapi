package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_SanitizesPropertyLevelRequiredBoolean(t *testing.T) {
	t.Parallel()

	specFile := filepath.Join(t.TempDir(), "malformed.json")
	content := `{
		"swagger": "2.0",
		"info": {"title": "Malformed", "version": "1.0.0"},
		"host": "example.com",
		"schemes": ["https"],
		"paths": {
			"/orders": {
				"get": {
					"summary": "List orders",
					"responses": {
						"200": {
							"description": "ok",
							"schema": {"$ref": "#/definitions/OrderList"}
						}
					}
				}
			}
		},
		"definitions": {
			"OrderList": {
				"type": "object",
				"properties": {
					"items": {
						"type": "array",
						"items": {"$ref": "#/definitions/Order"}
					}
				}
			},
			"Order": {
				"type": "object",
				"properties": {
					"id": {"type": "string", "required": true},
					"remark": {"type": "string", "required": false}
				}
			}
		}
	}`

	if err := os.WriteFile(specFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp spec: %v", err)
	}

	doc, err := Load(specFile)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if doc.OpenAPI != "2.0" {
		t.Fatalf("expected swagger version promoted to OpenAPI field, got %q", doc.OpenAPI)
	}
	if len(doc.Servers) != 1 || doc.Servers[0].URL != "https://example.com" {
		t.Fatalf("expected derived server URL, got %+v", doc.Servers)
	}
	if len(doc.Warnings) == 0 {
		t.Fatal("expected compatibility warnings, got none")
	}
	if !strings.Contains(doc.Warnings[0], "schema-level required must be an array of property names") {
		t.Fatalf("expected warning to explain required issue, got %q", doc.Warnings[0])
	}

	orderSchema, ok := doc.Definitions["Order"]
	if !ok {
		t.Fatal("expected Order definition to be loaded")
	}
	if len(orderSchema.Required) != 1 || orderSchema.Required[0] != "id" {
		t.Fatalf("expected required field to be promoted, got %+v", orderSchema.Required)
	}
	if orderSchema.Properties["id"].Required != nil {
		t.Fatalf("expected property-level required to be removed, got %+v", orderSchema.Properties["id"].Required)
	}
}

func TestLoad_NormalizesMalformedItemsString(t *testing.T) {
	t.Parallel()

	specFile := filepath.Join(t.TempDir(), "malformed-items.json")
	content := `{
		"swagger": "2.0",
		"info": {"title": "Malformed items", "version": "1.0.0"},
		"paths": {
			"/items": {
				"get": {
					"responses": {
						"200": {
							"description": "ok",
							"schema": {"$ref": "#/definitions/Payload"}
						}
					}
				}
			}
		},
		"definitions": {
			"Payload": {
				"type": "object",
				"properties": {
					"items": {
						"type": "array",
						"items": "template"
					}
				}
			}
		}
	}`

	if err := os.WriteFile(specFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp spec: %v", err)
	}

	doc, err := Load(specFile)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	payload := doc.Definitions["Payload"]
	itemsSchema := payload.Properties["items"]
	if itemsSchema.Items == nil {
		t.Fatal("expected malformed items to be normalized into an empty schema")
	}
	if itemsSchema.Items.Type != "" {
		t.Fatalf("expected fallback empty schema for unknown items string, got %+v", *itemsSchema.Items)
	}
	joinedWarnings := strings.Join(doc.Warnings, "\n")
	if !strings.Contains(joinedWarnings, "schema items must be an object") {
		t.Fatalf("expected items warning, got %s", joinedWarnings)
	}
}

func TestLoad_LoadsYAMLSpec(t *testing.T) {
	t.Parallel()

	doc, err := Load(filepath.Join("..", "..", "testdata", "openapi.sample.yaml"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if doc.Info.Title != "Sample YAML API" {
		t.Fatalf("expected title from YAML spec, got %q", doc.Info.Title)
	}
	if len(doc.Servers) != 1 || doc.Servers[0].URL != "https://api.example.com" {
		t.Fatalf("expected server URL from YAML spec, got %+v", doc.Servers)
	}
	if _, ok := doc.Paths["/ping"]; !ok {
		t.Fatalf("expected /ping path in YAML spec, got paths=%v", doc.Paths)
	}
}

func TestLoad_LoadsYAMLSpecFromTempFile(t *testing.T) {
	t.Parallel()

	specFile := filepath.Join(t.TempDir(), "openapi.yaml")
	content := `openapi: "3.0.3"
info:
  title: Temp YAML
  version: "1.0.0"
paths:
  /items:
    get:
      responses:
        "200":
          description: ok
`
	if err := os.WriteFile(specFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp spec: %v", err)
	}

	doc, err := Load(specFile)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if doc.Info.Title != "Temp YAML" {
		t.Fatalf("expected title Temp YAML, got %q", doc.Info.Title)
	}
}

func TestLoad_PreservesSecurityInheritanceAndExplicitPublicOperation(t *testing.T) {
	t.Parallel()

	specFile := filepath.Join(t.TempDir(), "security.yaml")
	content := `openapi: "3.0.3"
info:
  title: Security
  version: "1.0.0"
security:
  - ApiKeyAuth: []
components:
  securitySchemes:
    ApiKeyAuth:
      type: apiKey
      in: header
      name: X-Api-Key
paths:
  /secure:
    get:
      responses:
        "200":
          description: ok
  /public:
    get:
      security: []
      responses:
        "200":
          description: ok
`
	if err := os.WriteFile(specFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp spec: %v", err)
	}

	doc, err := Load(specFile)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(doc.Security) != 1 {
		t.Fatalf("root security = %+v, want one requirement", doc.Security)
	}
	scheme, ok := doc.Components.SecuritySchemes["ApiKeyAuth"]
	if !ok || scheme.Type != "apiKey" || scheme.In != "header" || scheme.Name != "X-Api-Key" {
		t.Fatalf("unexpected security scheme: %+v", scheme)
	}
	if got := doc.Paths["/secure"]["get"].Security; got != nil {
		t.Fatalf("inherited operation security pointer = %+v, want nil", got)
	}
	publicSecurity := doc.Paths["/public"]["get"].Security
	if publicSecurity == nil || len(*publicSecurity) != 0 {
		t.Fatalf("public operation security = %+v, want explicit empty slice", publicSecurity)
	}
}

func TestLoad_LoadsSwaggerSecurityDefinitions(t *testing.T) {
	t.Parallel()

	specFile := filepath.Join(t.TempDir(), "swagger.json")
	content := `{
  "swagger": "2.0",
  "info": {"title": "Security", "version": "1.0.0"},
  "security": [{"LegacyKey": []}],
  "securityDefinitions": {
    "LegacyKey": {"type": "apiKey", "in": "header", "name": "X-Legacy-Key"}
  },
  "paths": {}
}`
	if err := os.WriteFile(specFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp spec: %v", err)
	}

	doc, err := Load(specFile)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	scheme, ok := doc.SecurityScheme("LegacyKey")
	if !ok || scheme.Name != "X-Legacy-Key" {
		t.Fatalf("legacy security scheme = %+v, %t", scheme, ok)
	}
}

func TestLoad_NormalizesMalformedEnumObject(t *testing.T) {
	t.Parallel()

	specFile := filepath.Join(t.TempDir(), "malformed-enum.json")
	content := `{
		"swagger": "2.0",
		"info": {"title": "Malformed enum", "version": "1.0.0"},
		"paths": {},
		"definitions": {
			"Status": {
				"type": "object",
				"properties": {
					"status": {
						"type": "string",
						"enum": {
							"T": "正常",
							"F": "回收站"
						}
					}
				}
			}
		}
	}`

	if err := os.WriteFile(specFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp spec: %v", err)
	}

	doc, err := Load(specFile)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	statusSchema := doc.Definitions["Status"].Properties["status"]
	if len(statusSchema.Enum) != 2 {
		t.Fatalf("expected normalized enum keys, got %+v", statusSchema.Enum)
	}
	joinedWarnings := strings.Join(doc.Warnings, "\n")
	if !strings.Contains(joinedWarnings, "schema enum must be an array") {
		t.Fatalf("expected enum warning, got %s", joinedWarnings)
	}
}
