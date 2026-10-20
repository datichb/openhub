package views

import (
	"strings"
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		SaveConfig:   func(*config.Config) error { saves++; return nil },
		EnforcedURLs: func() map[string]*MCPEnforcedURL { return enforced },
	})
	sh := &trustShell{}
	v.SetShell(sh)
	v.app = tview.NewApplication()
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

	sh.onYes()
	assert.Equal(t, "https://jira.corp.example.com", cfg.MCP.Jira.TrustedURL)
	assert.Equal(t, 1, *saves)
	assert.Empty(t, cfg.MCP.Gitlab.TrustedURL)
	require.NotEmpty(t, sh.toasts)
	assert.Contains(t, sh.toasts[len(sh.toasts)-1], "https://jira.corp.example.com")
}

func TestMCPViewTrustWithNothingToTrust(t *testing.T) {
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
	v, _, _, _ := newTrustView(t, nil)
	for _, s := range v.services {
		assert.Nil(t, s.Enforced, s.Name)
	}
}
