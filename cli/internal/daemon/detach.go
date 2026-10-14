package daemon

import "os/exec"

// SetDetached makes cmd outlive the oh process that starts it (its own
// session on Unix, a new process group on Windows): the driver of a fan-out
// (R6).
func SetDetached(cmd *exec.Cmd) { setDetached(cmd) }
