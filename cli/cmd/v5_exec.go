package cmd

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/runsvc"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/runtime/container"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Execution settings of a project (P4-T09, `project.config` › Exécution):
// dev image of the container runtime, default workflow and runtime.

// projectExec returns the execution settings of a project (zero value when
// unset).
func projectExec(p *domain.Project) domain.ProjectExecConfig {
	if p == nil || p.ExecConfig == nil {
		return domain.ProjectExecConfig{}
	}
	return *p.ExecConfig.Clone()
}

// applyProjectExec fills the container settings of a request from the
// project: the dev Dockerfile is looked up in the project directory (not in
// the session location, which may be a worktree), at launch and at resume.
func applyProjectExec(req *runsvc.StartRequest, p *domain.Project) {
	if p == nil {
		return
	}
	ex := projectExec(p)
	req.ProjectDir = p.Path
	req.Dockerfile, req.BuildArgs, req.Volumes = ex.Dockerfile, ex.BuildArgs, ex.Volumes
}

// runtimePrefs are the preferred runtimes of a launch without --runtime,
// most specific first: the project default, then the Settings one. The
// workflow keeps the last word (`runtime.allowed`, then `runtime.default`).
func runtimePrefs(a *app.App, p *domain.Project) []string {
	prefs := []string{projectExec(p).DefaultRuntime}
	if a != nil && a.Config != nil {
		prefs = append(prefs, a.Config.Execution.Runtime)
	}
	return prefs
}

// pinRuntime refuses container launches when the pinned tool version
// (Settings › Exécution) differs from the machine client: the image and the
// client must run the same version. "" = no pin.
func pinRuntime(rt ohruntime.Runtime, pinned, client string) ohruntime.Runtime {
	pinned = strings.TrimPrefix(strings.TrimSpace(pinned), "v")
	if pinned == "" || pinned == strings.TrimPrefix(client, "v") {
		return rt
	}
	return pinnedRuntime{Runtime: rt, pinned: pinned, client: client}
}

type pinnedRuntime struct {
	ohruntime.Runtime
	pinned, client string
}

func (r pinnedRuntime) unavailable() ohruntime.Availability {
	return ohruntime.Availability{Reason: "tui.settings.exec.opencode.mismatch", Args: []any{r.pinned, r.client}}
}

// Available reports the version mismatch (the engine state comes first).
func (r pinnedRuntime) Available(ctx context.Context) (ohruntime.Availability, error) {
	av, err := r.Runtime.Available(ctx)
	if err != nil || !av.OK {
		return av, err
	}
	out := r.unavailable()
	out.Engine, out.Version = av.Engine, av.Version
	return out, nil
}

// Prepare refuses to build an image for another client version.
func (r pinnedRuntime) Prepare(context.Context, ohruntime.Group) (*ohruntime.Prepared, error) {
	return nil, errors.New(r.unavailable().Message())
}

// pinnedVersionOK tells whether the pinned version matches the machine client.
func pinnedVersionOK(pinned, client string) bool {
	_, ok := pinRuntime(nil, pinned, client).(pinnedRuntime)
	return !ok
}

// projectExecHints detects the dev Dockerfile of a project (config view).
func projectExecHints(projectPath string) views.ProjectExecHints {
	var h views.ProjectExecHints
	if projectPath == "" {
		return h
	}
	if df, err := container.DetectDockerfile(projectPath, ""); err == nil && df != "" {
		if rel, err := filepath.Rel(projectPath, df); err == nil {
			df = rel
		}
		h.DetectedDockerfile = df
	}
	return h
}

// tuiWorkflowIDs lists the workflows of the cached catalogue (default
// workflow choice).
func tuiWorkflowIDs() []string {
	if tuiStartWiring == nil {
		return nil
	}
	return tuiStartWiring.workflowIDs()
}

// v5Ver is the opencode V2 client version, once detected.
var v5Ver atomic.Value

// v5ToolVersion returns the detected client version without detecting it
// (event loop safe; "" before the first detection).
func v5ToolVersion() string {
	s, _ := v5Ver.Load().(string)
	return s
}

// Estimate delegates to the wrapped runtime (launch form).
func (r pinnedRuntime) Estimate(ctx context.Context, g ohruntime.Group) (ohruntime.PrepareEstimate, error) {
	if e, ok := r.Runtime.(ohruntime.Estimator); ok {
		return e.Estimate(ctx, g)
	}
	return ohruntime.PrepareEstimate{}, errors.New(r.unavailable().Message())
}
