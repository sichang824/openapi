package query

import (
	"testing"

	"openapi/internal/spec"
)

func TestSearch_MatchesMixedCasePathCaseInsensitively(t *testing.T) {
	t.Parallel()

	doc := &spec.Document{
		Paths: map[string]spec.PathItem{
			"/RestfulApi/order": {
				"get": spec.Operation{
					Summary: "订单列表",
				},
			},
		},
	}

	results := Search(doc, "RestfulApi/order", 20)
	if len(results) != 1 {
		t.Fatalf("expected one result, got %d", len(results))
	}
	if results[0].Path != "/RestfulApi/order" {
		t.Fatalf("expected /RestfulApi/order, got %s", results[0].Path)
	}
	if results[0].Method != "GET" {
		t.Fatalf("expected GET, got %s", results[0].Method)
	}

	results = Search(doc, "restfulapi/order", 20)
	if len(results) != 1 {
		t.Fatalf("expected one lowercase result, got %d", len(results))
	}
}
