// Package daemon implements ohd, the per-user oh background process. It
// outlives the oh CLI/TUI so that agentic sessions keep running when oh is
// closed (I1/I7): it hosts the LLM credential proxy and supervises the tool
// servers registered by oh clients. Clients talk to it over a Unix socket.
package daemon

import (
	"errors"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/config"
)

// Paths groups the daemon runtime files (all under ~/.oh/run, mode 0700).
type Paths struct {
	Dir string
}

// DefaultPaths returns the paths under the hub directory.
func DefaultPaths() Paths { return Paths{Dir: filepath.Join(config.HubDir(), "run")} }

// Socket is the Unix socket of the daemon API.
func (p Paths) Socket() string { return filepath.Join(p.Dir, "ohd.sock") }

// Lock guarantees a single daemon per user.
func (p Paths) Lock() string { return filepath.Join(p.Dir, "ohd.lock") }

// State keeps the stable proxy port across restarts.
func (p Paths) State() string { return filepath.Join(p.Dir, "ohd.json") }

// Gateway keeps the gateway grants (token hashes only, never the tokens).
func (p Paths) Gateway() string { return filepath.Join(p.Dir, "gateway.json") }

// Log is the daemon log file.
func (p Paths) Log() string { return filepath.Join(p.Dir, "ohd.log") }

// SpawnLock serializes daemon spawns between concurrent oh clients.
func (p Paths) SpawnLock() string { return filepath.Join(p.Dir, "spawn.lock") }

// GroupLockPath is the lock file of a server group under the servers
// directory (~/.oh/servers/<group>/lock). oh clients hold it while they start,
// restart or resume the group's server; the daemon takes it before putting
// the group to sleep.
func GroupLockPath(serversDir, group string) string {
	return filepath.Join(serversDir, group, "lock")
}

// ErrUnsupported: the background daemon is not supported on Windows yet (O16).
var ErrUnsupported = errors.New("ohd is not supported on Windows")
