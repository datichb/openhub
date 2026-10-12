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
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

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
	reasonAllFailed     = "cmd.runtime.container.all_failed"
)

// versionRetryDelay is the pause before the second `version` probe of an
// engine (a VM may be slow to answer).
var versionRetryDelay = 500 * time.Millisecond

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
type ExecRunner struct {
	// Dirs are searched when a tool is not in PATH, and appended to the PATH
	// of the commands (nil: well-known install directories).
	Dirs []string
}

func (r ExecRunner) dirs() []string {
	if r.Dirs != nil {
		return r.Dirs
	}
	return binDirs()
}

// binDirs are the usual install directories of the engine tools (a GUI or
// login shell may not have them in PATH).
func binDirs() []string {
	dirs := []string{"/opt/homebrew/bin", "/usr/local/bin", "/opt/local/bin", "/opt/podman/bin",
		"/Applications/Docker.app/Contents/Resources/bin", "/home/linuxbrew/.linuxbrew/bin"}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		dirs = append(dirs, filepath.Join(home, ".docker", "bin"), filepath.Join(home, ".rd", "bin"),
			filepath.Join(home, ".local", "bin"))
	}
	return dirs
}

// withDirs appends to a PATH value the existing dirs it lacks.
func withDirs(path string, dirs []string) string {
	have := filepath.SplitList(path)
	for _, d := range dirs {
		if slices.Contains(have, d) {
			continue
		}
		if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
			continue
		}
		have = append(have, d)
		if path == "" {
			path = d
		} else {
			path += string(os.PathListSeparator) + d
		}
	}
	return path
}

// env is the environment of the commands (nil: inherited as is).
func (r ExecRunner) env() []string {
	path := os.Getenv("PATH")
	np := withDirs(path, r.dirs())
	if np == path {
		return nil
	}
	env := slices.DeleteFunc(os.Environ(), func(kv string) bool { return strings.HasPrefix(kv, "PATH=") })
	return append(env, "PATH="+np)
}

// Stream implements Runner.
func (r ExecRunner) Stream(ctx context.Context, line func(string), name string, args ...string) error {
	pr, pw := io.Pipe()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = r.env()
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

// LookPath implements Runner: PATH first, then the install directories.
func (r ExecRunner) LookPath(name string) (string, error) {
	p, err := exec.LookPath(name)
	if err == nil || strings.ContainsRune(name, os.PathSeparator) {
		return p, err
	}
	for _, d := range r.dirs() {
		c := filepath.Join(d, name)
		if fi, serr := os.Stat(c); serr == nil && fi.Mode().IsRegular() && fi.Mode()&0o111 != 0 {
			return c, nil
		}
	}
	return "", err
}

// Run implements Runner.
func (r ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = r.env()
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
	ColimaHome    string // Colima directory (default: <Home>/.colima)
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
// is available, the reasons of every installed one are reported.
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
	var failed failures
	for _, k := range []EngineKind{EngineColima, EnginePodman, EngineDocker} {
		e, av, installed := probes[k](ctx)
		if av.OK {
			return e, av
		}
		if installed {
			failed = append(failed, av)
		}
	}
	switch len(failed) {
	case 0:
		return Engine{}, unavailable("", reasonNoEngine)
	case 1:
		return Engine{}, failed[0]
	}
	return Engine{}, unavailable("", reasonAllFailed, failed)
}

// failures are the reasons of the installed engines that do not work
// (localized when the message is formatted).
type failures []ohruntime.Availability

func (f failures) String() string {
	msgs := make([]string, len(f))
	for i, av := range f {
		msgs[i] = av.Message()
	}
	return strings.Join(msgs, "; ")
}

// serverVersion runs `version` on an engine CLI, once more after a short
// pause when it fails.
func (d Detector) serverVersion(ctx context.Context, cli string, args ...string) (string, error) {
	args = append(append([]string{}, args...), "version", "--format", "{{.Server.Version}}")
	for attempt := 0; ; attempt++ {
		out, err := d.Runner.Run(ctx, cli, args...)
		v := strings.TrimSpace(string(out))
		if err == nil && v != "" {
			return v, nil
		}
		if err == nil {
			err = errors.New("no server version")
		}
		if attempt > 0 || ctx.Err() != nil {
			return "", err
		}
		select {
		case <-ctx.Done():
			return "", err
		case <-time.After(versionRetryDelay):
		}
	}
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
// is driven through the Colima context, else through the Colima socket.
func (d Detector) colima(ctx context.Context) (Engine, ohruntime.Availability, bool) {
	name := string(EngineColima)
	profile := d.ColimaProfile
	if profile == "" {
		profile = "default"
	}
	if _, err := d.Runner.LookPath("colima"); err != nil {
		return Engine{}, unavailable(name, reasonNotInstalled, "Colima"), false
	}
	sockets := d.colimaSockets(profile)
	st, statusErr := d.colimaStatus(ctx, profile)
	if statusErr != nil && len(sockets) == 0 {
		return Engine{}, colimaDown(profile, statusErr), true
	}
	if statusErr == nil {
		if st.Runtime != "docker" {
			return Engine{}, unavailable(name, reasonColimaRuntime, st.Runtime), true
		}
		if s := strings.TrimPrefix(st.Socket, "unix://"); s != "" && !slices.Contains(sockets, s) {
			sockets = append([]string{s}, sockets...)
		}
	}
	cli, err := d.Runner.LookPath("docker")
	if err != nil {
		return Engine{}, unavailable(name, reasonCLIMissing, "Colima", "docker"), true
	}
	var tries [][]string
	if statusErr == nil {
		ctxName := "colima"
		if profile != "default" {
			ctxName += "-" + profile
		}
		tries = append(tries, []string{"--context", ctxName})
	}
	for _, s := range sockets {
		tries = append(tries, []string{"--host", "unix://" + s})
	}
	e := Engine{Kind: EngineColima, CLI: cli, Host: hostDocker, VM: true, Shared: d.colimaShared()}
	var dockerErr error
	for _, args := range tries {
		v, err := d.serverVersion(ctx, cli, args...)
		if err != nil {
			if dockerErr == nil {
				dockerErr = err
			}
			continue
		}
		e.Args, e.Version = args, v
		if st.MountType != "" {
			e.Details = append(e.Details, "mount="+st.MountType)
		}
		if st.Arch != "" {
			e.Details = append(e.Details, "arch="+st.Arch)
		}
		return e, available(e), true
	}
	if statusErr != nil {
		return Engine{}, colimaDown(profile, statusErr), true
	}
	return Engine{}, unavailable(name, reasonUnreachable, "Colima", dockerErr.Error()), true
}

// colimaDown explains a failed `colima status`: VM stopped, or a status that
// could not be read.
func colimaDown(profile string, err error) ohruntime.Availability {
	if !colimaStopped(err) {
		return unavailable(string(EngineColima), reasonUnreachable, "Colima", err.Error())
	}
	start := "colima start"
	if profile != "default" {
		start += " --profile " + profile
	}
	return unavailable(string(EngineColima), reasonVMStopped, "Colima", start)
}

// colimaStatus reads `colima status --json` (warnings may precede the JSON).
func (d Detector) colimaStatus(ctx context.Context, profile string) (colimaStatus, error) {
	var st colimaStatus
	out, err := d.Runner.Run(ctx, "colima", "status", "--profile", profile, "--json")
	if err != nil {
		return st, err
	}
	i := bytes.IndexByte(out, '{')
	if i < 0 {
		return st, fmt.Errorf("colima status: unexpected output %q", firstLine(string(out)))
	}
	if err := json.NewDecoder(bytes.NewReader(out[i:])).Decode(&st); err != nil {
		return st, fmt.Errorf("colima status: %w", err)
	}
	return st, nil
}

// colimaStopped tells whether a `colima status` error means the VM is not
// running (rather than a status that could not be read).
func colimaStopped(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not running") || strings.Contains(msg, "is stopped") || strings.Contains(msg, "does not exist")
}

// colimaSockets returns the existing docker sockets of a Colima profile.
func (d Detector) colimaSockets(profile string) []string {
	var dirs []string
	if d.ColimaHome != "" {
		dirs = append(dirs, d.ColimaHome)
	}
	if d.Home != "" {
		dirs = append(dirs, filepath.Join(d.Home, ".colima"), filepath.Join(d.Home, ".config", "colima"))
	}
	var out []string
	for _, dir := range dirs {
		s := filepath.Join(dir, profile, "docker.sock")
		if _, err := os.Stat(s); err == nil && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
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
	v, err := d.serverVersion(ctx, cli)
	if err != nil {
		if e.VM {
			if st, serr := d.Runner.Run(ctx, cli, "machine", "inspect", "--format", "{{.State}}"); serr == nil && strings.TrimSpace(string(st)) != "running" {
				return Engine{}, unavailable(name, reasonVMStopped, "Podman", "podman machine start"), true
			}
		}
		return Engine{}, unavailable(name, reasonUnreachable, "Podman", err.Error()), true
	}
	e.Version = v
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
	v, err := d.serverVersion(ctx, cli)
	if err != nil {
		return Engine{}, unavailable(name, reasonUnreachable, "Docker", err.Error()), true
	}
	e.Version = v
	if c, err := d.Runner.Run(ctx, cli, "context", "show"); err == nil {
		e.Details = append(e.Details, "context="+strings.TrimSpace(string(c)))
	}
	return e, available(e), true
}
