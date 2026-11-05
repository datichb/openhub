package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// teamBoardRepo is what the team board asks of the team-state: the tickets
// (claims and members) and their configuration to show, the claims to change,
// the pull to refresh.
type teamBoardRepo interface {
	teamstate.RepoInfo
	teamstate.RepoSync
	teamstate.BoardStatusSetter
	teamstate.ConfigReader
	teamstate.MemberReader
	teamstate.ClaimReader
	teamstate.ClaimWriter
}

// buildTeamBoardViewConfig builds the full TeamBoardViewConfig for the shell TUI,
// wiring the refresh, sync and action callbacks to the live team-state repo.
func buildTeamBoardViewConfig(a *app.App) views.TeamBoardViewConfig {
	// resolveRepo is a helper that returns the active team repo (or nil).
	// Resolution order (ADR-032):
	//   1. Project-level team config (TeamID or legacy TeamConfig)
	//   2. Hub-level ActiveTeam() — matches CLI sync-tracker behavior
	// This ensures tickets synced by the CLI are visible in the TUI.
	resolveRepo := func() teamBoardRepo {
		// Try project-level first
		project, _ := resolveActiveProject(a)
		tc := resolvedTeamConfig(a, project)
		if tc.Enabled && tc.StateRepo != "" {
			repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
			if repo.IsCloned() {
				return repo
			}
		}
		// Fallback: hub-level ActiveTeam() — same path as CLI sync-tracker
		hubTC := a.Config.ActiveTeam()
		if !hubTC.Enabled || hubTC.StateRepo == "" {
			return nil
		}
		repo := teamstate.NewRepo(hubTC.StateRepo, hubTC.StatePath)
		if !repo.IsCloned() {
			return nil
		}
		return repo
	}

	// The names of the projects are read again with each read of the tickets
	// (readNames): a project added or renamed shows at the next refresh.

	// Load tickets from the local clone immediately — no network call.
	// The async SyncFunc will refresh after pulling remote changes.
	// Use a recover guard for safety (resolveRepo may panic with minimal test fixtures).

	// The columns of the board and the mapping of the labels come from the
	// team config; they are read again with each read of the tickets (and by
	// the column editor), so that the board follows an edit without restarting.
	var layout atomic.Pointer[views.BoardLayout]
	layout.Store(&views.BoardLayout{})
	readLayout := func(repo teamBoardRepo) views.BoardLayout {
		l := *layout.Load()
		if cfg, err := repo.LoadConfig(tuiContext()); err == nil && cfg != nil {
			l = views.BoardLayout{Board: cfg.Board, LabelStatusMapping: cfg.Tracker.LabelStatusMapping}
			layout.Store(&l)
		}
		return l
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				safego.LogRecovered("board config init panic", r)
			}
		}()
		if repo := resolveRepo(); repo != nil {
			readLayout(repo)
		}
	}()

	// Wrap resolveRepo to propagate board statuses for custom column validation.
	resolveRepoWithBoard := func() teamBoardRepo {
		repo := resolveRepo()
		if repo != nil {
			repo.SetBoardStatuses(layout.Load().Board.AllStatuses())
		}
		return repo
	}

	var initialTickets []views.TeamTicket
	func() {
		defer func() {
			if r := recover(); r != nil {
				safego.LogRecovered("board tickets init panic", r)
			}
		}()
		if repo := resolveRepo(); repo != nil {
			ctx := tuiContext()
			initialTickets = views.FetchTeamTicketsWithColumns(ctx, repo, readNames(ctx, a, nil).Project, views.ColumnsFromConfig(layout.Load().Board))
		}
	}()

	return views.TeamBoardViewConfig{
		Tickets:            initialTickets,
		MemberID:           func() string { return a.Config.ActiveTeam().MemberID },
		RefreshRate:        5 * time.Second,
		LabelStatusMapping: layout.Load().LabelStatusMapping,
		BoardConfig:        layout.Load().Board,
		Layout:             func() views.BoardLayout { return *layout.Load() },
		OnBoardConfig:      actionBoardColumnConfig,
		IsConfigured: func() bool {
			return resolveRepo() != nil
		},
		BeadsSummaryFunc: buildBeadsSummaryFunc(a),
		RefreshFunc: func() []views.TeamTicket {
			repo := resolveRepo()
			if repo == nil {
				return nil
			}
			l := readLayout(repo)
			repo.SetBoardStatuses(l.Board.AllStatuses())
			ctx := tuiContext()
			return views.FetchTeamTicketsWithColumns(ctx, repo, readNames(ctx, a, nil).Project, views.ColumnsFromConfig(l.Board))
		},
		SyncFunc: func() error {
			// Git pull only — fetches colleagues' changes from the shared state repo.
			// Periodic: skipped when the clone is busy.
			ctx, cancel := context.WithTimeout(teamstate.WithTryLock(context.Background()), 15*time.Second)
			defer cancel()
			repo := resolveRepo()
			if repo == nil {
				return nil
			}
			err := repo.Pull(ctx)
			if errors.Is(err, teamstate.ErrBusy) {
				return nil
			}
			if (err == nil || teamstate.IsPullWarning(err)) && tuiShell != nil {
				views.AfterTeamSync(tuiShell.Context(), repo.Path())
			}
			return err
		},
		TrackerSyncFunc: func() error {
			// Full tracker API sync (GitLab/Jira) — called on manual 'r' key only.
			// Recover guard: runSyncTrackerForTUI may panic if app state is
			// partially initialized (e.g. Projects store not loaded yet).
			var err error
			func() {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf(i18n.T("cmd.tui_team_board.tracker_sync_panic"), r)
						safego.LogRecovered("tracker_sync.panic", r)
					}
				}()
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				_, err = runSyncTrackerForTUI(a, ctx)
			}()
			return err
		},
		OpenSyncFunc: func() (summary string, err error) {
			// Same guard as the manual sync: a panic of a half-initialized app
			// must not take the board down.
			defer func() {
				if r := recover(); r != nil {
					summary, err = "", fmt.Errorf(i18n.T("cmd.tui_team_board.tracker_sync_panic"), r)
					safego.LogRecovered("tracker_sync.panic", r)
				}
			}()
			ctx, cancel := context.WithTimeout(tuiContext(), tracker.RunTimeout)
			defer cancel()
			return autoSyncForTUI(a, ctx)
		},
		Actions: &views.BoardActions{
			Members: func() []views.SelectOption {
				repo := resolveRepo()
				if repo == nil {
					return nil
				}
				members, err := repo.ListMembers(tuiContext())
				if err != nil {
					return nil
				}
				opts := make([]views.SelectOption, 0, len(members))
				for _, m := range members {
					opts = append(opts, views.SelectOption{
						Label: m.DisplayName,
						Value: m.ID,
					})
				}
				return opts
			},
			FetchDetail: func(project, ticketID string) (string, string, error) {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				engine := resolveTrackerEngine(ctx, a)
				if engine == nil {
					return "", "", errors.New(i18n.T("tui.team_board.tracker_not_configured"))
				}
				return engine.FetchTicketDetail(ctx, project, ticketID)
			},
			OnClaim: func(cardProject, ticketID string) error {
				repo := resolveRepoWithBoard()
				if repo == nil {
					return errors.New(i18n.T("tui.team_board.team_not_configured"))
				}
				projectID := boardProjectID(a, cardProject)
				memberID := a.Config.ActiveTeam().MemberID
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()

				// A claim someone holds is left as it is; any other one is one
				// more ticket in progress for the member: refused past
				// max_in_progress_per_member.
				if held, gerr := repo.GetClaim(ctx, projectID, ticketID); gerr == nil && held.ClaimedBy != "" {
					return nil // already owned, idempotent
				}
				if err := checkInProgressRoom(ctx, repo, a.Config.Tracker, memberID, teamstate.ClaimRef{Project: projectID, TicketID: ticketID}); err != nil {
					return err
				}

				// The claim is taken from the board by hand: no session works on
				// it yet, so it carries none. The session that starts on the
				// ticket takes it (startTicketClaim, AdoptClaim) and from then on
				// the daemon renews its lease; until then it lasts lease_hours
				// and oh team claim renews it.
				//
				// Try creating a new claim first.
				_, err := repo.CreateClaim(ctx, teamstate.Claim{
					TicketID:  ticketID,
					Project:   projectID,
					ClaimedBy: memberID,
					Status:    teamstate.ClaimStatusInProgress,
				})
				if err == nil {
					return nil // new claim created
				}
				if err != teamstate.ErrClaimExists {
					return err // unexpected error
				}

				// Claim already exists — check if it's a pool ticket (unowned).
				existing, getErr := repo.GetClaim(ctx, projectID, ticketID)
				if getErr != nil {
					return nil //nolint:nilerr // can't read claim, treat as idempotent
				}
				if existing.ClaimedBy != "" {
					return nil // already owned, idempotent
				}

				// Pool ticket: claim it for ourselves.
				workStatus := layout.Load().Board.DefaultWorkStatus()
				if poolErr := repo.ClaimPoolTicket(ctx, projectID, ticketID, memberID, workStatus); poolErr != nil {
					return poolErr
				}

				// Push assignee to tracker if push_labels is enabled.
				safego.Go(func() {
					pushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					engine := resolveTrackerEngine(pushCtx, a)
					if engine == nil || existing.ExternalIID == 0 {
						return
					}
					if err := engine.AssignIssueToMember(pushCtx, projectID, existing.ExternalIID, memberID); err != nil {
						slog.Warn("pool-claim.push_assignee_failed", "ticket", ticketID, "error", err)
					}
				})
				return nil
			},
			OnRelease: func(cardProject, ticketID string) error {
				repo := resolveRepo()
				if repo == nil {
					return errors.New(i18n.T("tui.team_board.team_not_configured"))
				}
				projectID := boardProjectID(a, cardProject)
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				return repo.ReleaseClaim(ctx, projectID, ticketID)
			},
			OnTransfer: func(cardProject, ticketID, toMember string) error {
				repo := resolveRepo()
				if repo == nil {
					return errors.New(i18n.T("tui.team_board.team_not_configured"))
				}
				projectID := boardProjectID(a, cardProject)
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				// A claim in progress handed to oneself is one more ticket in progress.
				if me := a.Config.ActiveTeam().MemberID; toMember == me {
					if c, gerr := repo.GetClaim(ctx, projectID, ticketID); gerr == nil {
						if err := checkTransferToSelfRoom(ctx, repo, a.Config.Tracker, me, c); err != nil {
							return err
						}
					}
				}
				return repo.TransferClaim(ctx, projectID, ticketID, toMember)
			},
			OnStatus: func(cardProject, ticketID, newStatus string) error {
				repo := resolveRepoWithBoard()
				if repo == nil {
					return errors.New(i18n.T("tui.team_board.team_not_configured"))
				}
				projectID := boardProjectID(a, cardProject)
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				// A card of the member moved to a column of work in progress is one
				// more ticket in progress.
				if c, gerr := repo.GetClaim(ctx, projectID, ticketID); gerr == nil {
					if err := checkStatusMoveRoom(ctx, repo, a.Config.Tracker, a.Config.ActiveTeam().MemberID, c, newStatus); err != nil {
						return err
					}
				}
				return repo.UpdateClaimStatus(ctx, projectID, ticketID, newStatus)
			},
		},
		QuickActions: buildBoardQuickActions(a),
	}
}

// boardProjectID is the project an action of the team board works on: the one
// of the card (a board shows the tickets of several projects, and the active
// project is only one of them), else the active project.
func boardProjectID(a *app.App, cardProject string) string {
	if cardProject != "" {
		return cardProject
	}
	if project, _ := resolveActiveProject(a); project != nil {
		return project.ID
	}
	return ""
}

// buildBeadsSummaryFunc returns a function that aggregates bead task counts
// across all active projects, keyed by bare tracker ticket ID (e.g. "693").
// This enables the [N/M] badge on the team board (ADR-032).
func buildBeadsSummaryFunc(a *app.App) func() map[string]views.BeadsSummary {
	return func() map[string]views.BeadsSummary {
		projects, err := a.Projects.List(context.Background(), domain.ProjectStatusActive)
		if err != nil {
			slog.Warn("beads-summary: failed to list projects", "error", err)
			return nil
		}

		result := make(map[string]views.BeadsSummary)
		teamID := ""
		if tuiShell != nil && tuiShell.Mode() == views.ModeTeam && tuiShell.ActiveTeam() != nil {
			teamID = tuiShell.ActiveTeam().ID
		}
		for _, p := range projects {
			if teamID != "" && (p.TeamID == nil || *p.TeamID != teamID) {
				continue
			}
			if p.Path == "" || !beads.IsInitialized(p.Path) {
				continue
			}
			tickets, err := beads.ListAll(p.Path)
			if err != nil {
				slog.Debug("beads-summary: failed to list beads", "project", p.ID, "error", err)
				continue
			}
			for _, t := range tickets {
				ref := beads.ExternalRefForTicket(t)
				if ref == "" {
					continue
				}
				// Extract the bare tracker ID from the external ref.
				// e.g. "gitlab-693" → "693", "jira-MYAPP-42" → "MYAPP-42"
				_, ticketID := beads.ParseExternalRef(ref)
				if ticketID == "" {
					continue
				}
				bs := result[ticketID]
				bs.Total++
				if t.Status == "done" || t.Status == "closed" {
					bs.Done++
				}
				result[ticketID] = bs
			}
		}
		return result
	}
}
