package autoheaders

import "testing"

func TestResolveEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		flagValue bool
		flagSet   bool
		envValue  string
		envSet    bool
		want      bool
		wantErr   bool
	}{
		{name: "default disabled"},
		{name: "environment true", envValue: "on", envSet: true, want: true},
		{name: "environment false", envValue: "OFF", envSet: true},
		{name: "invalid environment", envValue: "maybe", envSet: true, wantErr: true},
		{name: "explicit true overrides environment", flagValue: true, flagSet: true, envValue: "0", envSet: true, want: true},
		{name: "explicit false skips invalid environment", flagSet: true, envValue: "maybe", envSet: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ResolveEnabled(tt.flagValue, tt.flagSet, func(name string) (string, bool) {
				if name != AutoHeadersEnv {
					t.Fatalf("unexpected environment lookup %q", name)
				}
				return tt.envValue, tt.envSet
			})
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveEnabled returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("ResolveEnabled = %t, want %t", got, tt.want)
			}
		})
	}
}
