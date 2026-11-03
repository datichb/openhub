package cmd

import (
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/gitutil"
)

// The fingerprints of the git configuration of the session locations
// (v5 critical N5) are kept under ~/.oh/run/git-trust.
func init() {
	gitutil.SetTrustDirFunc(func() string { return filepath.Join(ohRunDir(), "git-trust") })
}
