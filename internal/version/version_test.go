package version

import "testing"

func TestVersion_LdflagsInjected(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"v-prefixed semver", "v0.1.0", "0.1.0"},
		{"bare semver", "0.1.0", "0.1.0"},
		{"v-prefixed with prerelease", "v1.2.3-alpha", "1.2.3-alpha"},
		{"bare with prerelease", "1.2.3-alpha", "1.2.3-alpha"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := version
			version = tt.in
			defer func() { version = orig }()
			if got := Version(); got != tt.want {
				t.Errorf("Version() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersion_FallbackToDev(t *testing.T) {
	orig := version
	version = ""
	defer func() { version = orig }()

	// No ldflags set. Under `go test`, debug.ReadBuildInfo().Main.Version
	// is "(devel)", which Version() treats as no real version and falls
	// back to "dev".
	if got := Version(); got != "dev" {
		t.Errorf("Version() with no ldflags = %q, want %q", got, "dev")
	}
}
