// Package procinfo reads the parent and the command line of a process of
// the current user (macOS and Linux). The Beads gateway uses it to check
// that a git hook really runs under git (audit v5, S-06).
package procinfo

import "errors"

// ErrUnsupported: the system does not tell the parent or the command line
// of a process.
var ErrUnsupported = errors.New("procinfo: not supported on this system")

// Process is what is known of a running process.
type Process struct {
	PID, PPID int
	Args      []string // command line (argv)
}

// Get reads a process.
func Get(pid int) (Process, error) { return get(pid) }
