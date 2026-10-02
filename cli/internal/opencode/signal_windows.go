//go:build windows

package opencode

import "os"

// signalGraceful sends a termination signal to the process.
// On Windows, os.Interrupt is not reliably deliverable to non-console processes,
// so we fall back to os.Kill (TerminateProcess). The caller's fallback Kill()
// path (after the grace timeout) is therefore reached immediately.
func signalGraceful(p *os.Process) error {
	return p.Signal(os.Kill)
}

// gracefulSignals returns the OS signals that should trigger graceful shutdown.
// On Windows, only os.Interrupt (Ctrl+C / CTRL_C_EVENT) is supported;
// syscall.SIGTERM does not exist.
func gracefulSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
