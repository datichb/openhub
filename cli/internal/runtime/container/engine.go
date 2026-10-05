// Package container is the container runtime (phase 4): the tool server of a
// group runs in a container built from the project dev Dockerfile, driven by
// an OCI command-line tool (docker with a Colima context, podman, docker).
// No container engine API is used.
package container

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"

	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
)

// EngineKind identifies a container engine.
type EngineKind string

const (
	EngineAuto   EngineKind = "auto"
	EngineColima EngineKind = "colima"
	EnginePodman EngineKind = "podman"
	EngineDocker EngineKind = "docker"
)

// Machine address seen from containers, per engine.
const (
	hostDocker = "host.docker.internal"
	hostPodman = "host.containers.internal"
)

// i18n keys of availability reasons.
const (
	reasonAvailable     = "cmd.runtime.container.available"
	reasonUnsupportedOS = "cmd.runtime.container.unsupported_os"
	reasonNoEngine      = "cmd.runtime.container.no_engine"
	reasonUnknownEngine = "cmd.runtime.container.unknown_engine"
	reasonNotInstalled  = "cmd.runtime.container.not_installed"
	reasonCLIMissing    = "cmd.runtime.container.cli_missing"
	reasonVMStopped     = "cmd.runtime.container.vm_stopped"
	reasonUnreachable   = "cmd.runtime.container.unreachable"
	reasonColimaRuntime = "cmd.runtime.container.colima_runtime"
)

// Runner runs engine command-line tools (a fake one is used in tests).
type Runner interface {
	LookPath(name string) (string, error)
	// Run returns stdout; on failure the error includes stderr.
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
	// Stream runs a long command (image build) and sends each output line
	// (stdout and stderr) to line; on failure the error includes the last lines.
	Stream(ctx context.Context, line func(string), name string, args ...string) error
}

// ExecRunner runs real commands.
type ExecRunner struct{}

// Stream implements Runner.
func (ExecRunner) Stream(ctx context.Context, line func(string), name string, args ...string) error {
	pr, pw := io.Pipe()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		return err
	}
	var tail []string
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			l := sc.Text()
			if len(tail) == 10 {
				tail = tail[1:]
			}
			tail = append(tail, l)
			if line != nil {
				line(l)
			}
		}
		_, _ = io.Copy(io.Discard, pr)
	}()
	err := cmd.Wait()
	_ = pw.Close()
	<-done
	if err != nil {
		return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, strings.Join(tail, "\n"))
	}
	return nil
}

// LookPath implements Runner.
func (ExecRunner) LookPath(name string) (string, error) { return exec.LookPath(name) }

// Run implements Runner.
func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		return stdout.Bytes(), fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, firstLine(msg))
	}
	return stdout.Bytes(), nil
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}

// Engine is a detected, reachable container engine.
type Engine struct {
	Kind     EngineKind
	CLI      string   // docker or podman binary
	Args     []string // global CLI arguments (e.g. --context colima)
	Host     string   // machine address seen from containers
	Version  string   // engine server version
	Rootless bool     // podman rootless (keep-id applies)
	VM       bool     // the engine runs in a VM (macOS)
	// Shared are the machine directories shared with the VM (bind mount
	// sources must be under one of them); empty = no VM, everything is shared.
	Shared  []string
	Details []string
}

// Command returns the CLI argv for an engine subcommand.
func (e Engine) Command(args ...string) []string {
	argv := append([]string{e.CLI}, e.Args...)
	return append(argv, args...)
}

// Detector finds a container engine.
type Detector struct {
	Runner        Runner
	GOOS          string // runtime.GOOS
	ColimaProfile string // default: "default"
	Home          string // user home directory (Colima shares it with its VM)
}

// ParseEngine validates an engine setting ("" = auto).
func ParseEngine(s string) (EngineKind, bool) {
	switch k := EngineKind(strings.ToLower(strings.TrimSpace(s))); k {
	case "", EngineAuto:
		return EngineAuto, true
	case EngineColima, EnginePodman, EngineDocker:
		return k, true
	}
	return "", false
}

// Detect returns the engine for the setting pref and its availability. With
// auto, the first available engine wins (Colima, Podman, Docker); when none
// is available, the reason of the first installed one is reported.
func (d Detector) Detect(ctx context.Context, pref EngineKind) (Engine, ohruntime.Availability) {
	if d.GOOS == "windows" {
		return Engine{}, unavailable("", reasonUnsupportedOS, d.GOOS)
	}
	probes := map[EngineKind]func(context.Context) (Engine, ohruntime.Availability, bool){
		EngineColima: d.colima, EnginePodman: d.podman, EngineDocker: d.docker,
	}
	if pref == "" {
		pref = EngineAuto
	}
	if pref != EngineAuto {
		probe, ok := probes[pref]
		if !ok {
			return Engine{}, unavailable("", reasonUnknownEngine, string(pref))
		}
		e, av, _ := probe(ctx)
		return e, av
	}
	var first *ohruntime.Availability
	for _, k := range []EngineKind{EngineColima, EnginePodman, EngineDocker} {
		e, av, installed := probes[k](ctx)
		if av.OK {
			return e, av
		}
		if installed && first == nil {
			first = &av
		}
	}
	if first != nil {
		return Engine{}, *first
	}
	return Engine{}, unavailable("", reasonNoEngine)
}

func unavailable(engine, reason string, args ...any) ohruntime.Availability {
	return ohruntime.Availability{Engine: engine, Reason: reason, Args: args}
}

func available(e Engine) ohruntime.Availability {
	return ohruntime.Availability{OK: true, Engine: string(e.Kind), Version: e.Version, Reason: reasonAvailable,
		Args: []any{string(e.Kind), e.Version}, Details: e.Details}
}

type colimaStatus struct {
	Runtime   string `json:"runtime"`
	MountType string `json:"mount_type"`
	Arch      string `json:"arch"`
	Socket    string `json:"docker_socket"`
}

// colima probes Colima: the VM must run the docker runtime; the docker CLI
// is driven through the Colima context.
func (d Detector) colima(ctx context.Context) (Engine, ohruntime.Availability, bool) {
	name := string(EngineColima)
	if _, err := d.Runner.LookPath("colima"); err != nil {
		return Engine{}, unavailable(name, reasonNotInstalled, "Colima"), false
	}
	profile := d.ColimaProfile
	if profile == "" {
		profile = "default"
	}
	out, err := d.Runner.Run(ctx, "colima", "status", "--profile", profile, "--json")
	var st colimaStatus
	if err != nil || json.Unmarshal(out, &st) != nil {
		start := "colima start"
		if profile != "default" {
			start += " --profile " + profile
		}
		return Engine{}, unavailable(name, reasonVMStopped, "Colima", start), true
	}
	if st.Runtime != "docker" {
		return Engine{}, unavailable(name, reasonColimaRuntime, st.Runtime), true
	}
	cli, err := d.Runner.LookPath("docker")
	if err != nil {
		return Engine{}, unavailable(name, reasonCLIMissing, "Colima", "docker"), true
	}
	ctxName := "colima"
	if profile != "default" {
		ctxName += "-" + profile
	}
	e := Engine{Kind: EngineColima, CLI: cli, Args: []string{"--context", ctxName}, Host: hostDocker, VM: true, Shared: d.colimaShared()}
	v, err := d.Runner.Run(ctx, cli, e.Command("version", "--format", "{{.Server.Version}}")[1:]...)
	if err != nil {
		return Engine{}, unavailable(name, reasonUnreachable, "Colima", err.Error()), true
	}
	e.Version = strings.TrimSpace(string(v))
	if st.MountType != "" {
		e.Details = append(e.Details, "mount="+st.MountType)
	}
	if st.Arch != "" {
		e.Details = append(e.Details, "arch="+st.Arch)
	}
	return e, available(e), true
}

// colimaShared returns the directories Colima shares by default (the home
// directory; $TMPDIR is not shared).
func (d Detector) colimaShared() []string {
	if d.Home == "" {
		return nil
	}
	return []string{d.Home}
}

// podman probes Podman (a machine on macOS, native on Linux).
func (d Detector) podman(ctx context.Context) (Engine, ohruntime.Availability, bool) {
	name := string(EnginePodman)
	cli, err := d.Runner.LookPath("podman")
	if err != nil {
		return Engine{}, unavailable(name, reasonNotInstalled, "Podman"), false
	}
	e := Engine{Kind: EnginePodman, CLI: cli, Host: hostPodman, VM: d.GOOS != "linux"}
	if e.VM {
		e.Shared = []string{"/Users", "/private", "/var/folders"} // podman machine defaults
	}
	v, err := d.Runner.Run(ctx, cli, "version", "--format", "{{.Server.Version}}")
	if err != nil || strings.TrimSpace(string(v)) == "" {
		if e.VM {
			if st, serr := d.Runner.Run(ctx, cli, "machine", "inspect", "--format", "{{.State}}"); serr == nil && strings.TrimSpace(string(st)) != "running" {
				return Engine{}, unavailable(name, reasonVMStopped, "Podman", "podman machine start"), true
			}
		}
		msg := "no server version"
		if err != nil {
			msg = err.Error()
		}
		return Engine{}, unavailable(name, reasonUnreachable, "Podman", msg), true
	}
	e.Version = strings.TrimSpace(string(v))
	if r, err := d.Runner.Run(ctx, cli, "info", "--format", "{{.Host.Security.Rootless}}"); err == nil {
		e.Rootless = strings.TrimSpace(string(r)) == "true"
		e.Details = append(e.Details, "rootless="+strings.TrimSpace(string(r)))
	}
	return e, available(e), true
}

// docker probes the Docker CLI with its current context (Docker Desktop,
// native Linux engine, or any context the user selected).
func (d Detector) docker(ctx context.Context) (Engine, ohruntime.Availability, bool) {
	name := string(EngineDocker)
	cli, err := d.Runner.LookPath("docker")
	if err != nil {
		return Engine{}, unavailable(name, reasonNotInstalled, "Docker"), false
	}
	e := Engine{Kind: EngineDocker, CLI: cli, Host: hostDocker, VM: d.GOOS != "linux"}
	if e.VM {
		e.Shared = []string{"/Users", "/Volumes", "/private", "/tmp", "/var/folders"} // Docker Desktop defaults
	}
	v, err := d.Runner.Run(ctx, cli, "version", "--format", "{{.Server.Version}}")
	if err != nil {
		return Engine{}, unavailable(name, reasonUnreachable, "Docker", err.Error()), true
	}
	e.Version = strings.TrimSpace(string(v))
	if c, err := d.Runner.Run(ctx, cli, "context", "show"); err == nil {
		e.Details = append(e.Details, "context="+strings.TrimSpace(string(c)))
	}
	return e, available(e), true
}
