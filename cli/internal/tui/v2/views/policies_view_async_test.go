package views

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

// fakePolicyService records its calls and blocks them until release is
// closed, as a service waiting on the network would.
type fakePolicyService struct {
	mu      sync.Mutex
	calls   []string
	added   teamstate.Policy
	entered chan struct{}
	release chan struct{}
	err     error
}

func newFakePolicyService() *fakePolicyService {
	return &fakePolicyService{entered: make(chan struct{}, 8), release: make(chan struct{})}
}

func (f *fakePolicyService) wait(call string) error {
	f.mu.Lock()
	f.calls = append(f.calls, call)
	f.mu.Unlock()
	f.entered <- struct{}{}
	<-f.release
	return f.err
}

func (f *fakePolicyService) Add(_ context.Context, name string, p teamstate.Policy) error {
	f.mu.Lock()
	f.added = p
	f.mu.Unlock()
	return f.wait("add " + name)
}

func (f *fakePolicyService) Update(_ context.Context, name string, _ teamstate.Policy) error {
	return f.wait("update " + name)
}

func (f *fakePolicyService) SetEnforcement(_ context.Context, name string, _ teamstate.PolicyEnforcement) error {
	return f.wait("enforce " + name)
}

func (f *fakePolicyService) Remove(_ context.Context, name string) error {
	return f.wait("remove " + name)
}

func (f *fakePolicyService) recorded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func mountedPoliciesView(t *testing.T, fake *fakePolicyService) (*PoliciesView, *viewShell, *tview.Application) {
	t.Helper()
	frLocale(t)
	repo, tc := soloState(t)
	require.NoError(t, repo.SavePolicies(t.Context(), map[string]teamstate.Policy{
		"existing": {Type: teamstate.PolicyTypeBoolean, Enabled: true, Enforcement: teamstate.EnforcementWarn},
	}))
	v := NewPoliciesView(func() TeamResolution { return tc }, nil)
	v.SetPolicyService(func(teamstate.TeamStateWriter) PolicyService { return fake })
	sh := &viewShell{}
	v.SetShell(sh)
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	waitFor(t, app, func() bool { return len(v.policies) == 1 })
	return v, sh, app
}

// The service (pull + push) never runs on the event loop: the loop stays
// free while it is blocked, and its result is shown on the loop afterwards.
func TestPoliciesView_AddDoesNotBlockTheLoop(t *testing.T) {
	fake := newFakePolicyService()
	v, sh, app := mountedPoliciesView(t, fake)

	onLoop(app, func() { v.HandleKey(runeKey('n')) })
	require.NotNil(t, sh.form)
	// Submitting returns at once although the service is still blocked.
	onLoop(app, func() {
		sh.form.OnSubmit(map[string]string{
			"name": `say "hi"`, "type": "regex", "enforcement": "refuse",
			"rule": `^feat/`, "message": "m", "scope": "diff_only",
		}, nil)
	})
	select {
	case <-fake.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("the service was not called")
	}
	onLoop(app, func() {}) // the loop still answers
	assert.Empty(t, sh.lastToast(), "nothing shown before the service answers")

	close(fake.release)
	waitFor(t, app, func() bool { return sh.lastToast() != "" })
	assert.Contains(t, sh.lastToast(), `say "hi"`)
	assert.Equal(t, []string{`add say "hi"`}, fake.recorded())
	fake.mu.Lock()
	defer fake.mu.Unlock()
	assert.Equal(t, teamstate.PolicyTypeRegex, fake.added.Type)
	assert.Equal(t, "^feat/", fake.added.Rule)
	assert.Empty(t, fake.added.Scope, "scope belongs to forbidden_pattern only")
	assert.Equal(t, teamstate.EnforcementRefuse, fake.added.Enforcement)
}

func TestPoliciesView_ServiceErrorIsShown(t *testing.T) {
	fake := newFakePolicyService()
	fake.err = errors.New("push refusé")
	close(fake.release)
	v, sh, app := mountedPoliciesView(t, fake)

	onLoop(app, func() { v.HandleKey(runeKey('d')) })
	require.Equal(t, 1, sh.confirmCount())
	sh.confirmed(t, app)
	waitFor(t, app, func() bool { return sh.lastToast() != "" })
	assert.Contains(t, sh.lastToast(), "push refusé")
	assert.Equal(t, []string{"remove existing"}, fake.recorded())
}

func TestPoliciesView_EditCallsUpdate(t *testing.T) {
	fake := newFakePolicyService()
	close(fake.release)
	v, sh, app := mountedPoliciesView(t, fake)

	onLoop(app, func() { v.editPolicy(0) })
	require.NotNil(t, sh.form)
	onLoop(app, func() {
		sh.form.OnSubmit(map[string]string{"name": "renamed", "type": "boolean", "enforcement": "refuse"}, nil)
	})
	waitFor(t, app, func() bool { return sh.lastToast() != "" })
	assert.Equal(t, []string{"update existing"}, fake.recorded())
}
