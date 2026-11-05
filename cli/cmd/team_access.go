package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The steps every team assistant starts with: the repository of the
// team-state, a probe of the access (a read, nothing is written), and the
// access itself when the probe says it is missing. They run before anything
// is cloned and before the identity of the member is asked: a repository that
// asks for a token or a key is known before the work of the assistant.
//
// The four assistants (oh init « Créer » and « Rejoindre », the palette's
// « Initialiser l'équipe » and « Rejoindre une équipe ») build them with the
// same teamAccess, then go on with their own steps; an error of their own
// steps (the clone, the sync) offers the same buttons (ErrorActions).

// Step IDs the buttons of the error screens go back to.
const (
	stepTeamRepo   = "team_repo"
	stepTeamProbe  = "team_probe"
	stepTeamAccess = "team_access"
)

// teamAccess is the state and the steps of the repository and access part of
// a team assistant.
type teamAccess struct {
	// Ctx and App give the context and the application of the assistant when
	// a step runs (they are not known when the steps are built).
	Ctx func() context.Context
	App func() *app.App
	// Repo is the URL typed by the user; GitLabURL the GitLab of the
	// repository when its remote is an SSH alias (nil: not asked).
	Repo      *string
	GitLabURL *string
	// Active tells whether the steps apply (nil: always): a mode chosen, the
	// team not given up, no team configured yet.
	Active func() bool
	// Abandon gives the team up (« Passer l'équipe »): the assistant sets what
	// makes the steps of its team group skip. nil: nothing to set.
	Abandon func()
	// Quit makes « Passer l'équipe » a « Quitter l'assistant »: the assistant
	// is only the team (the palette), there is nothing left to go on with.
	Quit bool
	// RequiredKey is the message when the URL is empty; NextLabel the button
	// of the repository form ("" = the usual « Valider »).
	RequiredKey string
	NextLabel   string

	// Username and Token are what the user typed in the access step.
	Username, Token string
	// NeedAccess: the probe was refused for an access reason. ForceAccess: the
	// user asked for the access step after another step failed. Open: the
	// repository answers. TokenStored: a token was checked and kept in the
	// keychain of oh for it.
	NeedAccess, ForceAccess, Open, TokenStored bool
	// ProbeErr is the last refusal, for the words of the access step.
	ProbeErr error
	probed   string
}

func (a *teamAccess) ctx() context.Context {
	if a.Ctx != nil {
		if ctx := a.Ctx(); ctx != nil {
			return ctx
		}
	}
	return context.Background()
}

func (a *teamAccess) app() *app.App {
	if a.App == nil {
		return nil
	}
	return a.App()
}

func (a *teamAccess) active() bool { return a.Active == nil || a.Active() }

func (a *teamAccess) abandon() {
	if a.Abandon != nil {
		a.Abandon()
	}
}

func (a *teamAccess) repo() string { return strings.TrimSpace(*a.Repo) }

// RepoStep asks for the repository (and the GitLab of an SSH alias).
func (a *teamAccess) RepoStep() views.WizardStep {
	return views.WizardStep{
		ID:       stepTeamRepo,
		Label:    i18n.T("cmd.init.wizard_step_rejoin_repo"),
		SkipIf:   func() bool { return !a.active() },
		OnSkip:   a.abandon,
		Required: a.Quit,
		Validate: func() string {
			if a.repo() == "" {
				return i18n.T(firstNonEmpty(a.RequiredKey, "cmd.init.wizard_team_repo_required"))
			}
			if a.GitLabURL != nil {
				return teamGitLabURLProblem(*a.GitLabURL)
			}
			return ""
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			form.AddInputField(i18n.T("cmd.team.init.repo_url_title"), *a.Repo, 0, nil,
				func(text string) { *a.Repo = text })
			if a.GitLabURL != nil {
				form.AddInputField(i18n.T("cmd.init.wizard_team_gitlab_url"), *a.GitLabURL, 0, nil,
					func(text string) { *a.GitLabURL = strings.TrimSpace(text) })
				form.AddTextView("", i18n.T("cmd.init.wizard_team_gitlab_hint_short"), 60, 2, true, false)
			}
			form.AddButton(firstNonEmpty(a.NextLabel, i18n.T("wizard.hint.submit")), func() { onDone() })
			return form
		},
		InfoFields: func() []views.InfoField {
			return []views.InfoField{{Label: i18n.T("cmd.init.wizard_team_info_repo"), Value: *a.Repo}}
		},
	}
}

// ProbeStep reads the repository (git ls-remote) with what git has and the
// token oh holds for it. Success: the next steps go on. A refusal for an access
// reason is not an error: the access step is shown. Anything else (the
// network, a wrong URL) is an error with the buttons of ErrorActions.
func (a *teamAccess) ProbeStep() views.WizardStep {
	return views.WizardStep{
		ID:            stepTeamProbe,
		Label:         i18n.T("v532w.probe.label"),
		Processing:    i18n.T("v532w.probe.processing"),
		SidebarHidden: true,
		BackSkip:      true, // nothing to edit: Ctrl+B goes back to the URL
		SkipIf:        func() bool { return !a.active() },
		OnSkip:        a.abandon,
		Required:      a.Quit,
		OnDone: func() error {
			*a.Repo = a.repo()
			if a.probed != *a.Repo {
				a.Token, a.probed = "", *a.Repo // another repository: another token
			}
			a.Open, a.NeedAccess, a.ForceAccess, a.TokenStored, a.ProbeErr = false, false, false, false, nil
			err := teamstate.ProbeRemote(a.ctx(), *a.Repo)
			switch {
			case err == nil:
				a.Open = true
			case teamstate.IsAuthError(err):
				a.NeedAccess, a.ProbeErr = true, err
			default:
				return fmt.Errorf(i18n.T("v532w.probe.failed"), *a.Repo, gitCause(err))
			}
			return nil
		},
		ErrorActions: a.ErrorActions,
		InfoFields: func() []views.InfoField {
			value := i18n.T("v532w.probe.open")
			if a.NeedAccess {
				value = i18n.T("v532w.probe.needs_access")
			}
			return []views.InfoField{{Label: i18n.T("v532w.probe.field"), Value: value}}
		},
	}
}

// AccessStep sets the access up: a token for an HTTPS repository (checked,
// then kept in the keychain of oh and given to git by GIT_ASKPASS), the key of
// the agent for an SSH one. It is shown when the probe was refused for an
// access reason, or when the user asks for it after a later step failed.
func (a *teamAccess) AccessStep() views.WizardStep {
	https := func() bool { return teamstate.IsHTTPS(a.repo()) }
	return views.WizardStep{
		ID:       stepTeamAccess,
		Label:    i18n.T("v532w.access.label"),
		SkipIf:   func() bool { return !a.active() || (!a.NeedAccess && !a.ForceAccess) },
		OnSkip:   a.abandon,
		Required: a.Quit,
		Validate: func() string {
			if https() && strings.TrimSpace(a.Token) == "" {
				return i18n.T("v532w.access.token_required")
			}
			return ""
		},
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()
			if https() {
				if a.Username == "" {
					a.Username = defaultGitUser
				}
				form.AddTextView("", i18n.Tf("v532w.access.https_intro", a.repo(), a.cause()), 70, 4, true, false)
				form.AddInputField(i18n.T("cmd.init.wizard_team_cred_username"), a.Username, 0, nil,
					func(text string) { a.Username = text })
				form.AddPasswordField(i18n.T("cmd.init.wizard_team_cred_token"), a.Token, 0, '*',
					func(text string) { a.Token = text })
				hint := i18n.T("v532w.access.https_hint")
				if teamstate.IsGitLabHost(a.repo()) {
					hint += " " + strings.ReplaceAll(i18n.T("cmd.init.wizard_team_cred_gitlab_hint"), "\n", " ")
				}
				form.AddTextView("", hint, 70, 5, true, false)
				form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
				return form
			}
			form.AddTextView("", i18n.Tf("v532w.access.ssh_intro", a.repo(), a.cause()), 70, 9, true, false)
			form.AddButton(i18n.T("v532w.error.retry"), func() { onDone() })
			return form
		},
		Processing:   i18n.T("v532w.probe.processing"),
		OnDone:       a.checkAccess,
		ErrorActions: func(err error) []views.ErrorAction { return a.errorActions(err, true) },
		InfoFields: func() []views.InfoField {
			value := i18n.T("v532w.access.ssh_ok")
			if a.TokenStored {
				value = i18n.T("v532w.access.token_kept")
			}
			return []views.InfoField{{Label: i18n.T("cmd.init.wizard_team_info_auth"), Value: value}}
		},
	}
}

// checkAccess probes again with what the user gave: a token is checked before
// it is kept, so that a wrong one never reaches the keychain.
func (a *teamAccess) checkAccess() error {
	repo := a.repo()
	ctx := a.ctx()
	var err error
	if teamstate.IsHTTPS(repo) {
		user := firstNonEmpty(strings.TrimSpace(a.Username), defaultGitUser)
		token := strings.TrimSpace(a.Token)
		if err = teamstate.ProbeRemoteAs(ctx, repo, teamstate.GitCredentials{Username: user, Token: token}); err == nil {
			if saveErr := saveTeamGitToken(ctx, a.app(), repo, user, token); saveErr != nil {
				return saveErr
			}
			a.TokenStored = true
		}
	} else {
		err = teamstate.ProbeRemote(ctx, repo)
	}
	if err != nil {
		a.ProbeErr = err
		if teamstate.IsAuthError(err) {
			key := "v532w.access.refused_ssh"
			if teamstate.IsHTTPS(repo) {
				key = "v532w.access.refused_https"
			}
			return errors.New(i18n.Tf(key, repo, gitCause(err)))
		}
		return fmt.Errorf(i18n.T("v532w.probe.failed"), repo, gitCause(err))
	}
	a.Open, a.NeedAccess, a.ForceAccess, a.ProbeErr = true, false, false, nil
	return nil
}

// cause is the last refusal in a line, "" when there is none.
func (a *teamAccess) cause() string {
	if a.ProbeErr == nil {
		return i18n.T("v532w.access.cause_unknown")
	}
	return gitCause(a.ProbeErr)
}

// ErrorActions are the buttons of the error screen of a step of a team
// assistant: set the access up (first when git refused the access), retry
// (Enter), change the URL, give the team up (leave the assistant when it is
// only the team).
func (a *teamAccess) ErrorActions(err error) []views.ErrorAction { return a.errorActions(err, false) }

// errorActions is ErrorActions; the access step itself needs no button to go
// back to itself.
func (a *teamAccess) errorActions(err error, onAccess bool) []views.ErrorAction {
	var actions []views.ErrorAction
	if teamstate.IsAuthError(err) && !onAccess {
		actions = append(actions, views.ErrorAction{
			Label:    i18n.T("v532w.action.access"),
			Before:   func() { a.ForceAccess = true },
			GoToStep: stepTeamAccess,
		})
	}
	actions = append(actions,
		views.ErrorAction{Label: i18n.T("v532w.error.retry"), Retry: true},
		views.ErrorAction{Label: i18n.T("v532w.action.change_url"), GoToStep: stepTeamRepo},
	)
	if a.Quit {
		return append(actions, views.ErrorAction{Label: i18n.T("v532w.action.quit"), Quit: true})
	}
	return append(actions, views.ErrorAction{Label: i18n.T("v532w.action.skip_team"), Before: a.abandon, Skip: true})
}

// gitCause is what git said, in a line: the « fatal: » line, or the first one.
func gitCause(err error) string {
	if err == nil {
		return ""
	}
	// an access error words the fix for the user: the cause is what git said
	if authErr := teamstate.AccessError(err); authErr != nil && authErr.Cause != nil {
		err = authErr.Cause
	}
	var first string
	for _, line := range strings.Split(err.Error(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if first == "" {
			first = line
		}
		low := strings.ToLower(line)
		if strings.Contains(low, "fatal:") || strings.Contains(low, "permission denied") {
			first = line
			break
		}
	}
	if i := strings.Index(strings.ToLower(first), "fatal: "); i >= 0 {
		first = first[i+len("fatal: "):]
	}
	first = strings.TrimSuffix(first, ": exit status 128")
	if runes := []rune(first); len(runes) > 160 {
		first = string(runes[:160]) + "…"
	}
	return first
}
