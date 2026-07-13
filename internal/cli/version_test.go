package cli

import (
	"bytes"
	"runtime/debug"
	"strings"
	"testing"
)

func TestRun_Version(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}} {
		var out bytes.Buffer
		var errOut bytes.Buffer

		if err := Run(args, &out, &errOut); err != nil {
			t.Fatalf("Run(%v) returned error: %v", args, err)
		}
		if got, want := out.String(), "oapi "+version+"\n"; got != want {
			t.Fatalf("Run(%v) output = %q, want %q", args, got, want)
		}
	}
}

func TestResolveBuildVersion(t *testing.T) {
	tests := []struct {
		name string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{name: "tagged release", info: &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, ok: true, want: "v1.2.3"},
		{name: "dirty tagged release", info: &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3+dirty"}}, ok: true, want: "v1.2.3+dirty"},
		{name: "development build", info: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, ok: true, want: "dev"},
		{name: "empty version", info: &debug.BuildInfo{}, ok: true, want: "dev"},
		{name: "missing build info", ok: false, want: "dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveBuildVersion(tt.info, tt.ok); got != tt.want {
				t.Fatalf("resolveBuildVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRun_VersionFlagHasNoVerboseShorthand(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	if err := Run([]string{"--help"}, &out, &errOut); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	help := out.String()
	if !strings.Contains(help, "--version") {
		t.Fatalf("expected --version in help output, got: %s", help)
	}
	if strings.Contains(help, "-v, --version") {
		t.Fatalf("version must not claim the verbose shorthand -v, got: %s", help)
	}
}
