package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/runsvc"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
)

func TestApplyProjectExec(t *testing.T) {
	p := &domain.Project{Path: "/src/app", ExecConfig: &domain.ProjectExecConfig{
		Dockerfile: "docker/dev.Dockerfile", BuildArgs: map[string]string{"NODE": "22"}, Volumes: []string{"node_modules"}}}
	var req runsvc.StartRequest
	applyProjectExec(&req, p)
	assert.Equal(t, "/src/app", req.ProjectDir, "the Dockerfile is looked up in the project, not the location")
	assert.Equal(t, "docker/dev.Dockerfile", req.Dockerfile)
	assert.Equal(t, map[string]string{"NODE": "22"}, req.BuildArgs)
	assert.Equal(t, []string{"node_modules"}, req.Volumes)

	req.BuildArgs["NODE"] = "20"
	assert.Equal(t, "22", p.ExecConfig.BuildArgs["NODE"], "the request gets a copy")

	var empty runsvc.StartRequest
	applyProjectExec(&empty, &domain.Project{Path: "/src/b"})
	assert.Equal(t, "/src/b", empty.ProjectDir)
	assert.Empty(t, empty.Dockerfile)
	assert.Nil(t, empty.BuildArgs)
}

func TestRuntimePrefs(t *testing.T) {
	assert.Equal(t, []string{""}, runtimePrefs(nil, &domain.Project{}))
	assert.Equal(t, []string{"container"}, runtimePrefs(nil, &domain.Project{ExecConfig: &domain.ProjectExecConfig{DefaultRuntime: "container"}}))
}

func TestProjectExecHints(t *testing.T) {
	dir := t.TempDir()
	assert.Empty(t, projectExecHints(dir).DetectedDockerfile)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".devcontainer"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".devcontainer", "Dockerfile"), []byte("FROM alpine\n"), 0o644))
	assert.Equal(t, filepath.Join(".devcontainer", "Dockerfile"), projectExecHints(dir).DetectedDockerfile)
	assert.Empty(t, projectExecHints("").DetectedDockerfile)
}

type stubRuntime struct {
	ohruntime.Runtime
	av ohruntime.Availability
}

func (s stubRuntime) Available(context.Context) (ohruntime.Availability, error) { return s.av, nil }

func TestPinRuntime(t *testing.T) {
	up := stubRuntime{av: ohruntime.Availability{OK: true, Engine: "colima", Version: "28"}}
	assert.Equal(t, up, pinRuntime(up, "", "2.0.20"), "no pin")
	assert.Equal(t, up, pinRuntime(up, "v2.0.20", "2.0.20"), "same version")

	rt := pinRuntime(up, "2.0.19", "2.0.20")
	av, err := rt.Available(context.Background())
	require.NoError(t, err)
	assert.False(t, av.OK)
	assert.Equal(t, "colima", av.Engine)
	assert.Equal(t, "tui.settings.exec.opencode.mismatch", av.Reason)
	_, err = rt.Prepare(context.Background(), ohruntime.Group{})
	assert.Error(t, err, "no image built for another client version")

	down := stubRuntime{av: ohruntime.Availability{Reason: "cmd.runtime.container.vm_stopped"}}
	av, _ = pinRuntime(down, "2.0.19", "2.0.20").Available(context.Background())
	assert.Equal(t, "cmd.runtime.container.vm_stopped", av.Reason, "the engine state comes first")

	assert.True(t, pinnedVersionOK("", "2.0.20"))
	assert.False(t, pinnedVersionOK("2.0.19", "2.0.20"))
}

func TestRuntimePrefsWithSettings(t *testing.T) {
	a := &app.App{Config: &config.Config{Execution: config.ExecutionConfig{Runtime: "container"}}}
	assert.Equal(t, []string{"", "container"}, runtimePrefs(a, &domain.Project{}))
	assert.Equal(t, []string{"local", "container"}, runtimePrefs(a, &domain.Project{ExecConfig: &domain.ProjectExecConfig{DefaultRuntime: "local"}}))
}
