package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// webhookRecorder is a fake incoming webhook.
type webhookRecorder struct {
	mu    sync.Mutex
	texts []string
}

func (w *webhookRecorder) list() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.texts...)
}

// sinkTeam is a team-state on a bare repository whose webhook is a fake, with
// the event sink of oh installed.
func sinkTeam(t *testing.T, notification string) (*teamstate.Repo, *webhookRecorder) {
	t.Helper()
	if testing.Short() {
		t.Skip("runs git")
	}
	i18ntest.Set(t, "fr")
	rec := &webhookRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m struct {
			Text string `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&m)
		rec.mu.Lock()
		rec.texts = append(rec.texts, m.Text)
		rec.mu.Unlock()
	}))
	t.Cleanup(srv.Close)

	bare, clone := gittest.BareClone(t)
	require.NoError(t, os.WriteFile(filepath.Join(clone, "members.toml"), []byte(`
[members.alice]
display_name = "Alice M."
mattermost_username = "alice.m"
role = "dev"

[members.bob]
display_name = "Bob"
role = "dev"
`), 0o644))
	cfg := "[notification]\nenabled = true\ntype = \"mattermost\"\nwebhook_url = \"" + srv.URL + "/hooks/secret\"\n" + notification
	require.NoError(t, os.WriteFile(filepath.Join(clone, "config.toml"), []byte(cfg), 0o644))
	gittest.Run(t, clone, "add", "-A")
	gittest.Run(t, clone, "commit", "-m", "seed")
	gittest.Run(t, clone, "push", "origin", "HEAD")

	teamstate.SetEventSink(teamEventSink)
	t.Cleanup(func() { teamstate.SetEventSink(nil) })
	return teamstate.NewRepo(bare, clone), rec
}

// A review asked of someone is announced to the webhook of the team and
// mentions them; the event is announced once it is written, by whoever wrote it.
func TestEventSinkAnnouncesAReviewAndMentionsTheReviewer(t *testing.T) {
	repo, rec := sinkTeam(t, "")
	require.NoError(t, repo.AppendEvent(t.Context(), teamstate.Event{
		Actor: "bob", Type: teamstate.EventReviewReady, Project: "api", Ticket: "bd-7",
		Data: map[string]any{"reviewer": "alice", "mr_url": "https://gitlab.example.com/mr/1"},
	}))
	require.True(t, teamstate.WaitEventSinks(10*time.Second))
	got := rec.list()
	require.Len(t, got, 1)
	assert.True(t, strings.HasPrefix(got[0], "@alice.m "), got[0])
	assert.Contains(t, got[0], "Bob")
	assert.Contains(t, got[0], "https://gitlab.example.com/mr/1")
}

// The events outside the matrix of the webhook (claims, configuration) are not
// sent unless the team asks for them.
func TestEventSinkSendsWhatTheDestinationAskedFor(t *testing.T) {
	repo, rec := sinkTeam(t, "events = [\"claim.*\"]\n")
	ctx := t.Context()
	require.NoError(t, repo.AppendEvent(ctx, teamstate.Event{Actor: "bob", Type: teamstate.EventClaimTransferred, Project: "api", Ticket: "bd-7", Data: map[string]any{"to": "alice"}}))
	require.NoError(t, repo.AppendEvent(ctx, teamstate.Event{Actor: "bob", Type: teamstate.EventReviewReady, Project: "api", Ticket: "bd-7"}))
	require.True(t, teamstate.WaitEventSinks(10*time.Second))
	got := rec.list()
	require.Len(t, got, 1, "claims only, not the review")
	assert.True(t, strings.HasPrefix(got[0], "@alice.m "), "the member a claim went to is mentioned")
	assert.Contains(t, got[0], "a transféré api/bd-7 à Alice M.")
}

func TestEventSinkIsSilentWhenNotificationsAreOff(t *testing.T) {
	repo, rec := sinkTeam(t, "")
	cfgPath := filepath.Join(repo.Path(), "config.toml")
	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cfgPath, []byte(strings.Replace(string(data), "enabled = true", "enabled = false", 1)), 0o644))
	require.NoError(t, repo.AppendEvent(t.Context(), teamstate.Event{Actor: "bob", Type: teamstate.EventReviewReady, Project: "api", Ticket: "bd-7"}))
	require.True(t, teamstate.WaitEventSinks(10*time.Second))
	assert.Empty(t, rec.list())
}
