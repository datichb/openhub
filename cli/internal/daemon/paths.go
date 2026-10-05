// Package daemon implements ohd, the per-user oh background process. It
// outlives the oh CLI/TUI so that agentic sessions keep running when oh is
// closed (I1/I7): it hosts the LLM credential proxy and supervises the tool
// servers registered by oh clients. Clients talk to it over a Unix socket.
package daemon

import (
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

// Log is the daemon log file.
func (p Paths) Log() string { return filepath.Join(p.Dir, "ohd.log") }

// SpawnLock serializes daemon spawns between concurrent oh clients.
func (p Paths) SpawnLock() string { return filepath.Join(p.Dir, "spawn.lock") }
