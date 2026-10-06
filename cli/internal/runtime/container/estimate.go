package container

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
)

// Build steps reported by Estimate.
const (
	StepBase  = "base"  // project dev environment (dev Dockerfile)
	StepLayer = "layer" // thin oh layer (tool, fake bd)
)

// buildTimes are the last build durations of a project, per role.
type buildTimes struct {
	Base  float64 `json:"base_seconds,omitempty"`
	Layer float64 `json:"layer_seconds,omitempty"`
}

func (r *Runtime) buildTimesFile(projectID string) string {
	return filepath.Join(r.opts.CacheDir, "images", "builds-"+imageName(projectID)+".json")
}

func (r *Runtime) loadBuildTimes(projectID string) buildTimes {
	var t buildTimes
	if data, err := os.ReadFile(r.buildTimesFile(projectID)); err == nil {
		_ = json.Unmarshal(data, &t)
	}
	return t
}

// recordBuild keeps the duration of a build (estimates of the next ones).
func (r *Runtime) recordBuild(projectID, role string, d time.Duration) {
	t := r.loadBuildTimes(projectID)
	if role == "base" {
		t.Base = d.Seconds()
	} else {
		t.Layer = d.Seconds()
	}
	p := r.buildTimesFile(projectID)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return
	}
	data, _ := json.Marshal(t)
	_ = os.WriteFile(p, data, 0o600)
}

// Estimate implements ohruntime.Estimator: whether the project image of a
// group is cached, else what remains to build and how long the last builds
// of the project took. Nothing is built nor run (the base libc is only read
// from the probe cache).
func (r *Runtime) Estimate(ctx context.Context, g ohruntime.Group) (ohruntime.PrepareEstimate, error) {
	if g.Tool == nil {
		return ohruntime.PrepareEstimate{}, errors.New("container: no tool to install in the image")
	}
	e, av := r.Engine(ctx)
	if !av.OK {
		return ohruntime.PrepareEstimate{}, fmt.Errorf("%w: %s", ErrUnavailable, av.Message())
	}
	base, err := planBase(g)
	if err != nil {
		return ohruntime.PrepareEstimate{}, err
	}
	times := r.loadBuildTimes(g.ProjectID)
	steps := func(s ...string) ohruntime.PrepareEstimate {
		out := ohruntime.PrepareEstimate{Steps: s}
		known := true
		for _, st := range s {
			sec := times.Layer
			if st == StepBase {
				sec = times.Base
			}
			if sec <= 0 {
				known = false
			}
			out.Duration += time.Duration(sec * float64(time.Second))
		}
		if !known {
			out.Duration = 0
		}
		return out
	}
	if !r.imageExists(ctx, e, base.ref) {
		return steps(StepBase, StepLayer), nil
	}
	var p baseProbe
	data, err := os.ReadFile(filepath.Join(r.opts.CacheDir, "images", base.hash[:16]+".json"))
	if err != nil || json.Unmarshal(data, &p) != nil || p.Arch == "" {
		return steps(StepLayer), nil //nolint:nilerr // base not probed yet: the layer was never built on it
	}
	bd, err := r.bd(p.Arch)
	if err != nil {
		return ohruntime.PrepareEstimate{}, err
	}
	ref := devRef(g, base.hash, p.Arch, p.Libc, bd)
	if !r.imageExists(ctx, e, ref) {
		out := steps(StepLayer)
		out.Image = ref
		return out, nil
	}
	return ohruntime.PrepareEstimate{Ready: true, Image: ref}, nil
}
