package daemon

import (
	"os/exec"

	dclient "github.com/datichb/openhub/cli/internal/daemon/client"
)

// SetDetached makes cmd outlive the oh process that starts it (its own
// session on Unix, a new process group on Windows): the driver of a fan-out
// (R6).
func SetDetached(cmd *exec.Cmd) { dclient.SetDetached(cmd) }
