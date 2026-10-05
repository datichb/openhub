package container

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Options configures the container runtime.
type Options struct {
	Engine        EngineKind // auto | colima | podman | docker
	ColimaProfile string
	Runner        Runner // default: ExecRunner
	GOOS          string // default: runtime.GOOS
	CacheDir      string // build contexts and probes (~/.oh/cache/container)
	KeepImages    int    // images kept per project and role (default 2)
}

// Runtime runs server groups in containers.
type Runtime struct {
	opts Options

	mu      sync.Mutex
	engine  *Engine
	bd      func(arch string) ([]byte, error) // fake bd (overridable in tests)
	hostIPs map[EngineKind]string
}

// New returns a container runtime.
func New(opts Options) *Runtime {
	if opts.Runner == nil {
		opts.Runner = ExecRunner{}
	}
	if opts.GOOS == "" {
		opts.GOOS = runtime.GOOS
	}
	if opts.CacheDir == "" {
		opts.CacheDir = filepath.Join(os.TempDir(), "oh-container")
	}
	return &Runtime{opts: opts, bd: bdBinary, hostIPs: map[EngineKind]string{}}
}

// Kind implements ohruntime.Runtime.
func (r *Runtime) Kind() sessionspec.RuntimeKind { return sessionspec.RuntimeContainer }

// Available implements ohruntime.Runtime. The engine is detected again at
// each call (a VM may have been started or stopped meanwhile).
func (r *Runtime) Available(ctx context.Context) (ohruntime.Availability, error) {
	e, av := r.detector().Detect(ctx, r.opts.Engine)
	r.mu.Lock()
	if av.OK {
		r.engine = &e
	} else {
		r.engine = nil
	}
	r.mu.Unlock()
	return av, nil
}

// Engine returns the detected engine, detecting it if needed.
func (r *Runtime) Engine(ctx context.Context) (Engine, ohruntime.Availability) {
	r.mu.Lock()
	e := r.engine
	r.mu.Unlock()
	if e != nil {
		return *e, available(*e)
	}
	av, _ := r.Available(ctx)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.engine == nil {
		return Engine{}, av
	}
	return *r.engine, av
}

// HostAddress implements ohruntime.Runtime: the machine address seen from
// containers of the detected engine ("" before detection).
func (r *Runtime) HostAddress() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.engine == nil {
		return ""
	}
	return r.engine.Host
}

func (r *Runtime) detector() Detector {
	home, _ := os.UserHomeDir()
	return Detector{Runner: r.opts.Runner, GOOS: r.opts.GOOS, ColimaProfile: r.opts.ColimaProfile, Home: home}
}

var _ ohruntime.Runtime = (*Runtime)(nil)

// specFile is the container spec of a group, next to its data directory
// (~/.oh/servers/<group>/container.json).
func specFile(dataDir string) string { return filepath.Join(filepath.Dir(dataDir), "container.json") }

// Prepare implements ohruntime.Runtime: project image (built or cached),
// mounts, path translation, user and network of the group container.
func (r *Runtime) Prepare(ctx context.Context, g ohruntime.Group) (*ohruntime.Prepared, error) {
	e, av := r.Engine(ctx)
	if !av.OK {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, av.Message())
	}
	img, err := r.EnsureImage(ctx, g)
	if err != nil {
		return nil, err
	}
	spec, err := buildSpec(e, specInput{GOOS: r.opts.GOOS, UID: os.Getuid(), GID: os.Getgid()}, g, img.Ref, g.Key.String())
	if err != nil {
		return nil, err
	}
	pg := &ohruntime.Prepared{Group: g, Paths: spec.Paths, HostAddress: e.Host, Spec: &spec}
	if !e.VM {
		if pg.ListenHost, err = r.hostIP(ctx, e, spec); err != nil {
			return nil, err
		}
	}
	if g.DataDir != "" {
		if err := os.MkdirAll(filepath.Dir(specFile(g.DataDir)), 0o700); err != nil {
			return nil, err
		}
		data, _ := json.MarshalIndent(spec, "", "  ")
		if err := os.WriteFile(specFile(g.DataDir), data, 0o600); err != nil {
			return nil, err
		}
	}
	return pg, nil
}

// LoadSpec reads the container spec saved by Prepare for a group data dir.
func LoadSpec(dataDir string) (*Spec, error) {
	data, err := os.ReadFile(specFile(dataDir))
	if err != nil {
		return nil, err
	}
	var s Spec
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// hostIP resolves, from inside a container, the IP of the machine host
// name (Linux: a bridge gateway or the pasta host address). The credential
// proxy must also listen there. Cached per engine.
func (r *Runtime) hostIP(ctx context.Context, e Engine, s Spec) (string, error) {
	r.mu.Lock()
	ip := r.hostIPs[e.Kind]
	r.mu.Unlock()
	if ip != "" {
		return ip, nil
	}
	args := []string{"run", "--rm", "--entrypoint", "sh"}
	for _, h := range s.AddHosts {
		args = append(args, "--add-host", h)
	}
	args = append(args, s.Image, "-c", "getent hosts "+e.Host+" || grep -w "+e.Host+" /etc/hosts")
	out, err := r.opts.Runner.Run(ctx, e.CLI, e.Command(args...)[1:]...)
	if err != nil {
		return "", fmt.Errorf("resolving %s from a container: %w", e.Host, err)
	}
	for _, f := range strings.Fields(string(out)) {
		if p := net.ParseIP(f); p != nil && p.To4() != nil {
			ip = p.String()
			break
		}
	}
	if ip == "" {
		return "", fmt.Errorf("%s does not resolve inside containers", e.Host)
	}
	r.mu.Lock()
	r.hostIPs[e.Kind] = ip
	r.mu.Unlock()
	return ip, nil
}

// cliEnvKeys are the machine variables the engine CLI needs (connection,
// configuration); nothing else from the oh environment reaches it.
var cliEnvKeys = []string{"PATH", "HOME", "USER", "LOGNAME", "TMPDIR", "XDG_RUNTIME_DIR", "XDG_CONFIG_HOME",
	"DOCKER_HOST", "DOCKER_CONTEXT", "DOCKER_CONFIG", "DOCKER_CERT_PATH", "DOCKER_TLS_VERIFY",
	"CONTAINER_HOST", "CONTAINER_CONNECTION", "CONTAINERS_CONF", "CONTAINER_SSHKEY"}

// Command implements ohruntime.Runtime: `<cli> run --rm --init --name …`
// attached, so that the returned process lives as long as the container and
// signals sent to it are forwarded (stopping it stops the container). A stale
// container of the same group is removed first.
func (r *Runtime) Command(ctx context.Context, pg *ohruntime.Prepared, p ohruntime.Proc) (*exec.Cmd, error) {
	s, ok := pg.Spec.(*Spec)
	if !ok || s == nil {
		return nil, errors.New("container: group not prepared")
	}
	e, av := r.Engine(ctx)
	if !av.OK {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, av.Message())
	}
	_, _ = r.opts.Runner.Run(ctx, e.CLI, e.Command("rm", "-f", s.Name)[1:]...)
	dir := r.opts.CacheDir
	if pg.Group.DataDir != "" {
		dir = filepath.Dir(specFile(pg.Group.DataDir))
	}
	envFile := filepath.Join(dir, "container.env")
	args, content, err := runArgs(*s, p, envFile)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(envFile, []byte(content), 0o600); err != nil {
		return nil, err
	}
	cmd := exec.Command(e.CLI, append(append([]string{}, e.Args...), args...)...)
	for _, k := range cliEnvKeys {
		if v, ok := os.LookupEnv(k); ok {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	return cmd, nil
}

// Teardown implements ohruntime.Runtime: the container is removed (it is
// normally gone already: --rm).
func (r *Runtime) Teardown(ctx context.Context, pg *ohruntime.Prepared) error {
	s, ok := pg.Spec.(*Spec)
	if !ok || s == nil {
		return nil
	}
	e, av := r.Engine(ctx)
	if !av.OK {
		return nil
	}
	// "no such container" is the normal case.
	_, _ = r.opts.Runner.Run(ctx, e.CLI, e.Command("rm", "-f", s.Name)[1:]...)
	return nil
}
