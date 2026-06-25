package validator

import (
	"net/url"
	"strings"
	"testing"

	"openapi/internal/spec"
)

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

	body, err := BuildFormBody(params, operation)
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
