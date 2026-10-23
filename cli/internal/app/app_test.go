package app

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/platform"
)

// The stubs only need to be told apart: the container stores them, it never
// calls them.
type (
	projectStore    struct{ domain.ProjectStore }
	sessionStore    struct{ domain.SessionStore }
	eventStore      struct{ domain.AgentEventStore }
	secretStore     struct{ domain.SecretStore }
	preferenceStore struct{ domain.PreferenceStore }
	usageReader     struct{ domain.WorkflowUsageReader }
	statsProvider   struct{ platform.StatsProvider }
)

func TestAppContainer(t *testing.T) {
	configtest.Home(t)
	i18ntest.Restore(t)
	require.NoError(t, config.Save(&config.Config{CLI: config.CLIConfig{Language: "fr"}}), "writing the hub config")
	config.Reset()

	a, err := New()
	require.NoError(t, err, "New loads the hub config")
	require.NotNil(t, a.Config, "the config is loaded")
	assert.Equal(t, "fr", a.Config.CLI.Language, "the config is the one of the hub")
	assert.Equal(t, "fr", i18n.Locale(), "the messages follow the language of the config")
	assert.Equal(t, &IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}, a.IO, "the IO streams are the standard ones")
	assert.Nil(t, a.Projects, "the stores are wired by the caller")

	var (
		ps    = &projectStore{}
		ss    = &sessionStore{}
		es    = &eventStore{}
		sec   = &secretStore{}
		pref  = &preferenceStore{}
		usage = &usageReader{}
		stats = &statsProvider{}
		io    = &IOStreams{}
	)
	toolErr := errors.New("tool missing")
	chained := a.WithProjectStore(ps).WithSessionStore(ss).WithAgentEventStore(es).WithSecretStore(sec).
		WithPreferences(pref, usage).WithIO(io).WithStats(stats).
		WithToolVersion(func() (string, error) { return "", toolErr })

	assert.Same(t, a, chained, "the With* methods chain on the same App")
	assert.Same(t, ps, a.Projects, "project store")
	assert.Same(t, ss, a.Sessions, "session store")
	assert.Same(t, es, a.AgentEvents, "agent event store")
	assert.Same(t, sec, a.Secrets, "secret store")
	assert.Same(t, pref, a.Preferences, "preference store")
	assert.Same(t, usage, a.WorkflowUsage, "workflow usage reader")
	assert.Same(t, stats, a.Stats, "stats provider")
	assert.Same(t, io, a.IO, "IO streams")
	_, err = a.ToolVersion()
	assert.ErrorIs(t, err, toolErr, "tool version probe")

	assert.Same(t, a, a.WithSecretStore(nil), "a nil store is accepted")
	assert.Nil(t, a.Secrets, "and removes the secret store")
}

type (
	decisionStore   struct{ domain.DecisionStore }
	usageStore      struct{ domain.UsageStore }
	serverStore     struct{ domain.ServerStore }
	grantStore      struct{ domain.GrantStore }
	remoteStore     struct{ domain.RemoteStore }
	checkpointStore struct{ domain.CheckpointStorage }
	reportStore     struct{ domain.ReportStore }
	wikiStore       struct{ domain.WikiProposalStore }
	historyStore    struct{ domain.HistoryStore }
	fanoutStore     struct{ domain.FanoutStore }
)

// A degraded startup builds the App without database: every store is nil,
// then the stores are wired one by one.
func TestAppStoresNilUntilWired(t *testing.T) {
	a := &App{}
	stores := func() map[string]any {
		return map[string]any{
			"Decisions": a.Decisions, "Usage": a.Usage, "Servers": a.Servers, "Grants": a.Grants, "Remotes": a.Remotes,
			"Checkpoints": a.Checkpoints, "Reports": a.Reports, "WikiProposals": a.WikiProposals, "History": a.History, "Fanout": a.Fanout,
		}
	}
	for name, s := range stores() {
		assert.Nil(t, s, "%s is nil on a degraded startup", name)
	}

	var (
		dec  = &decisionStore{}
		use  = &usageStore{}
		srv  = &serverStore{}
		gr   = &grantStore{}
		rem  = &remoteStore{}
		chk  = &checkpointStore{}
		rep  = &reportStore{}
		wiki = &wikiStore{}
		hist = &historyStore{}
		fo   = &fanoutStore{}
	)
	chained := a.WithDecisions(dec).WithUsage(use).WithServers(srv, gr).WithRemotes(rem).WithCheckpoints(chk).
		WithReports(rep).WithWikiProposals(wiki).WithHistory(hist).WithFanout(fo)

	assert.Same(t, a, chained, "the With* methods chain on the same App")
	assert.Same(t, dec, a.Decisions, "decision store")
	assert.Same(t, use, a.Usage, "usage store")
	assert.Same(t, srv, a.Servers, "server store")
	assert.Same(t, gr, a.Grants, "grant store")
	assert.Same(t, rem, a.Remotes, "remote store")
	assert.Same(t, chk, a.Checkpoints, "checkpoint store")
	assert.Same(t, rep, a.Reports, "report store")
	assert.Same(t, wiki, a.WikiProposals, "wiki proposal store")
	assert.Same(t, hist, a.History, "history store")
	assert.Same(t, fo, a.Fanout, "fan-out store")
	for name, s := range stores() {
		assert.NotNil(t, s, "%s is set", name)
	}
}
