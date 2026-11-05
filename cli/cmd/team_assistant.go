package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The assistant of the team of the member who sets it up, shared by
// `oh team init` and the palette. It does two different things, which used to
// be one that guessed from the hub which one it was asked for:
//
//   - « Créer une équipe »: a new repository, always, even when teams exist
//     already. The steps: the URL, the probe (a dry-run push: the right to write
//     is checked before anything is asked), the access if the probe was refused,
//     the clone, then the configuration, the card of the member, the
//     notifications and the policies.
//   - « Reconfigurer »: the team that is open. The repository is known: the probe
//     and the access come first, then the synchronisation (cloned if it never was,
//     as with `oh teams add`), then the same configuration steps.
//
// Everything that waits for the network is a step with a spinner (the
// engine runs it off the event loop); nothing is cloned, pulled or pushed
// before the first screen. hub.toml is written right after the card of the
// member is in the team-state, so that an interrupted assistant leaves no
// card in a team the hub does not know.
type teamAssistant struct {
	ctx context.Context
	a   *app.App
	// team is the team that is reconfigured; nil: a team is created.
	team *config.TeamConfig

	stateRepo, statePath, gitlabURL string
	// repo is the team-state once it is cloned (nil before).
	repo             assistantRepo
	structureWarning string

	tss    *teamStepState
	access *teamAccess
}

// newTeamAssistant makes the assistant that creates a team (team nil) or
// reconfigures one.
func newTeamAssistant(ctx context.Context, a *app.App, team *config.TeamConfig) *teamAssistant {
	w := &teamAssistant{ctx: ctx, a: a, team: team}
	w.tss = &teamStepState{Ctx: ctx, StaleDaysStr: "3", BotName: "OpenHub"}
	w.access = &teamAccess{
		Ctx:         func() context.Context { return ctx },
		App:         func() *app.App { return a },
		Repo:        &w.stateRepo,
		GitLabURL:   &w.gitlabURL,
		Quit:        true, // the assistant is only the team: giving it up is leaving it
		Write:       func() bool { return true },
		RequiredKey: "cmd.team.init.validate.repo_required",
	}
	if team != nil {
		w.stateRepo = team.StateRepo
		w.statePath = teamService(a).StatePathFor(team.StateRepo)
		w.gitlabURL = team.GitLabURL
		w.access.FixedRepo = true
		w.tss.MemberID, w.tss.OwnID = team.MemberID, team.MemberID
	} else {
		w.access.Reject = w.rejectExistingTeam
		// who the person already is in another team: the form starts with it,
		// and a taken ID is refused all the same
		w.tss.MemberID, w.tss.DisplayName, w.tss.GitLabUsername = identityDefaults(a)()
		w.tss.SuggestedID = true
	}
	w.tss.AccessActions = w.access.ErrorActions
	w.tss.OnJoined = w.saveTeam
	return w
}

// assistantRepo is what the assistant asks of the team-state it clones: the
// steps' store, the layout of the structure, the configuration (the tracker
// step).
type assistantRepo interface {
	teamStepStore
	teamstate.StructureEnsurer
}

// reconfigurable tells whether team can be reconfigured: a shared team
// (a repository to probe) that is not archived.
func reconfigurable(team config.TeamConfig) bool {
	return team.Enabled && !team.Solo && team.StateRepo != ""
}

// archivedTeam reports whether team is a shared team that was archived: the
// one thing to do is to restore it (oh teams restore), not to create or
// reconfigure anything.
func archivedTeam(team config.TeamConfig) bool {
	return team.ID != "" && !team.Enabled && !team.Solo && team.StateRepo != ""
}

// archivedTeamMessage tells to restore an archived team before reconfiguring it.
func archivedTeamMessage(team config.TeamConfig) string {
	return i18n.Tf("v54l.init.archived", team.ID, "oh teams restore "+team.ID)
}

// rejectExistingTeam refuses the repository of a team the hub already has: that
// is « Reconfigurer » (or nothing to do), not a new team.
func (w *teamAssistant) rejectExistingTeam(repo string) string {
	if t := w.a.Config.FindTeamByRepo(repo); t != nil {
		return i18n.Tf("v54a.init.team_exists", t.ID)
	}
	return ""
}

// saveTeam writes the team in hub.toml once the card of the member is in the
// team-state.
func (w *teamAssistant) saveTeam(memberID string) error {
	if t := w.a.Config.FindTeamByRepo(w.stateRepo); t != nil && t.Enabled && t.MemberID == memberID &&
		(w.gitlabURL == "" || t.GitLabURL == w.gitlabURL) {
		return nil // hub.toml says it already
	}
	if w.statePath == "" {
		w.statePath = teamService(w.a).StatePathFor(w.stateRepo)
	}
	if err := teamService(w.a).SaveTeam(w.stateRepo, w.statePath, memberID, w.gitlabURL); err != nil {
		return fmt.Errorf(i18n.T("v54a.init.writing_hub"), err)
	}
	return nil
}

// steps are the steps of the assistant, up to the policies.
func (w *teamAssistant) steps() []views.WizardStep {
	var steps []views.WizardStep
	if w.team == nil {
		steps = append(steps, w.access.RepoStep())
		steps = append(steps, w.access.HostSteps()...)
		steps = append(steps, w.access.ProbeStep(), w.access.AccessStep(), w.cloneStep())
	} else {
		steps = append(steps, w.access.HostSteps()...)
		steps = append(steps, w.access.ProbeStep(), w.access.AccessStep(), w.syncStep())
	}
	opts := teamStepOpts{}
	return append(steps,
		buildTeamConfigStep(w.tss, opts),
		buildTeamIdentityStep(w.tss, opts),
		buildTeamNotifStep(w.tss, opts),
		buildTeamPoliciesStep(w.tss, opts),
	)
}

// cloneStep clones the new team-state (the probe said it answers and accepts
// the push), lays its structure out and gives it to the steps that follow.
func (w *teamAssistant) cloneStep() views.WizardStep {
	return views.WizardStep{
		ID:           "team_clone",
		Label:        i18n.T("v532w.clone.label"),
		Required:     true,
		BackSkip:     true,
		ErrorActions: w.access.ErrorActions,
		Processing:   i18n.T("cmd.team.init.processing_repo"),
		OnDone: func() error {
			if existing, found := findExistingCloneForRemote(w.ctx, w.a, w.stateRepo); found {
				return errors.New(i18n.Tf("tui.team.clone_exists", existing.Path, existing.Source))
			}
			w.statePath = teamService(w.a).StatePathFor(w.stateRepo)
			repo := teamstate.NewRepo(w.stateRepo, w.statePath)
			if repo.IsCloned() {
				if err := repo.CheckRemote(); err != nil {
					return err
				}
				if err := repo.Pull(w.ctx); err != nil {
					if teamstate.IsAuthError(err) {
						return err
					}
					slog.Debug("team.pull.skipped", "error", err) // stale local content, usable
				}
			} else if err := repo.Clone(w.ctx); err != nil {
				return fmt.Errorf(i18n.T("cmd.team.cloning_team_state"), err)
			}
			return w.ready(repo)
		},
		InfoFields: w.repoInfo,
	}
}

// syncStep brings the team-state of the team that is reconfigured up to date:
// cloned if it never was, pulled otherwise (a pull that fails for another
// reason than the access leaves the local content, usable).
func (w *teamAssistant) syncStep() views.WizardStep {
	return views.WizardStep{
		ID:           "team_sync_state",
		Label:        i18n.T("v532w.clone.label"),
		Required:     true,
		BackSkip:     true,
		ErrorActions: w.access.ErrorActions,
		Processing:   i18n.T("v54a.init.syncing"),
		OnDone: func() error {
			repo := teamstate.NewRepo(w.stateRepo, w.statePath)
			if !repo.IsCloned() {
				if err := repo.Clone(w.ctx); err != nil {
					return fmt.Errorf(i18n.T("cmd.team.cloning_team_state"), err)
				}
			} else {
				if err := repo.CheckRemote(); err != nil {
					return err
				}
				if err := repo.Pull(w.ctx); err != nil {
					if teamstate.IsAuthError(err) {
						return err
					}
					slog.Warn("team.init.pull_failed", "error", err)
				}
			}
			return w.ready(repo)
		},
		InfoFields: w.repoInfo,
	}
}

// ready lays the structure of the team-state out and gives it to the steps
// that follow.
func (w *teamAssistant) ready(repo assistantRepo) error {
	warn, err := ensureTeamStructure(w.ctx, repo)
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.team.initializing_structure"), err)
	}
	w.structureWarning = warn
	w.repo = repo
	w.tss.useRepo(repo)
	return nil
}

func (w *teamAssistant) repoInfo() []views.InfoField {
	fields := []views.InfoField{{Label: "Repo", Value: maskRepoURL(w.stateRepo)}}
	if w.structureWarning != "" {
		fields = append(fields, views.InfoField{Label: theme.IconWarning + " " + i18n.T("cmd.team.init.structure_label"), Value: w.structureWarning})
	}
	return fields
}

// ── Duplicate clone detection ─────────────────────────────────────────────────

// existingClone holds information about an existing clone of a team-state repo.
type existingClone struct {
	// Path is the local filesystem path of the existing clone.
	Path string
	// Source describes where this clone is referenced (a team of hub.toml, or
	// a project).
	Source string
}

// findExistingCloneForRemote scans the teams of hub.toml and every registered
// project to detect whether a clone of remoteURL already exists locally. Two
// clones of the same team-state repo diverge: the assistant refuses to make a
// second one. The repository is compared as git sees it (scheme, user, .git
// suffix and case of the host do not matter).
//
// Returns the first matching clone found, or (existingClone{}, false) if none.
func findExistingCloneForRemote(ctx context.Context, a *app.App, remoteURL string) (existingClone, bool) {
	teams := append(append([]config.TeamConfig{}, a.Config.Teams...), a.Config.Team)
	for _, t := range teams {
		if t.StateRepo == "" || t.StatePath == "" || !teamstate.SameRemote(t.StateRepo, remoteURL) {
			continue
		}
		if teamstate.NewRepo(t.StateRepo, t.StatePath).IsCloned() {
			return existingClone{Path: t.StatePath, Source: i18n.Tf("v54a.clone.source_team", firstNonEmpty(t.ID, "hub.toml"))}, true
		}
	}

	if a.Projects == nil {
		return existingClone{}, false
	}
	projects, err := a.Projects.List(ctx, "")
	if err != nil {
		return existingClone{}, false
	}
	for _, p := range projects {
		resolved := config.ResolveTeamForProject(a.Config, &p)
		if !resolved.Enabled || resolved.StateRepo == "" || resolved.StatePath == "" || !teamstate.SameRemote(resolved.StateRepo, remoteURL) {
			continue
		}
		if teamstate.NewRepo(resolved.StateRepo, resolved.StatePath).IsCloned() {
			return existingClone{Path: resolved.StatePath, Source: i18n.Tf("v54a.clone.source_project", p.Name)}, true
		}
	}
	return existingClone{}, false
}
