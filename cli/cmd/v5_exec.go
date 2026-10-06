package cmd

import (
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/runsvc"
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
// most specific first. The workflow keeps the last word
// (`runtime.allowed`, then `runtime.default`).
func runtimePrefs(_ *app.App, p *domain.Project) []string {
	return []string{projectExec(p).DefaultRuntime}
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
