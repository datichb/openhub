package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
)

func settingsField(t *testing.T, v *SettingsView, key string) *configField {
	t.Helper()
	for i := range v.fields {
		if v.fields[i].Key == key {
			return &v.fields[i]
		}
	}
	t.Fatalf("field %s not found", key)
	return nil
}

func TestSettingsExecSection(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	v := NewSettingsView(SettingsViewConfig{ToolVersion: func() string { return "2.0.20" }})
	v.live = cfg
	v.buildFields()

	eng := settingsField(t, v, "exec_engine")
	assert.Equal(t, "auto", eng.Get())
	eng.Set("podman")
	assert.Equal(t, "podman", cfg.Execution.Engine)
	eng.Set("auto")
	assert.Empty(t, cfg.Execution.Engine, "auto is stored as nothing")

	settingsField(t, v, "exec_runtime").Set("container")
	assert.Equal(t, "container", cfg.Execution.Runtime)

	keep := settingsField(t, v, "exec_keep_images")
	assert.Equal(t, "", keep.Get())
	assert.Equal(t, 2, cfg.Execution.Images())
	require.Error(t, keep.Validator.Validate("0"))
	keep.Set("3")
	assert.Equal(t, 3, cfg.Execution.Images())

	oc := settingsField(t, v, "exec_tool_version")
	assert.Contains(t, oc.Source(), "2.0.20")
	require.Error(t, oc.Validator.Validate("latest"))
	require.NoError(t, oc.Validator.Validate("v2.0.20"))
	oc.Set("2.0.19")
	assert.Contains(t, oc.Source(), "2.0.19", "mismatch shown")
	oc.Set("2.0.20")
	assert.Empty(t, oc.Source())

	st := settingsField(t, v, "exec_strict_isolation")
	assert.Equal(t, "false", st.Get())
	st.Set("true")
	assert.True(t, cfg.Execution.StrictIsolation)
}

func TestSettingsExecContainerMemory(t *testing.T) {
	useEnglish(t)
	cfg := &config.Config{}
	v := NewSettingsView(SettingsViewConfig{})
	v.live = cfg
	v.buildFields()

	f := settingsField(t, v, "exec_container_memory")
	assert.Equal(t, CfgFieldString, f.Kind)
	assert.Empty(t, f.Get(), "no limit by default")
	assert.NotEmpty(t, f.Description)
	for _, ok := range []string{"", "4g", "4G", "512m"} {
		require.NoError(t, f.Validator.Validate(ok), ok)
	}
	for _, bad := range []string{"4", "256m", "4gb", "2000g", "x"} {
		require.Error(t, f.Validator.Validate(bad), bad)
	}
	f.Set("4G")
	assert.Equal(t, "4g", cfg.Execution.ContainerMemory, "same normalization as oh config set")
	assert.Equal(t, "4g", f.Get())
	f.Set("")
	assert.Empty(t, cfg.Execution.ContainerMemory)
}
