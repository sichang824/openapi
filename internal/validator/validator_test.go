package validator

import (
	"net/url"
	"strings"
	"testing"

	"openapi/internal/spec"
)

func TestBuildRequestBodyParams_ExcludesPathAndQueryParams(t *testing.T) {
	doc := &spec.Document{}
	operation := spec.Operation{
		Parameters: []spec.Parameter{
			{Name: "appID", In: "path", Required: true, Schema: spec.Schema{Type: "string"}},
			{Name: "page", In: "query", Schema: spec.Schema{Type: "integer"}},
		},
		RequestBody: &spec.RequestBody{
			Content: map[string]spec.MediaType{
				"application/json": {
					Schema: spec.Schema{
						Type: "object",
						Properties: map[string]spec.Schema{
							"title": {Type: "string"},
						},
					},
				},
			},
		},
	}

	params := map[string]interface{}{
		"appID": "file-share-demo",
		"page":  float64(1),
		"title": "Upload test",
	}

	bodyParams := BuildRequestBodyParams(params, operation, doc)
	if _, ok := bodyParams["appID"]; ok {
		t.Fatalf("expected appID to be excluded from body params, got %#v", bodyParams)
	}
	if _, ok := bodyParams["page"]; ok {
		t.Fatalf("expected page to be excluded from body params, got %#v", bodyParams)
	}
	if bodyParams["title"] != "Upload test" {
		t.Fatalf("expected title in body params, got %#v", bodyParams)
	}
}

func TestBuildFormBody_ExcludesPathAndQueryParams(t *testing.T) {
	doc := &spec.Document{}
	operation := spec.Operation{
		Parameters: []spec.Parameter{
			{Name: "appID", In: "path", Required: true, Schema: spec.Schema{Type: "string"}},
		},
		RequestBody: &spec.RequestBody{
			Content: map[string]spec.MediaType{
				"application/x-www-form-urlencoded": {},
			},
		},
	}

	params := map[string]interface{}{
		"appID":  "file-share-demo",
		"source": "ai_analysis",
	}

	body, err := BuildFormBody(params, operation, doc)
	if err != nil {
		t.Fatalf("BuildFormBody returned error: %v", err)
	}
	if strings.Contains(body, "appID=") {
		t.Fatalf("expected appID to be excluded from form body, got %q", body)
	}
	if !strings.Contains(body, "source=ai_analysis") {
		t.Fatalf("expected source in form body, got %q", body)
	}
}

func TestBuildFormBody_EncodesValues(t *testing.T) {
	operation := spec.Operation{
		RequestBody: &spec.RequestBody{
			Content: map[string]spec.MediaType{
				"application/x-www-form-urlencoded": {},
			},
		},
	}

	params := map[string]interface{}{
		"source":          "ai_analysis",
		"searchCondition": "{\"start_date\":\"2026-02-09 00:00:00\",\"count\":\"1\"}",
	}

	body, err := BuildFormBody(params, operation, nil)
	if err != nil {
		t.Fatalf("BuildFormBody returned error: %v", err)
	}

	if body == "" {
		t.Fatal("expected non-empty form body")
	}

	if !strings.Contains(body, "searchCondition=%7B%22start_date%22%3A%222026-02-09+00%3A00%3A00%22%2C%22count%22%3A%221%22%7D") {
		t.Fatalf("expected percent-encoded searchCondition, got %q", body)
	}

	values, err := url.ParseQuery(body)
	if err != nil {
		t.Fatalf("form body should be URL-encoded, got parse error: %v; body=%q", err, body)
	}

	if got := values.Get("source"); got != "ai_analysis" {
		t.Fatalf("expected source to round-trip, got %q", got)
	}

	wantSearchCondition := "{\"start_date\":\"2026-02-09 00:00:00\",\"count\":\"1\"}"
	if got := values.Get("searchCondition"); got != wantSearchCondition {
		t.Fatalf("expected searchCondition %q, got %q", wantSearchCondition, got)
	}
}

func TestParseParams_URLQueryString(t *testing.T) {
	params, err := ParseParams("", "", "source=ai_analysis&page=1&keyword=hello%20world")
	if err != nil {
		t.Fatalf("ParseParams returned error: %v", err)
	}

	if got := params["source"]; got != "ai_analysis" {
		t.Fatalf("expected source=ai_analysis, got %v", got)
	}
	if got := params["page"]; got != "1" {
		t.Fatalf("expected page=1, got %v", got)
	}
	if got := params["keyword"]; got != "hello world" {
		t.Fatalf("expected decoded keyword, got %v", got)
	}
}

func TestParseParams_URLQueryString_PreservesRepeatedArrayParams(t *testing.T) {
	params, err := ParseParams("", "", "order%5B%5D=status&order%5B%5D=admin_order&goods%5B%5D=brand")
	if err != nil {
		t.Fatalf("ParseParams returned error: %v", err)
	}

	orderVals, ok := params["order[]"].([]interface{})
	if !ok {
		t.Fatalf("expected order[] to be parsed as []interface{}, got %T", params["order[]"])
	}
	if len(orderVals) != 2 || orderVals[0] != "status" || orderVals[1] != "admin_order" {
		t.Fatalf("unexpected order[] values: %#v", orderVals)
	}

	goodsVals, ok := params["goods[]"].([]interface{})
	if !ok {
		t.Fatalf("expected goods[] to be parsed as []interface{}, got %T", params["goods[]"])
	}
	if len(goodsVals) != 1 || goodsVals[0] != "brand" {
		t.Fatalf("unexpected goods[] values: %#v", goodsVals)
	}
}

func TestParseParams_URLQueryString_AcceptsRawBracketArrayParams(t *testing.T) {
	params, err := ParseParams("", "", "order[]=status&order[]=admin_order&client[]=type")
	if err != nil {
		t.Fatalf("ParseParams returned error: %v", err)
	}

	orderVals, ok := params["order[]"].([]interface{})
	if !ok {
		t.Fatalf("expected order[] to be parsed as []interface{}, got %T", params["order[]"])
	}
	if len(orderVals) != 2 || orderVals[0] != "status" || orderVals[1] != "admin_order" {
		t.Fatalf("unexpected order[] values: %#v", orderVals)
	}

	clientVals, ok := params["client[]"].([]interface{})
	if !ok {
		t.Fatalf("expected client[] to be parsed as []interface{}, got %T", params["client[]"])
	}
	if len(clientVals) != 1 || clientVals[0] != "type" {
		t.Fatalf("unexpected client[] values: %#v", clientVals)
	}
}

func TestParseParams_RejectsMultipleSources(t *testing.T) {
	_, err := ParseParams(`{"a":1}`, "", "a=1")
	if err == nil {
		t.Fatal("expected error when multiple param sources are set")
	}
	if !strings.Contains(err.Error(), "only one") {
		t.Fatalf("expected mutual exclusion error, got %v", err)
	}
}

func TestValidateParams_SkipsRequiredHeaderParameters(t *testing.T) {
	operation := spec.Operation{
		Parameters: []spec.Parameter{
			{Name: "Authorization", In: "header", Required: true, Schema: spec.Schema{Type: "string"}},
			{Name: "Action", In: "query", Required: true, Schema: spec.Schema{Type: "string"}},
		},
	}

	result := ValidateParams(map[string]interface{}{
		"Action": "DescribeCodingCurrentUser",
	}, operation, &spec.Document{}, false)
	if result.HasErrors() {
		t.Fatalf("expected no errors when Authorization is supplied via --header, got %#v", result.Errors)
	}
}

func TestValidateParams_AllowsNullForNullableSchema(t *testing.T) {
	operation := spec.Operation{
		Parameters: []spec.Parameter{
			{Name: "note", In: "query", Schema: spec.Schema{Type: "string", Nullable: true}},
		},
	}

	result := ValidateParams(map[string]interface{}{
		"note": nil,
	}, operation, &spec.Document{}, true)
	if result.HasErrors() {
		t.Fatalf("expected nullable query param to accept null, got %#v", result.Errors)
	}
}

func TestValidateParams_ValidatesOpenAPI31TypeUnion(t *testing.T) {
	operation := spec.Operation{
		Parameters: []spec.Parameter{
			{
				Name: "value",
				In:   "query",
				Schema: spec.Schema{
					Type:  "string",
					Types: []string{"string", "number", "null"},
				},
			},
		},
	}

	for _, value := range []interface{}{"text", float64(1), nil} {
		result := ValidateParams(map[string]interface{}{"value": value}, operation, &spec.Document{}, true)
		if result.HasErrors() {
			t.Fatalf("expected union to accept %#v, got %#v", value, result.Errors)
		}
	}

	result := ValidateParams(map[string]interface{}{"value": true}, operation, &spec.Document{}, true)
	if !result.HasErrors() {
		t.Fatal("expected union to reject boolean")
	}
}
