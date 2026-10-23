package views

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// trustShell records the confirmation and the toasts.
type trustShell struct {
	mockShell
	title, detail string
	danger        bool
	onYes         func()
	toasts        []string
}

func (s *trustShell) Confirm(title, detail string, danger bool, onYes func()) {
	s.title, s.detail, s.danger, s.onYes = title, detail, danger, onYes
}
func (s *trustShell) ShowToastMsg(msg string, _ bool) { s.toasts = append(s.toasts, msg) }

func newTrustView(t *testing.T, enforced map[string]*MCPEnforcedURL) (*MCPView, *config.Config, *trustShell, *int) {
	t.Helper()
	cfg := &config.Config{}
	saves := 0
	v := NewMCPView(nil, MCPViewConfig{
		GetConfig:    func() *config.Config { return cfg },
		UpdateConfig: func(fn func(*config.Config) error) error { saves++; return fn(cfg) },
		EnforcedURLs: func() func() map[string]*MCPEnforcedURL {
			return func() map[string]*MCPEnforcedURL { return enforced }
		},
	})
	sh := &trustShell{}
	v.SetShell(sh)
	v.app = runApp(t, tview.NewFlex()) // the services are read off the loop and applied on it
	v.list = widgets.NewSectionedList()
	v.loadServices()
	v.populateList()
	v.buildCommands()
	return v, cfg, sh, &saves
}

func selectService(t *testing.T, v *MCPView, name string) {
	t.Helper()
	for i, s := range v.services {
		if s.Name == name {
			v.list.SelectIndex(i + 1) // +1 for the header
			require.Equal(t, i, v.currentServiceIndex())
			return
		}
	}
	t.Fatalf("no service %s", name)
}

// S-31: the MCP view warns about a URL the team imposes that is not
// approved, and pins it after a confirmation.
func TestMCPViewPinsTheURLOfTheTeamAfterConfirmation(t *testing.T) {
	t.Parallel()
	v, cfg, sh, saves := newTrustView(t, map[string]*MCPEnforcedURL{
		"jira": {URL: "https://jira.corp.example.com", Warning: "not approved: oh mcp trust jira"},
	})
	var withWarning []string
	for _, s := range v.services {
		if s.Enforced != nil {
			withWarning = append(withWarning, s.Name)
		}
	}
	assert.Equal(t, []string{"jira"}, withWarning)

	var ids []string
	for _, c := range v.ContextCommands() {
		ids = append(ids, c.ID)
	}
	assert.Contains(t, ids, "trust.jira")
	assert.NotContains(t, ids, "trust.gitlab")
	assert.Contains(t, v.StatusHints(), "T")

	selectService(t, v, "jira")
	v.confirmTrust("jira", "https://jira.corp.example.com", []string{"10.1.2.3  ⚠ private"}, true)
	assert.True(t, sh.danger, "a private address makes the confirmation a dangerous one")
	assert.Contains(t, sh.title, "jira")
	assert.Contains(t, sh.detail, "https://jira.corp.example.com")
	assert.Contains(t, sh.detail, "10.1.2.3")
	assert.Empty(t, cfg.MCP.Jira.TrustedURL, "nothing is pinned before the answer")
	assert.Zero(t, *saves)

	app := v.app
	sh.onYes()
	assert.Equal(t, "https://jira.corp.example.com", cfg.MCP.Jira.TrustedURL)
	assert.Equal(t, 1, *saves)
	assert.Empty(t, cfg.MCP.Gitlab.TrustedURL)
	// The services are read again off the loop; the toast comes with them.
	require.Eventually(t, func() bool {
		var done bool
		onLoop(app, func() {
			done = len(sh.toasts) > 0 && strings.Contains(sh.toasts[len(sh.toasts)-1], "https://jira.corp.example.com")
		})
		return done
	}, 2*time.Second, 10*time.Millisecond)
}

func TestMCPViewTrustWithNothingToTrust(t *testing.T) {
	t.Parallel()
	v, cfg, sh, saves := newTrustView(t, map[string]*MCPEnforcedURL{
		"jira": {Warning: "refused: https is required"}, // cannot be pinned
	})
	selectService(t, v, "gitlab")
	v.trustCurrent()
	require.Len(t, sh.toasts, 1)
	assert.Contains(t, sh.toasts[0], "gitlab", "no URL to approve")

	selectService(t, v, "jira")
	v.trustCurrent()
	require.Len(t, sh.toasts, 2)
	assert.True(t, strings.Contains(sh.toasts[1], "refused"), sh.toasts[1])
	assert.Nil(t, sh.onYes, "no confirmation for a refused URL")
	assert.Empty(t, cfg.MCP.Jira.TrustedURL)
	assert.Zero(t, *saves)
}

func TestMCPViewWithoutTeamURLs(t *testing.T) {
	t.Parallel()
	v, _, _, _ := newTrustView(t, nil)
	for _, s := range v.services {
		assert.Nil(t, s.Enforced, s.Name)
	}
}

// passwordShell answers the token modal with a fixed token.
type passwordShell struct {
	trustShell
	token string
}

func (s *passwordShell) ShowPasswordModal(_ string, onDone func(string)) { onDone(s.token) }

// memSecrets is an in-memory keychain (the real one is never used in tests).
type memSecrets struct{ m map[string]string }

func (s *memSecrets) Get(_ context.Context, k string) (string, error) { return s.m[k], nil }
func (s *memSecrets) Set(_ context.Context, k, v string) error        { s.m[k] = v; return nil }
func (s *memSecrets) Delete(_ context.Context, k string) error        { delete(s.m, k); return nil }
func (s *memSecrets) List(context.Context) ([]string, error)          { return nil, nil }

// The token typed in the MCP view is stored under the current key name, and
// the view still finds a token that only exists under the legacy name.
func TestMCPViewStoresTheTokenUnderTheCurrentKeyName(t *testing.T) {
	t.Parallel()
	store := &memSecrets{m: map[string]string{}}
	cfg := &config.Config{}
	v := NewMCPView(&app.App{Secrets: store}, MCPViewConfig{
		GetConfig:    func() *config.Config { return cfg },
		UpdateConfig: func(fn func(*config.Config) error) error { return fn(cfg) },
	})
	sh := &passwordShell{token: "s3cret"}
	v.SetShell(sh)
	v.app = runApp(t, tview.NewFlex())
	v.list = widgets.NewSectionedList()
	v.loadServices()
	v.populateList()
	v.buildCommands()

	selectService(t, v, "gitlab")
	v.promptTokenCurrent() // the keychain is written off the loop
	require.Eventually(t, func() bool {
		var done bool
		onLoop(v.app, func() { done = len(sh.toasts) > 0 })
		return done
	}, 2*time.Second, 10*time.Millisecond)

	assert.Equal(t, map[string]string{"openhub.mcp.gitlab.token": "s3cret"}, store.m)
	assert.True(t, v.checkToken("gitlab"))
	onLoop(v.app, func() {
		for _, s := range v.services {
			if s.Name == "gitlab" {
				assert.True(t, s.HasToken, "the list shows the token once it is stored")
			}
		}
	})

	store.m = map[string]string{"jira-token": "legacy"}
	assert.True(t, v.checkToken("jira"), "legacy name read as a fallback")
	assert.Equal(t, "legacy", store.m["openhub.mcp.jira.token"], "and copied")
	assert.False(t, v.checkToken("figma"))
}
