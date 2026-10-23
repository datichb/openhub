package views

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// askShell records the input and password modals the view opens, in order,
// and the toasts; answer plays the user.
type askShell struct {
	mockShell
	modals []askedModal
	toasts []string
}

type askedModal struct {
	kind, title, def string
	fn               func(string)
}

func (s *askShell) ShowInputModal(title, def string, fn func(string)) {
	s.modals = append(s.modals, askedModal{"input", title, def, fn})
}
func (s *askShell) ShowPasswordModal(title string, fn func(string)) {
	s.modals = append(s.modals, askedModal{"password", title, "", fn})
}
func (s *askShell) ShowToastMsg(msg string, _ bool) { s.toasts = append(s.toasts, msg) }

func (s *askShell) lastToast() string {
	if len(s.toasts) == 0 {
		return ""
	}
	return s.toasts[len(s.toasts)-1]
}

// answer plays the modals that are open: each input modal gets the next of
// inputs (nil = keep the value it proposes), the password modal gets token.
func (s *askShell) answer(token string, inputs ...*string) {
	for len(s.modals) > 0 {
		m := s.modals[0]
		s.modals = s.modals[1:]
		switch m.kind {
		case "input":
			v := m.def
			if len(inputs) > 0 {
				if inputs[0] != nil {
					v = *inputs[0]
				}
				inputs = inputs[1:]
			}
			m.fn(v)
		default:
			m.fn(token)
		}
	}
}

func ptr(s string) *string { return &s }

// tokenEnv is a team detail on a hub.toml in a temporary home, a keychain in
// memory and a team-state config the test sets.
type tokenEnv struct {
	v       *TeamDetailView
	shared  *config.Config
	sh      *askShell
	secrets map[string]string
	saved   *teamstate.TeamConfig
}

func newTokenEnv(t *testing.T, trackerType string) *tokenEnv {
	t.Helper()
	frLocale(t)
	shared := seedHome(t)
	e := &tokenEnv{shared: shared, sh: &askShell{}, secrets: map[string]string{}}
	tc := TeamResolution{Enabled: true, TeamID: "core", StatePath: t.TempDir(), StateRepo: "git@gitlab.corp.example:acme/core.git"}
	require.NoError(t, config.Update(func(c *config.Config) error {
		c.Teams = []config.TeamConfig{{ID: "core", Enabled: true, StateRepo: tc.StateRepo, StatePath: tc.StatePath, MemberID: "alice"}}
		return nil
	}))
	fresh, err := config.Load()
	require.NoError(t, err)
	*shared = *fresh
	e.v = NewTeamDetailView(TeamDetailViewConfig{
		GetMCPConfig:          func() config.MCPConfig { return shared.MCP },
		GetTrackerLocalConfig: func() config.TrackerLocalConfig { return shared.Tracker },
		ResolveTeam: func() TeamResolution {
			tc.GitLabURL = shared.FindTeam("core").GitLabURL
			return tc
		},
		GetSecrets:      func() tracker.SecretGetter { return nil },
		GetHubConfig:    func() *config.Config { return shared },
		UpdateHubConfig: hubHost{shared}.update,
		SetSecret: func(_ context.Context, key, value string) error {
			if strings.HasPrefix(key, "broken") {
				return errors.New("keychain locked")
			}
			e.secrets[key] = value
			return nil
		},
		SaveTeamConfig: func(_ context.Context, cfg *teamstate.TeamConfig) error {
			c := *cfg
			e.saved = &c
			return nil
		},
	})
	e.v.SetShell(e.sh)
	e.v.loadData()
	e.v.teamCfg.Tracker.Type = trackerType
	e.v.buildFields()
	return e
}

func (e *tokenEnv) field(t *testing.T, key string) configField {
	t.Helper()
	f, ok := fieldByKey(e.v, key)
	require.True(t, ok, "no field %q", key)
	return f
}

// shown is the line of a field as the list draws it.
func (e *tokenEnv) shown(t *testing.T, key string) string {
	t.Helper()
	return renderConfigItem(e.field(t, key), 28).MainText
}

func (e *tokenEnv) hub(t *testing.T) *config.Config {
	t.Helper()
	got, err := config.Read()
	require.NoError(t, err)
	return got
}

// Bug 4: the field showed openhub.tracker.gitlab.token as soon as the type of
// the tracker was set, whatever was configured.
func TestTeamDetailTokenFieldShowsWhatIsConfigured(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	notConfigured := i18n.T("tui.config.not_configured")

	assert.Contains(t, e.shown(t, "tracker_token"), notConfigured)
	assert.NotContains(t, e.shown(t, "tracker_token"), "openhub.tracker", "a default computed is not what is configured")
	assert.Equal(t, ScopeTeamLocal, e.field(t, "tracker_token").Scope, "the key is declared in hub.toml: local")

	e.v.teamCfg.Tracker.TrackerURL = "https://gitlab.forge.example"
	e.v.localTrk.SetTokenDecl("forge.token", "")
	e.v.buildFields()
	line := e.shown(t, "tracker_token")
	assert.Contains(t, line, "forge.token")
	assert.Contains(t, line, "implicite : serveur du tracker", "no server declared: the key is for the server of the tracker")

	e.v.localTrk.SetTokenDecl("forge.token", "https://gitlab.other.example")
	e.v.buildFields()
	line = e.shown(t, "tracker_token")
	assert.Contains(t, line, "gitlab.other.example")
	assert.NotContains(t, line, "implicite")
	assert.Contains(t, e.field(t, "tracker_token").Source(), "local")

	e.v.localTrk = config.TrackerLocalConfig{}
	e.v.teamCfg.Tracker.TrackerTokenKey = "team.token"
	e.v.buildFields()
	assert.Contains(t, e.shown(t, "tracker_token"), "team.token")
	assert.Contains(t, e.field(t, "tracker_token").Source(), "équipe")
}

// Bug 4: the token ended under the key of the MCP of the hub, enabled
// mcp.gitlab, and never wrote tracker_token_key.
func TestTeamDetailTokenSetupWritesTrackerTokenKey(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	e.v.teamCfg.Tracker.TrackerURL = "https://gitlab.forge.example"

	e.v.promptTokenSetup()
	e.sh.answer("glpat-secret", ptr("forge.token"), ptr("https://gitlab.forge.example"))

	got := e.hub(t)
	assert.Equal(t, "forge.token", got.Tracker.TrackerTokenKey)
	assert.Equal(t, "https://gitlab.forge.example", got.Tracker.TrackerTokenURL)
	assert.Equal(t, "glpat-secret", e.secrets["forge.token"], "the token is under the key that is declared")
	assert.False(t, got.MCP.Gitlab.Enabled, "the token of the tracker does not turn the MCP GitLab on")
	assert.Empty(t, got.MCP.Gitlab.Token, "nor does it use the key of the MCP")
	assert.Empty(t, e.v.teamCfg.Tracker.TrackerTokenKey, "it is a local setting, the team config is untouched")
	assert.False(t, e.v.dirtyTeam)
	assert.False(t, e.v.dirtyLocal, "written at once: nothing is waiting for w")
	assert.Equal(t, "forge.token", e.shared.Tracker.TrackerTokenKey, "the shared config is up to date")
	assert.Contains(t, e.shown(t, "tracker_token"), "forge.token")
}

// The default key is read as the former key of the tracker for the server of
// the tracker: keeping it declares nothing. Jira never reads it: declared.
func TestTeamDetailTokenSetupKeepingTheDefaultKey(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	e.v.promptTokenSetup()
	require.Len(t, e.sh.modals, 1)
	assert.Equal(t, "openhub.tracker.gitlab.token", e.sh.modals[0].def, "the key proposed is the default one, said as a proposal")
	e.sh.answer("tok")

	got := e.hub(t)
	assert.Empty(t, got.Tracker.TrackerTokenKey)
	assert.Equal(t, "tok", e.secrets["openhub.tracker.gitlab.token"])

	j := newTokenEnv(t, "jira")
	j.v.promptTokenSetup()
	require.Len(t, j.sh.modals, 1)
	j.sh.answer("jira-tok")
	assert.Equal(t, "openhub.tracker.jira.token", j.hub(t).Tracker.TrackerTokenKey, "Jira only reads a declared key")
	assert.Equal(t, "jira-tok", j.secrets["openhub.tracker.jira.token"])
	assert.Empty(t, j.hub(t).Tracker.TrackerTokenURL, "no server rule for Jira")
}

func TestTeamDetailTokenSetupRejectsWhatCannotBeKept(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	e.v.promptTokenSetup()
	e.sh.answer("tok", ptr("forge.token"), ptr("not a server"))
	assert.Empty(t, e.hub(t).Tracker.TrackerTokenKey, "an invalid server is refused before anything is written")
	assert.Empty(t, e.secrets)
	assert.Contains(t, e.sh.lastToast(), "not a server")

	e = newTokenEnv(t, "gitlab")
	e.v.promptTokenSetup()
	e.sh.answer("tok", ptr("broken.token"), ptr(""))
	assert.Empty(t, e.hub(t).Tracker.TrackerTokenKey, "the key is not declared when the token could not be stored")
	assert.Contains(t, e.sh.lastToast(), "keychain locked")

	// no token typed: the declaration alone is written (the token is already in the keychain)
	e = newTokenEnv(t, "gitlab")
	e.v.promptTokenSetup()
	e.sh.answer("", ptr("forge.token"), ptr("https://gitlab.forge.example"))
	assert.Equal(t, "forge.token", e.hub(t).Tracker.TrackerTokenKey)
	assert.Empty(t, e.secrets)
}

// Undo must not bring back a declaration that was written to hub.toml.
func TestTeamDetailUndoKeepsTheDeclaredKey(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	e.v.pushUndo()
	e.v.teamCfg.Tracker.TrackerURL = "https://gitlab.forge.example"
	e.v.dirtyTeam = true
	e.v.promptTokenSetup()
	e.sh.answer("tok", ptr("forge.token"), ptr(""))

	e.v.undo()
	assert.Equal(t, "forge.token", e.v.localTrk.TrackerTokenKey)
	e.v.dirtyLocal = true
	e.v.save()
	assert.Equal(t, "forge.token", e.hub(t).Tracker.TrackerTokenKey, "saving after an undo does not revert the declaration")
}

// The team declares the key and its server for everybody (config.toml); the
// server is checked, and an empty key clears both.
func TestTeamDetailSharedTokenDeclaration(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	key, srv := e.field(t, "tracker_token_key"), e.field(t, "tracker_token_url")
	assert.Equal(t, ScopeTeamShared, key.Scope)
	assert.Equal(t, ScopeTeamShared, srv.Scope)

	require.Error(t, srv.Validator.Validate("not a server"))
	require.NoError(t, srv.Validator.Validate("https://gitlab.forge.example"))
	require.NoError(t, srv.Validator.Validate(""))

	key.Set("team.token")
	srv.Set("https://gitlab.forge.example")
	assert.True(t, e.v.dirtyTeam)
	e.v.save()
	require.NotNil(t, e.saved)
	assert.Equal(t, "team.token", e.saved.Tracker.TrackerTokenKey)
	assert.Equal(t, "https://gitlab.forge.example", e.saved.Tracker.TrackerTokenURL)

	key.Set("")
	assert.Empty(t, e.v.teamCfg.Tracker.TrackerTokenURL, "a server without a key means nothing")
}

// Reste de B2a: the lease and the heartbeat of the claims, with the rules of
// ClaimConfig.Validate.
func TestTeamDetailClaimSettings(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	e.v.teamCfg.Claim = teamstate.ClaimConfig{LeaseHours: 24, HeartbeatMinutes: 60}
	e.v.buildFields()
	lease, hb := e.field(t, "lease_hours"), e.field(t, "heartbeat_minutes")
	assert.Equal(t, ScopeTeamShared, lease.Scope)
	assert.Equal(t, "24", lease.Get())
	assert.Equal(t, "60", hb.Get())

	require.NoError(t, lease.Validator.Validate("48"))
	require.Error(t, lease.Validator.Validate("0"), "0 hour is below the bounds")
	require.Error(t, lease.Validator.Validate("1000"))
	require.Error(t, lease.Validator.Validate("1"), "a 1 hour lease leaves 30 minutes of heartbeat at most, 60 is set")
	require.NoError(t, hb.Validator.Validate("0"), "0 disables the heartbeat")
	require.Error(t, hb.Validator.Validate("3"))
	require.Error(t, hb.Validator.Validate("2000"))
	require.Error(t, hb.Validator.Validate("721"), "more than half of 24 hours")

	lease.Set("48")
	hb.Set("120")
	assert.Equal(t, 48, e.v.teamCfg.Claim.LeaseHours)
	assert.Equal(t, 120, e.v.teamCfg.Claim.HeartbeatMinutes)
	assert.True(t, e.v.dirtyTeam)
	e.v.save()
	require.NotNil(t, e.saved)
	assert.Equal(t, 48, e.saved.Claim.LeaseHours)
	assert.Equal(t, 120, e.saved.Claim.HeartbeatMinutes)
	require.NoError(t, e.saved.Claim.Validate())
}

// Point 3 of K2: the real GitLab of the team-state repository, local to the
// member (an SSH alias is a name of ~/.ssh/config).
func TestTeamDetailTeamGitLabURL(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	f := e.field(t, "team_gitlab_url")
	assert.Equal(t, ScopeTeamLocal, f.Scope)
	assert.Empty(t, f.Get())
	require.Error(t, f.Validator.Validate("not a server"))

	f.Set("https://gitlab.corp.example")
	assert.Equal(t, "https://gitlab.corp.example", e.hub(t).FindTeam("core").GitLabURL, "written to the team of hub.toml")
	assert.Equal(t, "https://gitlab.corp.example", e.field(t, "team_gitlab_url").Get(), "read back from the shared config")
	assert.Equal(t, "alice", e.hub(t).FindTeam("core").MemberID, "the other settings of the team are kept")

	f.Set("")
	assert.Empty(t, e.hub(t).FindTeam("core").GitLabURL)
}

// What the list draws for the tracker lines: the declared key with its server
// and its level, never a key nobody declared.
func TestTeamDetailDrawsTheTokenLines(t *testing.T) {
	e := newTokenEnv(t, "gitlab")
	require.NoError(t, e.v.cfg.UpdateHubConfig(func(c *config.Config) error {
		c.Tracker.SetTokenDecl("forge.token", "https://gitlab.forge.example")
		return nil
	}))
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { e.v.Mount(content, app) })
	waitFor(t, app, func() bool { return e.v.list != nil && len(e.v.list.GetItems()) > 0 })

	screen := drawn(t, app, content, 140, 60)
	assert.Contains(t, screen, "Jeton du tracker")
	assert.Contains(t, screen, "forge.token → gitlab.forge.example")
	assert.Contains(t, screen, "(local)")
	assert.NotContains(t, screen, "openhub.tracker.gitlab.token")
	var lines []string
	onLoop(app, func() {
		for _, it := range e.v.list.GetItems() {
			lines = append(lines, it.MainText)
		}
	})
	assert.Contains(t, strings.Join(lines, "\n"), "Durée du bail (heures)")
}
