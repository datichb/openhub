package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
)

func TestSettingsRemoteFields(t *testing.T) {
	t.Parallel()
	v := &SettingsView{live: &config.Config{}}
	f := v.remoteFields()
	if len(f) != 3 || f[0].Kind != CfgFieldSectionHeader || f[1].Key != "remote_experimental" || f[2].Kind != CfgFieldPlaceholder {
		t.Fatalf("no target: %+v", f)
	}

	v.live.Remote.Targets = []config.RemoteTarget{
		{Name: "acme", URL: "https://gitlab.com", Group: "acme"},
		{Name: "corp", URL: "https://git.corp", Group: "dev", Builder: "dind", Tag: "oh-corp"},
	}
	byKey := map[string][]configField{}
	for _, fld := range v.remoteFields() {
		byKey[fld.Key] = append(byKey[fld.Key], fld)
	}
	if got := byKey["remote_runner_project"][0].Get(); got != "acme/oh-runner" {
		t.Fatalf("runner project = %q", got)
	}
	if got := byKey["remote_builder"][0].Get(); got != "kaniko" {
		t.Fatalf("default builder = %q", got)
	}
	if got := byKey["remote_builder"][1].Get(); got != "dind" {
		t.Fatalf("builder = %q", got)
	}
	byKey["remote_tag"][1].Set("gpu")
	if v.live.Remote.Targets[1].Tag != "gpu" || v.live.Remote.Targets[0].Tag != "" {
		t.Fatalf("set tag on the wrong target: %+v", v.live.Remote.Targets)
	}
	if got := byKey["remote_token_key"][0].Get(); got != "openhub.remote.acme.token" {
		t.Fatalf("token key = %q", got)
	}

	// The undo snapshot does not share the targets with the live config.
	cp := deepCopyConfig(v.live)
	cp.Remote.Targets[0].Tag = "changed"
	if v.live.Remote.Targets[0].Tag != "" {
		t.Fatal("deep copy shares the targets")
	}
}

// remote.experimental is set in the « Remote » section, with or without a
// target, saved by the common write path (UpdateConfig) like every setting,
// and the existing warning of the mode is shown when it is enabled.
func TestSettingsRemoteExperimental(t *testing.T) {
	useEnglish(t)
	shared := seedHome(t)
	v := NewSettingsView(SettingsViewConfig{
		GetConfig:    func() *config.Config { return shared },
		UpdateConfig: hubHost{shared}.update,
	})
	v.live = shared
	v.buildFields()

	f := settingsField(t, v, "remote_experimental")
	require.Equal(t, CfgFieldBool, f.Kind)
	assert.Equal(t, "false", f.Get())
	assert.Empty(t, f.Source(), "no warning while it is off")
	assert.NotEmpty(t, f.Description)

	sh := &typingShell{}
	v.shell = sh
	v.pushUndo()
	editConfigField(sh, f, func() { v.scheduleAutoSave() })
	assert.Equal(t, "true", f.Get())
	assert.Equal(t, i18n.T("cmd.remote.experimental.warning"), f.Source())
	require.Len(t, sh.toasts, 1)
	assert.Equal(t, i18n.T("cmd.remote.experimental.warning"), sh.toasts[0])

	v.doSave()
	got, err := config.Read()
	require.NoError(t, err)
	assert.True(t, got.Remote.Experimental, "written to hub.toml by UpdateConfig")

	v.pushUndo()
	toggleConfigField(f)
	sh.toasts = nil
	assert.Equal(t, "false", f.Get())
	assert.Empty(t, f.Source())
}
