// Package buildinfo exposes build-time metadata such as the kb version.
package buildinfo

// Version is the kb version. It is overridden at build time with
// -ldflags "-X github.com/CptPhenomeno/kb/internal/buildinfo.Version=<v>".
var Version = "dev"

// String returns the human-readable version line printed by `kb version`.
func String() string {
	return "kb " + Version
}
