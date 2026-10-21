package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

type datedSecrets struct{ at time.Time }

func (datedSecrets) Get(context.Context, string) (string, error) { return "", nil }
func (s datedSecrets) ModifiedAt(context.Context, string) (time.Time, error) {
	return s.at, nil
}

// The provider refusing the credential of a group raises an alert on its
// live sessions naming the key, its date and the command to renew it; once
// per credential value.
func TestAuthRejectedAlert(t *testing.T) {
	prev := i18n.Locale()
	i18n.SetLocale("en")
	t.Cleanup(func() { i18n.SetLocale(prev) })
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	sessions, decisions, servers := sqlite.NewSessionStore(st), sqlite.NewDecisionStore(st), sqlite.NewServerStore(st)
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g1", ProjectID: "p1", Status: domain.ServerReady}))
	for _, s := range []domain.Session{
		{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunActive},
		{ID: "ses_b", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunCompleted},
		{ID: "ses_c", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g2", State: domain.RunActive},
	} {
		require.NoError(t, sessions.Create(ctx, &s))
	}
	stored := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	d := &Daemon{opts: Options{Sessions: sessions, Decisions: decisions, Servers: servers, Secrets: datedSecrets{at: stored},
		ProjectName: func(context.Context, string) string { return "demo" }},
		proxy: credproxy.New(), watchers: map[string]*watcher{}, feed: newHub()}

	src := domain.CredentialSource{Kind: domain.CredentialBearer, KeychainKey: "openhub.provider.bedrock.token.p1", Scope: "project"}
	rej := credproxy.AuthRejection{Owner: "g1", Provider: credproxy.ProviderBedrock, Status: 403,
		Message: "Authentication failed: Please make sure your API Key is valid.", Credential: credentialRef(src, "old"), At: time.Now()}
	d.raiseAuthRejected(ctx, rej)

	open := func(id string) []domain.Decision {
		l, err := decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: id})
		require.NoError(t, err)
		return l
	}
	l := open("ses_a")
	require.Len(t, l, 1)
	assert.True(t, isOhAlert(l[0]), "dismissed by the user only")
	assert.Equal(t, "Bedrock key rejected by AWS", l[0].Payload.Title)
	assert.Contains(t, l[0].Payload.Message, "the key of project demo (openhub.provider.bedrock.token.p1)")
	assert.Contains(t, l[0].Payload.Message, stored.Local().Format("2006-01-02"))
	assert.Contains(t, l[0].Payload.Message, "oh provider setup bedrock -p demo")
	assert.Contains(t, l[0].Payload.Message, "HTTP 403")
	assert.NotContains(t, l[0].Payload.Message, "old")
	assert.Empty(t, open("ses_b"), "ended session")
	assert.Empty(t, open("ses_c"), "other group")

	d.raiseAuthRejected(ctx, rej)
	assert.Len(t, open("ses_a"), 1, "raised once per credential value")

	rej.Credential = credentialRef(src, "renewed")
	d.raiseAuthRejected(ctx, rej)
	assert.Len(t, open("ses_a"), 2, "a renewed key refused again is raised")
}

func TestCredentialRefNeverHoldsTheSecret(t *testing.T) {
	src := domain.CredentialSource{Kind: domain.CredentialBearer, KeychainKey: "k", Scope: "hub"}
	a, b := credentialRef(src, "s1"), credentialRef(src, "s2")
	assert.NotEqual(t, a.ID, b.ID)
	assert.Equal(t, "k", a.Key)
	assert.NotContains(t, a.ID, "s1")
}
