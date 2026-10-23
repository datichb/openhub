package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	teamsvc "github.com/datichb/openhub/cli/internal/services/team"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Where the token of the tracker is: tracker_token_key and its server
// ─────────────────────────────────────────────────────────────────────────────

// declScope says which configuration a declaration of the token is written to.
type declScope int

const (
	// declTeam: the [tracker] of the team config, for every member.
	declTeam declScope = iota
	// declLocal: the [tracker] of hub.toml, for this member only (it takes the
	// key and its server together over the team's).
	declLocal
)

// askTrackerTokenPromptKey and askTrackerTokenPromptServer are the questions
// of askTrackerTokenDecl.
func askTrackerTokenPromptKey() string {
	return i18n.T("cmd.init.wizard_tracker_decl_key")
}

func askTrackerTokenPromptServer() string {
	return i18n.T("cmd.init.wizard_tracker_decl_server")
}

// askTrackerTokenDecl asks where the token of the tracker is: its key in the
// keychain (tracker_token_key) and, for GitLab, the server it is valid for
// (tracker_token_url; empty = the tracker's). It starts from what is declared
// and changes nothing on the user's behalf: an empty answer to the key leaves
// no key (and no server), and the question says what is read then. "-" clears
// a key already declared. A server that designates nothing is refused: the
// previous one stays.
func askTrackerTokenDecl(out io.Writer, ask func(prompt, def string) string, scope declScope, trackerType, key, server string) (newKey, newServer string) {
	if scope == declLocal {
		fmt.Fprintf(out, "\n  %s\n", i18n.T("cmd.init.wizard_tracker_decl_intro_local"))
	} else {
		fmt.Fprintf(out, "\n  %s\n", i18n.T("cmd.init.wizard_tracker_decl_intro_team"))
	}
	if trackerType == string(tracker.TypeJira) {
		fmt.Fprintf(out, "  %s\n", i18n.T("cmd.init.wizard_tracker_decl_empty_jira"))
	} else {
		fmt.Fprintf(out, "  %s\n", i18n.Tf("cmd.init.wizard_tracker_decl_empty_gitlab", tracker.DefaultTokenKey(trackerType)))
	}

	newKey = strings.TrimSpace(ask(askTrackerTokenPromptKey(), key))
	if newKey == "" || newKey == "-" {
		return "", ""
	}
	if trackerType == string(tracker.TypeJira) {
		return newKey, ""
	}
	newServer = strings.TrimSpace(ask(askTrackerTokenPromptServer(), server))
	if newServer == "-" {
		return newKey, ""
	}
	if err := tracker.ValidateServer(newServer); err != nil {
		fmt.Fprintf(out, "  %s %s\n", theme.WarningStyle.Render(theme.IconWarning),
			i18n.Tf("cmd.init.wizard_tracker_decl_server_invalid", newServer))
		return newKey, server
	}
	return newKey, newServer
}

// askTeamGitLabURL asks for the GitLab that hosts the team-state repository
// when its remote goes through an SSH alias (empty = the host of the remote,
// "-" clears the one declared). An invalid answer keeps the current value.
func askTeamGitLabURL(out io.Writer, ask func(prompt, def string) string, current string) string {
	fmt.Fprintf(out, "\n  %s\n", i18n.T("cmd.init.wizard_team_gitlab_hint"))
	v := strings.TrimSpace(ask(i18n.T("cmd.init.wizard_team_gitlab_url"), current))
	if v == "-" {
		return ""
	}
	if err := tracker.ValidateServer(v); err != nil {
		fmt.Fprintf(out, "  %s %s\n", theme.WarningStyle.Render(theme.IconWarning),
			i18n.Tf("cmd.init.wizard_tracker_decl_server_invalid", v))
		return current
	}
	return v
}

// teamGitLabURLProblem is the message that explains why the GitLab typed for a
// team repository is not a server ("" when it is, or when it is empty).
func teamGitLabURLProblem(v string) string {
	if err := tracker.ValidateServer(strings.TrimSpace(v)); err != nil {
		return i18n.Tf("cmd.init.wizard_tracker_decl_server_invalid", v)
	}
	return ""
}

// setTeamGitLabURL declares the GitLab of the team-state repository of the
// team teamID in hub.toml (only that setting of the team changes).
func setTeamGitLabURL(a *app.App, teamID, url string) error {
	return updateHubConfig(a, func(c *config.Config) error {
		t := c.FindTeam(teamID)
		if t == nil {
			return fmt.Errorf("%s", i18n.Tf("tui.team.detail.team_not_in_hub", teamID))
		}
		t.GitLabURL = url
		return nil
	})
}

// trackerTokenCheck says whether a token serves the tracker of a team: the
// answer of the cascade that the sync reads, for the server of the tracker —
// not whether a secret exists under some key.
type trackerTokenCheck struct {
	// Eff is the tracker configuration in force (team config and hub.toml).
	Eff tracker.EffectiveTrackerConfig
	// Server is the server the tracker is called on.
	Server string
	// Found: a token is declared for Server.
	Found bool
}

// checkTrackerToken runs the cascade of the token of the tracker of the team
// teamID, whose config.toml is teamCfg, against the keychain of the app.
func checkTrackerToken(ctx context.Context, a *app.App, teamID string, teamCfg *teamstate.TeamConfig) trackerTokenCheck {
	eff := tracker.ResolveTrackerConfig(&teamCfg.Tracker, a.Config.Tracker)
	c := trackerTokenCheck{Eff: eff}
	if eff.Type == string(tracker.TypeJira) {
		c.Server = eff.TrackerURL
		if os.Getenv("JIRA_TOKEN") != "" {
			c.Found = true
		} else if eff.TrackerTokenKey != "" && a.Secrets != nil {
			v, err := a.Secrets.Get(ctx, eff.TrackerTokenKey)
			c.Found = err == nil && v != ""
		}
		return c
	}
	c.Server = tracker.GitLabBaseURL(eff.TrackerURL)
	keyServer, implicit := eff.TokenServer()
	res := teamService(a).GitLabToken(ctx, teamID, c.Server, teamsvc.WithTrackerToken(eff.TrackerTokenKey, keyServer, implicit))
	c.Found = res.Found()
	return c
}

// declareLocalTrackerToken writes key and server to the [tracker] of hub.toml
// when the configuration in force does not already say so, and reports whether
// it did.
func declareLocalTrackerToken(a *app.App, eff tracker.EffectiveTrackerConfig, key, server string) (bool, error) {
	if !eff.NeedsTokenDecl(key, server) {
		return false, nil
	}
	err := updateHubConfig(a, func(c *config.Config) error {
		c.Tracker.SetTokenDecl(key, server)
		return nil
	})
	return err == nil, err
}

// trackerTokenStep describes the step « personal token of the tracker » that
// oh init (new team and rejoin) and the rejoin of the TUI share.
type trackerTokenStep struct {
	ID        string
	Label     string
	InfoLabel string
	App       func() *app.App
	Ctx       func() context.Context
	// Repo is the team-state repository of the team; StatePath its clone
	// ("" = the default path of the repository).
	Repo      func() string
	StatePath func() string
	// Keep is true once the user kept the tracker of the team.
	Keep func() bool
}

func (c trackerTokenStep) teamConfig() *teamstate.TeamConfig {
	repoURL := c.Repo()
	if repoURL == "" {
		return nil
	}
	path := ""
	if c.StatePath != nil {
		path = c.StatePath()
	}
	if path == "" {
		path = config.TeamStatePath(repoURL)
	}
	cfg, err := teamstate.NewRepo(repoURL, path).LoadConfig()
	if err != nil || cfg.Tracker.Type == "" {
		return nil
	}
	return cfg
}

// buildTrackerTokenStep builds the step. It is shown when the user keeps the
// tracker of the team and no token serves the server of that tracker (the
// cascade of the sync answers, not the mere presence of a secret): it asks
// where the token is (key and server), and the token itself. The token goes to
// the keychain under that key; the key and the server go to the [tracker] of
// hub.toml when the configuration in force does not already say so.
func buildTrackerTokenStep(c trackerTokenStep) views.WizardStep {
	var (
		token, key, server     string
		proposedKey, proposedS string
		loaded, declared       bool
	)
	return views.WizardStep{
		ID:    c.ID,
		Label: c.Label,
		SkipIf: func() bool {
			a := c.App()
			if a == nil || !c.Keep() {
				return true
			}
			teamCfg := c.teamConfig()
			if teamCfg == nil {
				return true
			}
			return checkTrackerToken(c.Ctx(), a, config.RepoNameFromRemote(c.Repo()), teamCfg).Found
		},
		Validate: func() string {
			if err := tracker.ValidateServer(strings.TrimSpace(server)); err != nil {
				return i18n.Tf("cmd.init.wizard_tracker_decl_server_invalid", server)
			}
			return ""
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			a := c.App()
			teamCfg := c.teamConfig()
			if a == nil || teamCfg == nil {
				form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
				return form
			}
			check := checkTrackerToken(c.Ctx(), a, config.RepoNameFromRemote(c.Repo()), teamCfg)
			trackerType := check.Eff.Type
			if !loaded {
				proposedKey = firstNonEmpty(check.Eff.TrackerTokenKey, tracker.DefaultTokenKey(trackerType))
				proposedS = check.Eff.TrackerTokenURL
				key, server, loaded = proposedKey, proposedS, true
			}
			form.AddTextView("",
				i18n.Tf("cmd.init.wizard_tracker_token_missing_server", strings.ToUpper(trackerType), tracker.ServerOf(check.Server)),
				60, 3, true, false)
			form.AddInputField(askTrackerTokenPromptKey(), key, 0, nil, func(t string) { key = t })
			if trackerType != string(tracker.TypeJira) {
				form.AddInputField(askTrackerTokenPromptServer(), server, 0, nil, func(t string) { server = t })
			}
			form.AddPasswordField(i18n.Tf("cmd.init.wizard_tracker_token_label", strings.ToUpper(trackerType)),
				"", 0, '*', func(t string) { token = t })
			form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
			return form
		},
		OnDone: func() error {
			a := c.App()
			teamCfg := c.teamConfig()
			if a == nil || teamCfg == nil {
				return nil
			}
			eff := tracker.ResolveTrackerConfig(&teamCfg.Tracker, a.Config.Tracker)
			tokenKey := firstNonEmpty(strings.TrimSpace(key), tracker.DefaultTokenKey(eff.Type))
			tokenServer := strings.TrimSpace(server)
			if eff.Type == string(tracker.TypeJira) {
				tokenServer = ""
			}
			changed := tokenKey != proposedKey || tokenServer != proposedS
			if token != "" {
				if a.Secrets == nil {
					return fmt.Errorf(i18n.T("cmd.team_helpers.storing_tracker_token"), i18n.T("tui.views.keychain_unavailable"))
				}
				if err := a.Secrets.Set(c.Ctx(), tokenKey, token); err != nil {
					return fmt.Errorf(i18n.T("cmd.team_helpers.storing_tracker_token"), err)
				}
			}
			if token != "" || changed {
				did, err := declareLocalTrackerToken(a, eff, tokenKey, tokenServer)
				if err != nil {
					return fmt.Errorf("%s: %w", i18n.T("cmd.teams.save_error"), err)
				}
				declared = did
			}
			return nil
		},
		InfoFields: func() []views.InfoField {
			switch {
			case token != "":
				return []views.InfoField{{Label: c.InfoLabel, Value: i18n.T("cmd.init.wizard_tracker_token_stored")}}
			case declared:
				return []views.InfoField{{Label: c.InfoLabel, Value: i18n.T("cmd.init.wizard_tracker_decl_saved")}}
			}
			return []views.InfoField{{Label: c.InfoLabel, Value: i18n.T("cmd.init.wizard_team_skipped")}}
		},
	}
}
