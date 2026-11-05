package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

func TestFormatEvent(t *testing.T) {
	i18ntest.Set(t, "fr")
	ev := func(typ string, data map[string]any) teamstate.Event {
		return teamstate.Event{Type: typ, Actor: "benjamin", Project: "T-SRU", Ticket: "SRU-142", Data: data}
	}
	tests := []struct {
		name     string
		event    teamstate.Event
		expected string
	}{
		{"session complete with duration", ev(teamstate.EventSessionComplete, map[string]any{"duration_s": float64(4500)}), "✔ benjamin a terminé une session sur T-SRU/SRU-142 (75 min)"},
		{"session complete with summary", ev(teamstate.EventSessionComplete, map[string]any{"duration_s": float64(120), "summary": "fix login bug"}), "✔ benjamin a terminé une session sur T-SRU/SRU-142 (2 min) — fix login bug"},
		{"session complete zero duration", ev(teamstate.EventSessionComplete, map[string]any{"duration_s": float64(0)}), "✔ benjamin a terminé une session sur T-SRU/SRU-142"},
		{"claim taken", ev(teamstate.EventClaimTaken, nil), "● benjamin a pris T-SRU/SRU-142"},
		{"claim conflict", ev(teamstate.EventClaimConflict, map[string]any{"current_owner": "benjamin"}), "✗ benjamin conflit de prise sur T-SRU/SRU-142 (déjà pris par benjamin)"},
		{"review ready", ev(teamstate.EventReviewReady, map[string]any{"mr_url": "https://gitlab.com/mr/234"}), "◐ benjamin revue prête · T-SRU/SRU-142 — https://gitlab.com/mr/234"},
		{"wiki proposal", teamstate.Event{Type: teamstate.EventWikiProposal, Actor: "documentarian", Project: "T-SRU", Data: map[string]any{"page": "decisions"}}, "✎ documentarian proposition wiki decisions"},
		{"claim released", ev(teamstate.EventClaimReleased, nil), "○ benjamin a libéré T-SRU/SRU-142"},
		{"unknown event", teamstate.Event{Type: "unknown.event", Actor: "test", Project: "X"}, ""},
		{"wiki rejected with its reason", teamstate.Event{Type: teamstate.EventWikiRejected, Actor: "benjamin", Project: "T-SRU", Data: map[string]any{"page": "architecture", "reason": "déjà couvert"}}, "✗ benjamin proposition wiki architecture rejetée : déjà couvert"},
		{"review approved", teamstate.Event{Type: teamstate.EventReviewApproved, Actor: "reviewer", Project: "T-SRU", Ticket: "SRU-142"}, "✔ reviewer a approuvé la revue de T-SRU/SRU-142"},
		{"review rejected with reason", teamstate.Event{Type: teamstate.EventReviewRejected, Actor: "reviewer", Project: "T-SRU", Ticket: "SRU-142", Data: map[string]any{"reason": "corrections-requises"}}, "✗ reviewer a rejeté la revue de T-SRU/SRU-142 : corrections-requises"},
		{"a message", teamstate.Event{Type: teamstate.EventCustomNotification, Data: map[string]any{"message": "déploiement fini"}}, "déploiement fini"},
		{"config changed", teamstate.Event{Type: teamstate.EventConfigChanged, Actor: "alice", Project: teamstate.TeamEventsProject, Data: map[string]any{"keys": []any{"tracker.type"}}}, "✎ alice a modifié la configuration de l'équipe (tracker.type)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, FormatEvent(tt.event))
		})
	}

	i18ntest.Set(t, "en")
	assert.Equal(t, "● benjamin claimed T-SRU/SRU-142", FormatEvent(ev(teamstate.EventClaimTaken, nil)), "the text follows the language of the sender")
}

type fakePeople struct{}

func (fakePeople) Project(id string) string { return map[string]string{"api": "API"}[id] }
func (fakePeople) Member(id string) string {
	return map[string]string{"alice": "Alice M.", "bob": "Bob"}[id]
}
func (fakePeople) Handle(id string) string { return map[string]string{"alice": "alice.m"}[id] }

// hook is a webhook that records what it receives.
type hook struct {
	srv  *httptest.Server
	mu   sync.Mutex
	got  []string
	code atomic.Int32
}

func newHook(t *testing.T) *hook {
	t.Helper()
	h := &hook{}
	h.code.Store(200)
	h.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		text, _ := m["text"].(string)
		if text == "" {
			text, _ = m["content"].(string)
		}
		if code := int(h.code.Load()); code != 200 {
			w.WriteHeader(code)
			return
		}
		h.mu.Lock()
		h.got = append(h.got, text)
		h.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(h.srv.Close)
	return h
}

func (h *hook) texts() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.got...)
}

func cfgFor(n teamstate.NotificationConfig) *teamstate.TeamConfig {
	n.Enabled = true
	return &teamstate.TeamConfig{Notification: n}
}

func TestDestinationsSubscribeToTheirEvents(t *testing.T) {
	i18ntest.Set(t, "fr")
	review, claims := newHook(t), newHook(t)
	cfg := cfgFor(teamstate.NotificationConfig{Destinations: []teamstate.NotificationDestination{
		{Type: "mattermost", WebhookURL: review.srv.URL},
		{Type: "slack", WebhookURL: claims.srv.URL, Events: []string{"claim.*", "session.complete"}},
	}})
	d := NewDispatcher(cfg)
	require.True(t, d.Enabled())
	ctx := context.Background()

	// A review goes to the destination that did not choose; a claim, only to the one that asked.
	require.NoError(t, d.Dispatch(ctx, teamstate.Event{Type: teamstate.EventReviewReady, Actor: "bob", Project: "api", Ticket: "bd-7"}))
	require.NoError(t, d.Dispatch(ctx, teamstate.Event{Type: teamstate.EventClaimTaken, Actor: "bob", Project: "api", Ticket: "bd-7"}))
	require.NoError(t, d.Dispatch(ctx, teamstate.Event{Type: teamstate.EventSessionComplete, Actor: "bob", Project: "api", Ticket: "bd-7"}))
	require.NoError(t, d.Dispatch(ctx, teamstate.Event{Type: teamstate.EventConfigChanged, Actor: "bob", Project: "_team"}))

	assert.Len(t, review.texts(), 1, "default events: review and wiki, not claims or configuration")
	assert.Contains(t, review.texts()[0], "revue prête")
	assert.Len(t, claims.texts(), 2, "its own list: claims and sessions, not reviews")
}

func TestMentionsTheRecipient(t *testing.T) {
	i18ntest.Set(t, "fr")
	h := newHook(t)
	off := false
	ctx := context.Background()
	ready := teamstate.Event{Type: teamstate.EventReviewReady, Actor: "bob", Project: "api", Ticket: "bd-7", Data: map[string]any{"reviewer": "alice"}}

	d := NewDispatcher(cfgFor(teamstate.NotificationConfig{Type: "mattermost", WebhookURL: h.srv.URL}), WithPeople(fakePeople{}))
	require.NoError(t, d.Dispatch(ctx, ready))
	require.Len(t, h.texts(), 1)
	assert.True(t, strings.HasPrefix(h.texts()[0], "@alice.m "), h.texts()[0])
	assert.Contains(t, h.texts()[0], "Bob revue prête · API/bd-7", "the names, not the IDs")

	// The author of a reviewed MR, the member a claim went to.
	require.NoError(t, d.Dispatch(ctx, teamstate.Event{Type: teamstate.EventReviewRejected, Actor: "bob", Project: "api", Ticket: "bd-7", Data: map[string]any{"author": "alice"}}))
	assert.True(t, strings.HasPrefix(h.texts()[1], "@alice.m "))

	// No handle, no mention; mention = false, none either; a service that cannot mention says the name.
	require.NoError(t, d.Dispatch(ctx, teamstate.Event{Type: teamstate.EventReviewRejected, Actor: "alice", Project: "api", Ticket: "bd-7", Data: map[string]any{"author": "bob"}}))
	assert.False(t, strings.HasPrefix(h.texts()[2], "@"), "bob gave no mattermost_username")

	d = NewDispatcher(cfgFor(teamstate.NotificationConfig{Type: "mattermost", WebhookURL: h.srv.URL, Mention: &off}), WithPeople(fakePeople{}))
	require.NoError(t, d.Dispatch(ctx, ready))
	assert.False(t, strings.HasPrefix(h.texts()[3], "@"), "mention = false")

	d = NewDispatcher(cfgFor(teamstate.NotificationConfig{Type: "discord", WebhookURL: h.srv.URL}), WithPeople(fakePeople{}))
	require.NoError(t, d.Dispatch(ctx, ready))
	assert.False(t, strings.HasPrefix(h.texts()[4], "@"), "discord: no mention")
}

func TestRecipients(t *testing.T) {
	assert.Equal(t, []string{"alice"}, Recipients(teamstate.Event{Type: teamstate.EventClaimTransferred, Actor: "bob", Data: map[string]any{"to": "alice"}}))
	assert.Equal(t, []string{"alice"}, Recipients(teamstate.Event{Type: teamstate.EventClaimExpiring, Data: map[string]any{"owner": "alice"}}))
	assert.Empty(t, Recipients(teamstate.Event{Type: teamstate.EventClaimTransferred, Actor: "alice", Data: map[string]any{"to": "alice"}}), "nobody is mentioned for what they did")
	assert.Empty(t, Recipients(teamstate.Event{Type: teamstate.EventClaimTaken, Data: map[string]any{"to": "alice"}}))
}

func TestDeliveryFailureNeverCarriesTheWebhook(t *testing.T) {
	h := newHook(t)
	url := h.srv.URL + "/hooks/secret-token"
	h.srv.Close() // nothing listens any more
	err := NewMattermost(url, "", "bot").Send(context.Background(), "x")
	require.Error(t, err)
	assert.True(t, IsRetryable(err))
	assert.NotContains(t, err.Error(), "secret-token")
	assert.NotContains(t, err.Error(), url)
}

func TestOutboxTriesAgainThenGivesUp(t *testing.T) {
	i18ntest.Set(t, "fr")
	h := newHook(t)
	h.code.Store(503)
	box := NewOutbox(t.TempDir())
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	box.now = func() time.Time { return now }
	var gaveUp []string
	box.OnGiveUp = func(team, text string, err error) { gaveUp = append(gaveUp, team+"|"+text) }

	d := NewDispatcher(cfgFor(teamstate.NotificationConfig{Type: "mattermost", WebhookURL: h.srv.URL}), WithOutbox(box, "alpha"))
	rep := d.Send(context.Background(), teamstate.Event{Type: teamstate.EventReviewReady, Actor: "bob", Project: "api", Ticket: "bd-7"})
	assert.Equal(t, Report{Matched: 1, Queued: 1}, rep, "a server error is kept to be tried again, not reported")
	assert.Equal(t, 1, box.Pending())

	// Not due yet.
	assert.Equal(t, Flushed{Retry: 1}, box.Flush(context.Background()))
	// The server is back.
	h.code.Store(200)
	now = now.Add(2 * time.Minute)
	assert.Equal(t, Flushed{Sent: 1}, box.Flush(context.Background()))
	assert.Equal(t, 0, box.Pending())
	assert.Len(t, h.texts(), 1)

	// A server that never recovers: the pauses grow, then the message is given up.
	h.code.Store(500)
	require.NoError(t, box.Add(Spec{Type: "mattermost", URL: h.srv.URL}, "hello", "alpha", nil))
	for i := 0; i < len(backoff)-1; i++ {
		now = now.Add(7 * time.Hour)
		assert.Equal(t, Flushed{Retry: 1}, box.Flush(context.Background()), "retry %d", i+1)
	}
	now = now.Add(7 * time.Hour)
	assert.Equal(t, Flushed{GaveUp: 1}, box.Flush(context.Background()))
	assert.Equal(t, []string{"alpha|hello"}, gaveUp)
	assert.Equal(t, 0, box.Pending())
}

func TestARevokedWebhookIsNotRetried(t *testing.T) {
	h := newHook(t)
	h.code.Store(404)
	box := NewOutbox(t.TempDir())
	d := NewDispatcher(cfgFor(teamstate.NotificationConfig{Type: "mattermost", WebhookURL: h.srv.URL}), WithOutbox(box, "alpha"))
	rep := d.Send(context.Background(), teamstate.Event{Type: teamstate.EventReviewReady, Actor: "bob", Project: "api"})
	assert.Error(t, rep.Err)
	assert.Equal(t, 0, rep.Queued)
	assert.Equal(t, 0, box.Pending())
}

func TestDestinationsInParallel(t *testing.T) {
	// Two slow destinations do not add up their waits.
	slow := func() *httptest.Server {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(300 * time.Millisecond) }))
		t.Cleanup(s.Close)
		return s
	}
	a, b := slow(), slow()
	d := NewDispatcher(cfgFor(teamstate.NotificationConfig{Destinations: []teamstate.NotificationDestination{
		{Type: "mattermost", WebhookURL: a.URL}, {Type: "slack", WebhookURL: b.URL}}}))
	start := time.Now()
	require.NoError(t, d.Dispatch(context.Background(), teamstate.Event{Type: teamstate.EventCustomNotification, Data: map[string]any{"message": "x"}}))
	assert.Less(t, time.Since(start), 550*time.Millisecond)
}
