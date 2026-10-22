package views

import (
	"context"
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
