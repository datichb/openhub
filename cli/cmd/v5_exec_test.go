package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/runsvc"
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
