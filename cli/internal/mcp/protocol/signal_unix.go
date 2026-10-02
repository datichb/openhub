//go:build !windows

package protocol

import (
	"os"
	"syscall"
)

// gracefulSignals returns the OS signals that should trigger graceful shutdown.
// On Unix, both SIGINT (Ctrl+C) and SIGTERM (kill) are standard.
func gracefulSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM}
}
