package config_test

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGitRemote(t *testing.T) {
	for raw, want := range map[string]config.GitRemote{
		"https://gitlab.com/acme/dev/api.git":            {"gitlab.com", "acme/dev/api"},
		"https://GitLab.example.com:8443/acme/api":       {"gitlab.example.com:8443", "acme/api"},
		"git@gitlab.com:acme/dev/api.git":                {"gitlab.com", "acme/dev/api"},
		"ssh://git@gitlab.example.com:2222/acme/api.git": {"gitlab.example.com", "acme/api"},
	} {
		got, err := config.ParseGitRemote(raw)
		require.NoError(t, err, raw)
		assert.Equal(t, want, got, raw)
	}
	_, err := config.ParseGitRemote("not a remote")
	assert.Error(t, err)
}

func TestMatchTarget(t *testing.T) {
	r := config.RemoteConfig{
		Targets: []config.RemoteTarget{
			{Name: "acme", URL: "https://gitlab.com", Group: "acme"},
			{Name: "acme-dev", URL: "https://gitlab.com", Group: "acme/dev"},
			{Name: "corp", URL: "https://gitlab.corp.example:8443", Group: "acme"},
		},
		Projects: map[string]string{"forced": "acme"},
	}
	m := func(project, raw string) string {
		gr, err := config.ParseGitRemote(raw)
		require.NoError(t, err)
		if tg := r.MatchTarget(project, gr); tg != nil {
			return tg.Name
		}
		return ""
	}
	assert.Equal(t, "acme-dev", m("p", "git@gitlab.com:acme/dev/api.git"), "deepest group wins")
	assert.Equal(t, "acme", m("p", "https://gitlab.com/acme/web.git"))
	assert.Equal(t, "acme", m("forced", "git@gitlab.com:acme/dev/api.git"), "explicit project entry")
	assert.Equal(t, "corp", m("p", "ssh://git@gitlab.corp.example:22/acme/api.git"), "ssh port ignored")
	assert.Equal(t, "", m("p", "https://gitlab.com/other/api.git"))
	assert.Equal(t, "", m("p", "https://gitlab.com/acmeX/api.git"), "group prefix on a path boundary")
}

func TestRemoteTargetValidate(t *testing.T) {
	ok := config.RemoteTarget{Name: "acme", URL: "https://gitlab.com", Group: "acme"}
	assert.NoError(t, ok.Validate())
	assert.Equal(t, "acme/oh-runner", ok.RunnerProjectPath())
	assert.Equal(t, "openhub.remote.acme.token", ok.TokenKeyOrDefault())
	for _, bad := range []config.RemoteTarget{
		{Name: "Acme", URL: "https://gitlab.com", Group: "acme"},
		{Name: "acme", URL: "gitlab.com", Group: "acme"},
		{Name: "acme", URL: "https://gitlab.com/x", Group: "acme"},
		{Name: "acme", URL: "https://gitlab.com", Group: ""},
	} {
		assert.Error(t, bad.Validate(), "%+v", bad)
	}
	var r config.RemoteConfig
	r.Upsert(ok)
	ok.Tag = "x"
	r.Upsert(ok)
	require.Len(t, r.Targets, 1)
	assert.Equal(t, "x", r.Target("acme").Tag)
}

func TestRemoteConfigRoundtrip(t *testing.T) {
	configtest.Home(t)
	cfg, err := config.Load()
	require.NoError(t, err)
	cfg.Remote.Upsert(config.RemoteTarget{Name: "acme", URL: "https://gitlab.com", Group: "acme/dev", RunnerProjectID: 42,
		Tag: "oh", Binaries: map[string]string{"5.0.0-dev/amd64": "abc"}})
	cfg.Remote.Projects = map[string]string{"proj-1": "acme"}
	require.NoError(t, config.Save(cfg))

	config.Reset()
	got, err := config.Load()
	require.NoError(t, err)
	require.Len(t, got.Remote.Targets, 1)
	tg := got.Remote.Target("acme")
	assert.Equal(t, int64(42), tg.RunnerProjectID)
	assert.Equal(t, "abc", tg.Binaries["5.0.0-dev/amd64"])
	assert.Equal(t, "acme", got.Remote.Projects["proj-1"])
}
