package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestParseBuildArgs(t *testing.T) {
	m, err := ParseBuildArgs(" NODE=22 , PY_VER=3.12,EMPTY=")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"NODE": "22", "PY_VER": "3.12", "EMPTY": ""}, m)
	assert.Equal(t, "EMPTY=, NODE=22, PY_VER=3.12", FormatBuildArgs(m))

	m, err = ParseBuildArgs("  ")
	require.NoError(t, err)
	assert.Nil(t, m)
	for _, bad := range []string{"NODE", "1X=2", "A B=1", "=x"} {
		_, err := ParseBuildArgs(bad)
		assert.Error(t, err, bad)
	}
}

func TestParseVolumes(t *testing.T) {
	l, err := ParseVolumes("node_modules, /root/.cache ,node_modules/, ./vendor")
	require.NoError(t, err)
	assert.Equal(t, []string{"node_modules", "/root/.cache", "vendor"}, l)
	for _, bad := range []string{"..", "../x", "a/../../b", "/", ".", "vol:/data"} {
		_, err := ParseVolumes(bad)
		assert.Error(t, err, bad)
	}
}

func execField(t *testing.T, v *ProjectConfigView, key string) *configField {
	t.Helper()
	for i := range v.fields {
		if v.fields[i].Key == key {
			return &v.fields[i]
		}
	}
	t.Fatalf("field %s not found", key)
	return nil
}

func TestProjectConfigExecSection(t *testing.T) {
	v := NewProjectConfigView(ProjectConfigViewConfig{WorkflowIDs: func() []string { return []string{"ticket", "review"} }})
	v.live = &domain.Project{ID: "p", Path: "/src/p"}
	v.execHints = ProjectExecHints{DetectedDockerfile: "Dockerfile.dev"}
	v.buildFields()

	// Order: section before the links.
	var exec, links int
	for i, f := range v.fields {
		switch f.Key {
		case "exec_dockerfile":
			exec = i
		case "mcp_services":
			links = i
		}
	}
	assert.Less(t, exec, links)

	df := execField(t, v, "exec_dockerfile")
	assert.Equal(t, "", df.Get())
	assert.Contains(t, df.Source(), "Dockerfile.dev")
	assert.Nil(t, v.live.ExecConfig, "reading does not create the config")

	df.Set(" docker/dev.Dockerfile ")
	assert.Equal(t, "docker/dev.Dockerfile", v.live.ExecConfig.Dockerfile)
	assert.Empty(t, df.Source(), "configured: no detection hint")

	ba := execField(t, v, "exec_build_args")
	require.Error(t, ba.Validator.Validate("NODE"))
	ba.Set("NODE=22")
	assert.Equal(t, "NODE=22", ba.Get())

	vol := execField(t, v, "exec_volumes")
	require.Error(t, vol.Validator.Validate("../x"))
	vol.Set("node_modules, .cache")
	assert.Equal(t, []string{"node_modules", ".cache"}, v.live.ExecConfig.Volumes)

	wf := execField(t, v, "exec_default_workflow")
	opts := wf.OptionsFunc()
	require.Len(t, opts, 3)
	assert.Equal(t, "", opts[0].Value)
	wf.Set("ticket")
	execField(t, v, "exec_default_runtime").Set("container")
	assert.Equal(t, &domain.ProjectExecConfig{Dockerfile: "docker/dev.Dockerfile", BuildArgs: map[string]string{"NODE": "22"},
		Volumes: []string{"node_modules", ".cache"}, DefaultWorkflow: "ticket", DefaultRuntime: "container"}, v.live.ExecConfig)

	// Undo snapshots are deep copies.
	snap := deepCopyProject(v.live)
	v.live.ExecConfig.BuildArgs["NODE"] = "20"
	assert.Equal(t, "22", snap.ExecConfig.BuildArgs["NODE"])
}

func TestStartSectionDefaultFirst(t *testing.T) {
	cfg := StartSectionConfig{Entries: func(StartScope) StartEntries {
		return StartEntries{Loaded: true, Total: 2, Default: &StartEntry{ID: "ticket"},
			Pinned: []StartEntry{{ID: "review", Pinned: true}}}
	}}
	_, items, ok := startSection(cfg, StartScope{ProjectID: "p"}, false, nil)
	require.True(t, ok)
	require.GreaterOrEqual(t, len(items), 2)
	assert.Equal(t, "ticket", items[0].Label)
	assert.Equal(t, "◆", items[0].Icon)
	assert.Equal(t, "review", items[1].Label)
}
