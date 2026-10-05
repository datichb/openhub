// Package ohruntime defines where a tool server runs (03 §2.3): on the
// machine (local), in a container (Colima/Podman/Docker) or remotely.
//
// A runtime prepares the execution environment of a server group (image,
// mounts, network) and wraps the server command; the tool adapter keeps
// control of the server lifecycle (readiness, attestation, stop).
package ohruntime

import (
	"context"
	"os/exec"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Availability reports whether a runtime can be used now.
type Availability struct {
	OK      bool
	Engine  string   // e.g. colima | podman | docker
	Version string   // engine server version
	Reason  string   // i18n key explaining the state
	Args    []any    // arguments of Reason
	Details []string // extra facts (rootless, mount type…)
}

// Message is the localized explanation of the availability.
func (a Availability) Message() string {
	if a.Reason == "" {
		return ""
	}
	return i18n.Tf(a.Reason, a.Args...)
}

// Tool provides the tool binary installed in container images. It is
// implemented by the tool adapter (download source, version pinning).
type Tool interface {
	Name() string    // command name inside the image, e.g. "opencode"
	Version() string // pinned version (= adapter version)
	// LinuxBinary returns a local path to the Linux binary for arch
	// (amd64|arm64) and libc (glibc|musl).
	LinuxBinary(ctx context.Context, arch, libc string) (string, error)
}

// Group describes the server group to prepare.
type Group struct {
	Key        sessionspec.GroupKey
	ProjectID  string
	ProjectDir string   // project base directory (holds the dev Dockerfile)
	Locations  []string // session working directories to expose
	BundleDir  string   // immutable bundle (read-only)
	DataDir    string   // tool data directory of the group (read-write)
	Tool       Tool

	// Dev environment of the project (container): Dockerfile path (absolute
	// or relative to ProjectDir, "" = detected) and build arguments.
	Dockerfile string
	BuildArgs  map[string]string
	// Progress receives preparation output (image build), line by line.
	Progress func(line string)
}

// Prepared is a group ready to run commands.
type Prepared struct {
	Group Group
	// Paths maps host paths to the paths seen by the server process.
	Paths PathMap
	// Spec is runtime-specific state (e.g. the container spec).
	Spec any
}

// Proc is a command to run in a prepared group. Dir and the paths in Argv and
// Env are already expressed in the runtime's view (see Prepared.Paths).
type Proc struct {
	Argv []string
	Env  map[string]string
	Dir  string
	// Ports to publish on the machine loopback (container port = host port).
	Ports []int
}

// Runtime is implemented once per execution environment.
type Runtime interface {
	Kind() sessionspec.RuntimeKind
	Available(ctx context.Context) (Availability, error)
	Prepare(ctx context.Context, g Group) (*Prepared, error)
	// Command returns the (not yet started) machine command that runs p.
	Command(ctx context.Context, pg *Prepared, p Proc) (*exec.Cmd, error)
	// HostAddress is the machine address seen from inside (proxy, gateways).
	HostAddress() string
	Teardown(ctx context.Context, pg *Prepared) error
}
