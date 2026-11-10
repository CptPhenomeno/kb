// Package buildinfo exposes build-time metadata such as the kb version.
package buildinfo

import "runtime/debug"

// Version is the kb version. It is overridden at build time with
// -ldflags "-X github.com/CptPhenomeno/kb/internal/buildinfo.Version=<v>".
var Version = "dev"

// String returns the human-readable version line printed by `kb version`.
func String() string {
	module := ""
	if bi, ok := debug.ReadBuildInfo(); ok {
		module = bi.Main.Version
	}
	return "kb " + resolve(Version, module)
}

// resolve picks the version to show: the one injected with ldflags, else
// the module version that go install embeds (e.g. v1.0.0), else "dev".
func resolve(injected, module string) string {
	if injected != "dev" || module == "" || module == "(devel)" {
		return injected
	}
	return module
}
