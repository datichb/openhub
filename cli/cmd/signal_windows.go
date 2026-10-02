//go:build windows

package cmd

import "os"

// gracefulSignals returns the OS signals that should trigger graceful shutdown.
// On Windows, only os.Interrupt (Ctrl+C / CTRL_C_EVENT) is supported;
// syscall.SIGTERM does not exist.
func gracefulSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
