package views

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
)

// gatedSecrets is a keychain whose reads wait for release, as a locked
// keychain does.
type gatedSecrets struct {
	memSecrets
	reads   atomic.Int32
	release chan struct{}
}

func (s *gatedSecrets) Get(ctx context.Context, k string) (string, error) {
	s.reads.Add(1)
	<-s.release
	return s.memSecrets.Get(ctx, k)
}

// The keychain is read off the event loop: the mount returns with the loading
// placeholder while it is blocked, and the list comes once it answers.
func TestMCPViewMountDoesNotBlockOnTheKeychain(t *testing.T) {
	frLocale(t)
	store := &gatedSecrets{memSecrets: memSecrets{m: map[string]string{"openhub.mcp.gitlab.token": "x"}}, release: make(chan struct{})}
	cfg := &config.Config{}
	v := NewMCPView(&app.App{Secrets: store}, MCPViewConfig{GetConfig: func() *config.Config { return cfg }})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := runApp(t, content)

	onLoop(app, func() { v.Mount(content, app) }) // returns although the keychain is blocked
	require.Eventually(t, func() bool { return store.reads.Load() > 0 }, 2*time.Second, 5*time.Millisecond)
	onLoop(app, func() {
		assert.Nil(t, v.list, "the list waits for the data")
		assert.Equal(t, 1, content.GetItemCount(), "the loading placeholder is shown")
	})

	close(store.release)
	require.Eventually(t, func() bool {
		var ok bool
		onLoop(app, func() { ok = v.list != nil && len(v.services) == 5 })
		return ok
	}, 2*time.Second, 10*time.Millisecond)
	onLoop(app, func() {
		for _, s := range v.services {
			if s.Name == "gitlab" {
				assert.True(t, s.HasToken)
			}
		}
	})
}

// The services read by a previous mount, slow to come back, are not applied
// to the new one, nor are widgets added to the old content.
func TestMCPViewStaleServicesAfterRemountAreIgnored(t *testing.T) {
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	cfg := &config.Config{}
	v := NewMCPView(nil, MCPViewConfig{
		GetConfig: func() *config.Config { return cfg },
		EnforcedURLs: func() func() map[string]*MCPEnforcedURL {
			first := calls.Add(1) == 1
			return func() map[string]*MCPEnforcedURL {
				if first {
					close(started)
					<-release
					return map[string]*MCPEnforcedURL{"jira": {Warning: "old"}}
				}
				return map[string]*MCPEnforcedURL{"gitlab": {Warning: "new"}}
			}
		},
	})
	v.SetShell(&mockShell{})
	app := runApp(t, tview.NewFlex())

	first := tview.NewFlex()
	onLoop(app, func() { v.Mount(first, app) })
	<-started
	onLoop(app, v.Unmount)
	second := tview.NewFlex()
	onLoop(app, func() { v.Mount(second, app) })
	require.Eventually(t, func() bool {
		var ok bool
		onLoop(app, func() { ok = v.list != nil })
		return ok
	}, 2*time.Second, 10*time.Millisecond)

	close(release)
	time.Sleep(50 * time.Millisecond)
	onLoop(app, func() {
		for _, s := range v.services {
			switch s.Name {
			case "gitlab":
				assert.NotNil(t, s.Enforced, "the services of the new mount")
			case "jira":
				assert.Nil(t, s.Enforced, "not those of the old one")
			}
		}
		assert.Equal(t, 1, first.GetItemCount(), "the old content keeps its placeholder only")
	})
}

// The enforced URLs are prepared on the event loop, which owns the config,
// and read off it on that snapshot only: a config saved while the team-state
// is being read does not reach the reader (run with -race).
func TestMCPViewEnforcedURLsReadOnASnapshot(t *testing.T) {
	cfg := &config.Config{Name: "before"}
	reading, release := make(chan struct{}), make(chan struct{})
	v := NewMCPView(nil, MCPViewConfig{
		GetConfig: func() *config.Config { return cfg },
		EnforcedURLs: func() func() map[string]*MCPEnforcedURL {
			name := cfg.Name // on the loop
			return func() map[string]*MCPEnforcedURL {
				close(reading)
				<-release
				return map[string]*MCPEnforcedURL{"jira": {Warning: name}}
			}
		},
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := runApp(t, content)

	onLoop(app, func() { v.Mount(content, app) })
	<-reading
	onLoop(app, func() { cfg.Name = "after" }) // the loop saves meanwhile
	close(release)

	require.Eventually(t, func() bool {
		var ok bool
		onLoop(app, func() { ok = v.list != nil })
		return ok
	}, 2*time.Second, 10*time.Millisecond)
	onLoop(app, func() {
		for _, s := range v.services {
			if s.Name == "jira" {
				require.NotNil(t, s.Enforced)
				assert.Equal(t, "before", s.Enforced.Warning)
			}
		}
	})
}

// lockableSecrets is a keychain that can be locked: its reads and writes wait
// until it is unlocked, as a locked keychain waits for the user.
type lockableSecrets struct {
	mu   sync.Mutex
	m    map[string]string
	gate chan struct{}
}

func newLockableSecrets() *lockableSecrets { return &lockableSecrets{m: map[string]string{}} }

func (s *lockableSecrets) lock() {
	s.mu.Lock()
	s.gate = make(chan struct{})
	s.mu.Unlock()
}

func (s *lockableSecrets) unlock() {
	s.mu.Lock()
	close(s.gate)
	s.gate = nil
	s.mu.Unlock()
}

func (s *lockableSecrets) wait() {
	s.mu.Lock()
	g := s.gate
	s.mu.Unlock()
	if g != nil {
		<-g
	}
}

func (s *lockableSecrets) Get(_ context.Context, k string) (string, error) {
	s.wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[k], nil
}

func (s *lockableSecrets) Set(_ context.Context, k, v string) error {
	s.wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[k] = v
	return nil
}

func (s *lockableSecrets) Delete(_ context.Context, k string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, k)
	return nil
}
func (s *lockableSecrets) List(context.Context) ([]string, error) { return nil, nil }

// mountedMCP mounts a view on a running app and waits for its list.
func mountedMCP(t *testing.T, store *lockableSecrets, token string) (v *MCPView, loop *tview.Application, content *tview.Flex, sh *passwordShell, saves *atomic.Int32) {
	t.Helper()
	cfg := &config.Config{}
	saves = new(atomic.Int32)
	v = NewMCPView(&app.App{Secrets: store}, MCPViewConfig{
		GetConfig:  func() *config.Config { return cfg },
		SaveConfig: func(*config.Config) error { saves.Add(1); return nil },
	})
	sh = &passwordShell{token: token}
	v.SetShell(sh)
	content = tview.NewFlex()
	loop = runApp(t, content)
	onLoop(loop, func() { v.Mount(content, loop) })
	require.Eventually(t, func() bool {
		var ok bool
		onLoop(loop, func() { ok = v.list != nil })
		return ok
	}, 2*time.Second, 10*time.Millisecond)
	return v, loop, content, sh, saves
}

func toastsOf(loop *tview.Application, sh *trustShell) (out []string) {
	onLoop(loop, func() { out = append(out, sh.toasts...) })
	return
}

// Pinning a URL saves the config on the loop, then reads the services again
// off it: a keychain that waits does not freeze the view, and the toast comes
// with the new services.
func TestMCPViewPinURLDoesNotBlockOnTheKeychain(t *testing.T) {
	frLocale(t)
	store := newLockableSecrets()
	v, app, _, sh, saves := mountedMCP(t, store, "")

	store.lock()
	onLoop(app, func() { v.pinURL("jira", "https://jira.corp.example.com") }) // returns although the keychain waits
	assert.Equal(t, int32(1), saves.Load(), "the config is saved at once")
	assert.Empty(t, toastsOf(app, &sh.trustShell), "the services are being read")

	store.unlock()
	require.Eventually(t, func() bool {
		ts := toastsOf(app, &sh.trustShell)
		return len(ts) == 1 && strings.Contains(ts[0], "https://jira.corp.example.com")
	}, 2*time.Second, 10*time.Millisecond)
}

// The services read after a pin, slow to come back, do not reach a view that
// was left meanwhile.
func TestMCPViewPinURLAfterUnmountIsIgnored(t *testing.T) {
	frLocale(t)
	store := newLockableSecrets()
	v, app, _, sh, _ := mountedMCP(t, store, "")

	store.lock()
	onLoop(app, func() { v.pinURL("jira", "https://jira.corp.example.com") })
	onLoop(app, v.Unmount)
	store.unlock()
	time.Sleep(50 * time.Millisecond)
	assert.Empty(t, toastsOf(app, &sh.trustShell), "no toast from a view that is gone")
	onLoop(app, func() { assert.Nil(t, v.list) })

	// Mounted again, the view reads its own services and keeps the old
	// result out.
	second := tview.NewFlex()
	onLoop(app, func() { v.Mount(second, app) })
	require.Eventually(t, func() bool {
		var ok bool
		onLoop(app, func() { ok = v.list != nil })
		return ok
	}, 2*time.Second, 10*time.Millisecond)
	assert.Empty(t, toastsOf(app, &sh.trustShell))
}

// A token is written off the event loop: a locked keychain does not freeze the
// view, and the list shows the token once it is stored. A view left meanwhile
// shows nothing, the token is stored all the same.
func TestMCPViewStoresTheTokenOffTheLoop(t *testing.T) {
	frLocale(t)
	store := newLockableSecrets()
	v, app, _, sh, _ := mountedMCP(t, store, "s3cret")

	store.lock()
	onLoop(app, func() {
		idx := -1
		for i, s := range v.services {
			if s.Name == "gitlab" {
				idx = i
			}
		}
		require.GreaterOrEqual(t, idx, 0)
		v.list.SelectIndex(idx + 1)
		v.promptTokenCurrent() // returns although the keychain waits
	})
	assert.Empty(t, toastsOf(app, &sh.trustShell))

	store.unlock()
	require.Eventually(t, func() bool { return len(toastsOf(app, &sh.trustShell)) == 1 }, 2*time.Second, 10*time.Millisecond)
	onLoop(app, func() {
		for _, s := range v.services {
			if s.Name == "gitlab" {
				assert.True(t, s.HasToken)
			}
		}
	})
	store.mu.Lock()
	assert.Equal(t, "s3cret", store.m["openhub.mcp.gitlab.token"])
	store.mu.Unlock()
}

func TestMCPViewStoredTokenAfterUnmountIsIgnored(t *testing.T) {
	frLocale(t)
	store := newLockableSecrets()
	v, app, _, sh, _ := mountedMCP(t, store, "s3cret")

	store.lock()
	onLoop(app, func() {
		v.list.SelectIndex(2) // gitlab (figma is first)
		v.promptTokenCurrent()
	})
	onLoop(app, v.Unmount)
	store.unlock()
	time.Sleep(50 * time.Millisecond)
	assert.Empty(t, toastsOf(app, &sh.trustShell), "no toast from a view that is gone")
}

// A read that fails is told in place of the placeholder, in the words common
// to the views.
func TestMCPViewMountFailureIsShownInThePlaceholder(t *testing.T) {
	frLocale(t)
	cfg := &config.Config{}
	v := NewMCPView(nil, MCPViewConfig{
		GetConfig: func() *config.Config { return cfg },
		EnforcedURLs: func() func() map[string]*MCPEnforcedURL {
			return func() map[string]*MCPEnforcedURL { panic(errors.New("team state unreadable")) }
		},
	})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := runApp(t, content)

	onLoop(app, func() { v.Mount(content, app) })
	require.Eventually(t, func() bool {
		var text string
		onLoop(app, func() {
			if tv, ok := content.GetItem(0).(*tview.TextView); ok {
				text = tv.GetText(true)
			}
		})
		return strings.Contains(text, "Impossible de charger") && strings.Contains(text, "team state unreadable")
	}, 2*time.Second, 10*time.Millisecond)
	onLoop(app, func() { assert.Nil(t, v.list, "no list without data") })
}
