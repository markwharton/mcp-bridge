// Package version provides build version information.
//
// The build version is determined by the build path:
//   - go install @latest: Go embeds the module version via debug.ReadBuildInfo()
//   - make build VERSION=x.y.z: ldflags override sets a specific version
//   - make build: ldflags sets "dev" for development builds
package version

import (
	"runtime/debug"
	"strings"
)

// version is set at build time via -ldflags for development and release builds.
// Empty means no ldflags were set (go install path).
var version string

// Version returns the build version as a bare semver string (no leading "v").
// A leading "v" is stripped so all three build paths report consistently —
// release workflow already strips it, but go install surfaces tag names verbatim.
func Version() string {
	v := "dev"
	if version != "" {
		v = version
	} else if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		v = info.Main.Version
	}
	return strings.TrimPrefix(v, "v")
}
