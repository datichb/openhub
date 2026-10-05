package container

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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

	mu     sync.Mutex
	engine *Engine
	bd     func(arch string) ([]byte, error) // fake bd (overridable in tests)
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
	return &Runtime{opts: opts, bd: bdBinary}
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
	return Detector{Runner: r.opts.Runner, GOOS: r.opts.GOOS, ColimaProfile: r.opts.ColimaProfile}
}
