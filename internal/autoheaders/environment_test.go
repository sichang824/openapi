package autoheaders

import (
	"strings"
	"testing"
)

func TestScanEnvironment_NormalizesCandidates(t *testing.T) {
	t.Parallel()

	candidates, err := ScanEnvironment([]string{
		"PATH=/usr/bin",
		"OAPI_HEADER_X_API_KEY=sk_test",
		"OAPI_HEADER_AUTHORIZATION=Bearer token",
		"OAPI_HEADER_X_EMPTY=",
	})
	if err != nil {
		t.Fatalf("ScanEnvironment returned error: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("got %d candidates, want 2: %+v", len(candidates), candidates)
	}
	if candidates[0].Name != "Authorization" || candidates[0].Value != "Bearer token" {
		t.Fatalf("unexpected first candidate: %+v", candidates[0])
	}
	if candidates[1].Name != "X-Api-Key" || candidates[1].Variable != "OAPI_HEADER_X_API_KEY" {
		t.Fatalf("unexpected second candidate: %+v", candidates[1])
	}
}

func TestScanEnvironment_RejectsUnsafeCandidates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		environ []string
		want    string
	}{
		{name: "empty suffix", environ: []string{"OAPI_HEADER_=value"}, want: "suffix"},
		{name: "header injection", environ: []string{"OAPI_HEADER_X_TRACE_ID=ok\r\ninjected"}, want: "CR/LF"},
		{name: "canonical collision", environ: []string{"OAPI_HEADER_X_API_KEY=one", "OAPI_HEADER_X-API-KEY=two"}, want: "same Header"},
		{name: "reserved header", environ: []string{"OAPI_HEADER_CONTENT_TYPE=text/plain"}, want: "Content-Type"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ScanEnvironment(tt.environ)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want text %q", err, tt.want)
			}
		})
	}
}
