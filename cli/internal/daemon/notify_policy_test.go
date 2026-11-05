package daemon

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/daemon/api"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/notifypolicy"
)

// livePolicy is a policy the test changes while the notifier reads it.
type livePolicy struct {
	mu sync.Mutex
	s  notifypolicy.Settings
}

func (l *livePolicy) get() notifypolicy.Settings { l.mu.Lock(); defer l.mu.Unlock(); return l.s }
func (l *livePolicy) set(fn func(*notifypolicy.Settings)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fn(&l.s)
}

func policyNotifier(t *testing.T, p *livePolicy, sent *notes, store domain.DecisionStore) *notifier {
	t.Helper()
	d := &Daemon{opts: Options{Decisions: store, NotifyPolicy: p.get}, clients: map[string]client{}}
	n := newNotifier(d, sent.add)
	n.window = 10 * time.Millisecond
	n.primed = true
	return n
}

func TestPolicyFiltersAndSilencesTheSystemNotifications(t *testing.T) {
	var sent notes
	p := &livePolicy{}
	n := policyNotifier(t, p, &sent, nil)
	wait := func(count int) {
		t.Helper()
		require.Eventually(t, func() bool { return len(sent.list()) == count }, time.Second, 5*time.Millisecond)
	}

	n.turnDone("a") // step_done is a toast by default: no system notification
	assert.Never(t, func() bool { return len(sent.list()) > 0 }, 100*time.Millisecond, 10*time.Millisecond)

	n.sessionComplete("a")
	wait(1)
	assert.Equal(t, []string{"oh-complete"}, sent.groups)
	assert.Equal(t, []bool{true}, sent.sounds)

	p.set(func(s *notifypolicy.Settings) { s.Sound = new(bool) }) // hot reload: the sound goes off at the next notification
	n.sessionComplete("b")
	wait(2)
	assert.False(t, sent.sounds[1])

	p.set(func(s *notifypolicy.Settings) { s.Events = map[string]string{"session_complete": "toast"} })
	n.sessionComplete("c")
	assert.Never(t, func() bool { return len(sent.list()) > 2 }, 100*time.Millisecond, 10*time.Millisecond, "a kind set to toast sends no system notification")

	p.set(func(s *notifypolicy.Settings) { s.Events, s.QuietHours = nil, "00:00-23:59" })
	n.sessionComplete("d")
	assert.Never(t, func() bool { return len(sent.list()) > 2 }, 100*time.Millisecond, 10*time.Millisecond, "quiet hours")

	off := false
	p.set(func(s *notifypolicy.Settings) { s.QuietHours, s.System = "", &off })
	n.sessionComplete("e")
	assert.Never(t, func() bool { return len(sent.list()) > 2 }, 100*time.Millisecond, 10*time.Millisecond, "system notifications off")
}

func TestAWorkflowEndIsNotAnnouncedAsAStepToo(t *testing.T) {
	var sent notes
	p := &livePolicy{s: notifypolicy.Settings{Events: map[string]string{"step_done": "system"}}}
	n := policyNotifier(t, p, &sent, nil)
	n.sessionComplete("a")
	n.turnDone("a") // the same turn ended the workflow
	require.Eventually(t, func() bool { return len(sent.list()) == 1 }, time.Second, 5*time.Millisecond)
	assert.Never(t, func() bool { return len(sent.list()) > 1 }, 100*time.Millisecond, 10*time.Millisecond)
	assert.Equal(t, []string{"oh-complete"}, sent.groups)
}

func TestAttachedAndOpenTUIAreRespected(t *testing.T) {
	var sent notes
	p := &livePolicy{}
	n := policyNotifier(t, p, &sent, nil)
	fs := &fakeSessions{s: domain.Session{ID: "a", GroupKey: "g1", ProjectID: "p"}}
	n.d.opts.Sessions = fs
	n.d.mu.Lock()
	n.d.clients["c1"] = client{kind: api.ClientAttach, group: "g1", expires: time.Now().Add(time.Hour)}
	n.d.mu.Unlock()

	n.sessionComplete("a")
	assert.Never(t, func() bool { return len(sent.list()) > 0 }, 100*time.Millisecond, 10*time.Millisecond, "someone is attached: they are looking at it")

	p.set(func(s *notifypolicy.Settings) { s.WhenAttached = true })
	n.sessionComplete("a")
	require.Eventually(t, func() bool { return len(sent.list()) == 1 }, time.Second, 5*time.Millisecond)

	n.d.mu.Lock()
	delete(n.d.clients, "c1")
	n.d.clients["tui"] = client{kind: api.ClientPresence, expires: time.Now().Add(time.Hour)}
	n.d.mu.Unlock()
	no := false
	p.set(func(s *notifypolicy.Settings) { s.WhenTUIFocused = &no })
	n.sessionComplete("a")
	assert.Never(t, func() bool { return len(sent.list()) > 1 }, 100*time.Millisecond, 10*time.Millisecond, "the TUI is open and its toast is enough")
	p.set(func(s *notifypolicy.Settings) { s.WhenTUIFocused = nil })
	n.sessionComplete("a")
	require.Eventually(t, func() bool { return len(sent.list()) == 2 }, time.Second, 5*time.Millisecond)
}

func TestDecisionNotesAreToldOncePerKey(t *testing.T) {
	var sent notes
	p := &livePolicy{}
	n := policyNotifier(t, p, &sent, nil)
	alert := func(session, ref string) domain.Decision {
		return domain.Decision{ID: "error:" + session + ":" + ref, SessionID: session, Kind: domain.DecisionError, ToolRef: ref,
			Payload: domain.DecisionPayload{Title: "Clé Bedrock refusée par AWS", Message: "ligne 1\nligne 2",
				Data: map[string]any{dataOhAlert: "provider_auth"}}}
	}
	nt, ok := n.noteOf(alert("s1", "provider_auth:k1"))
	require.True(t, ok)
	assert.Equal(t, notifypolicy.KindCredential, nt.kind)
	assert.Equal(t, "Clé Bedrock refusée par AWS", nt.title, "the cause is the title")
	assert.Equal(t, "ligne 1", nt.message)
	_, ok = n.noteOf(alert("s2", "provider_auth:k1"))
	assert.False(t, ok, "the same key on another session is told once")
	_, ok = n.noteOf(alert("s2", "provider_auth:k2"))
	assert.True(t, ok, "another key is another note")

	// A host alert of the daemon (team_host) is one too.
	host := alert("s1", "team_host:abc")
	host.Payload.Title = "Serveur SSH inconnu"
	nt, ok = n.noteOf(host)
	assert.True(t, ok)
	assert.Equal(t, "Serveur SSH inconnu", nt.title)

	budget := domain.Decision{ID: "budget:s1:x", SessionID: "s1", Kind: domain.DecisionBudget}
	_, ok = n.noteOf(budget)
	assert.True(t, ok)
	_, ok = n.noteOf(budget)
	assert.False(t, ok, "a budget asked again after each step is not notified again")

	failure := domain.Decision{ID: "error:s1:1", SessionID: "s1", Kind: domain.DecisionError}
	_, ok = n.noteOf(failure)
	assert.True(t, ok)
	failure.ID = "error:s1:2"
	_, ok = n.noteOf(failure)
	assert.False(t, ok, "one failure notification per session in 10 minutes")
	n.mu.Lock()
	n.now = func() time.Time { return time.Now().Add(11 * time.Minute) }
	n.mu.Unlock()
	_, ok = n.noteOf(failure)
	assert.True(t, ok)
}

func TestAlertsAreSeparateNotifications(t *testing.T) {
	var sent notes
	p := &livePolicy{}
	n := policyNotifier(t, p, &sent, nil)
	n.mu.Lock()
	n.enqueueLocked(note{kind: notifypolicy.KindCredential, class: classAlert + ":k1", title: "Clé A refusée", message: "m1"})
	n.enqueueLocked(note{kind: notifypolicy.KindCredential, class: classAlert + ":k2", title: "Jeton git expiré", message: "m2"})
	n.mu.Unlock()
	require.Eventually(t, func() bool { return len(sent.list()) == 2 }, time.Second, 5*time.Millisecond)
	assert.Equal(t, [][2]string{{"Clé A refusée", "m1"}, {"Jeton git expiré", "m2"}}, sent.list())
	assert.Equal(t, []string{"oh-alert", "oh-alert"}, sent.groups)
}

// fakeSessions serves one session.
type fakeSessions struct {
	domain.SessionStore
	s domain.Session
}

func (f *fakeSessions) Get(context.Context, string) (*domain.Session, error) {
	s := f.s
	return &s, nil
}
