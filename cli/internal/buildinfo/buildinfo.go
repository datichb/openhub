// Package buildinfo holds build-time variables injected via -ldflags.
// Placed in internal/ so it can be imported by both cmd/ and internal/ packages
// without circular dependencies.
package buildinfo

// Build-time variables injected via -ldflags.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)
