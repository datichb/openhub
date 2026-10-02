//go:build !windows

package opencode

import (
	"os"
	"syscall"
)

// signalGraceful sends SIGTERM to the process for graceful shutdown.
// On Unix, SIGTERM is the standard graceful termination signal.
func signalGraceful(p *os.Process) error {
	return p.Signal(syscall.SIGTERM)
}

// gracefulSignals returns the OS signals that should trigger graceful shutdown.
// On Unix, both SIGINT (Ctrl+C) and SIGTERM (kill) are standard.
func gracefulSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}
