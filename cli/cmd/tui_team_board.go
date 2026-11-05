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
	teamstate.ClaimLifecycle
	teamstate.MemberClaimMover
}

// buildTeamBoardViewConfig builds the full TeamBoardViewConfig for the shell TUI,
// wiring the refresh, sync and action callbacks to the live team-state repo.
func buildTeamBoardViewConfig(a *app.App) views.TeamBoardViewConfig {
	// resolveRepo returns the repo of the team the board is on (ResolveTeam: the
	// team chosen in the TUI, else the team of the active project, else the
	// default team), or nil. The member of the user is the one of that same
	// team (boardMember): a board never mixes the repo of one team with the
	// member of another.
	resolveRepo := func() teamBoardRepo {
		t, err := ResolveTeam(context.Background(), a)
		if err != nil || !t.Enabled || t.StateRepo == "" {
			return nil
		}
		repo := teamstate.NewRepo(t.StateRepo, t.StatePath)
		if !repo.IsCloned() {
			return nil
		}
		return repo
	}
	boardMember := func() string { return currentTeamMember(a) }

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
		MemberID:           boardMember,
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
		TrackerSyncFunc: func() (summary string, err error) {
			// Full tracker API sync (GitLab/Jira) — called on the manual 'r'
			// key only. Recover guard: it may panic if app state is partially
			// initialized (e.g. Projects store not loaded yet).
			defer func() {
				if r := recover(); r != nil {
					summary, err = "", fmt.Errorf(i18n.T("cmd.tui_team_board.tracker_sync_panic"), r)
					safego.LogRecovered("tracker_sync.panic", r)
				}
			}()
			// No deadline of its own: the engine bounds the run (tracker.RunTimeout).
			return syncTrackerForBoard(a, tuiContext())
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
				memberID := boardMember()
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()

				// A claim someone holds is left as it is, and the board says who
				// holds it; any other one is one more ticket in progress for the
				// member: refused past max_in_progress_per_member.
				if held, gerr := repo.GetClaim(ctx, projectID, ticketID); gerr == nil && held.ClaimedBy != "" {
					return claimHeldBy(ctx, repo, held, memberID)
				}
				if err := checkInProgressRoom(ctx, repo, trackerLocalOf(ctx, a), memberID, teamstate.ClaimRef{Project: projectID, TicketID: ticketID}); err != nil {
					return err
				}

				// The claim is taken from the board by hand: no session works on
				// it yet, so it carries none. The session that starts on the
				// ticket takes it (startTicketClaim, AdoptClaim) and from then on
				// the daemon renews its lease; until then it lasts lease_hours
				// and oh team claim renews it.
				//
				// Try creating a new claim first.
				workStatus := layout.Load().Board.DefaultWorkStatus()
				// The claim.taken event goes in the commit of the claim.
				ctx = teamstate.WithClaimEvent(ctx, memberID)
				_, err := repo.CreateClaim(ctx, teamstate.Claim{
					TicketID:  ticketID,
					Project:   projectID,
					ClaimedBy: memberID,
					Status:    workStatus,
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
					return claimHeldBy(ctx, repo, existing, memberID)
				}

				// Pool ticket: claim it for ourselves.
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
			OnTakeOver: func(cardProject, ticketID string) error {
				repo := resolveRepo()
				if repo == nil {
					return errors.New(i18n.T("tui.team_board.team_not_configured"))
				}
				projectID := boardProjectID(a, cardProject)
				memberID := boardMember()
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				// Judged again on the claim as it is now: it may have been
				// renewed since the board read it.
				c, err := repo.GetClaim(ctx, projectID, ticketID)
				if err != nil {
					return err
				}
				if c.ClaimedBy == memberID {
					return nil
				}
				cfg, _ := repo.LoadConfig(ctx)
				if c.ClaimedBy != "" && (!c.HasLease(cfg) || !c.IsExpired(time.Now(), cfg)) {
					return claimHeldBy(ctx, repo, c, memberID)
				}
				if err := checkTransferToSelfRoom(ctx, repo, trackerLocalOf(ctx, a), memberID, c); err != nil {
					return err
				}
				reason := "stale"
				if !c.LeaseUntil.IsZero() {
					reason = "expired"
				}
				_, err = repo.TransferClaimWith(teamstate.WithActor(ctx, memberID), projectID, ticketID, memberID,
					teamstate.TransferOpts{Reason: reason, BriefReason: reason})
				return explainHandOver(err, memberID)
			},
			OnRelease: func(cardProject, ticketID string) error {
				repo := resolveRepo()
				if repo == nil {
					return errors.New(i18n.T("tui.team_board.team_not_configured"))
				}
				projectID := boardProjectID(a, cardProject)
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				// The owner the board showed must still be the owner: a claim
				// that changed hands meanwhile is not released under the feet of
				// its new owner.
				owner := ""
				if c, gerr := repo.GetClaim(ctx, projectID, ticketID); gerr == nil {
					owner = c.ClaimedBy
				}
				_, err := repo.ReleaseClaimAs(teamstate.WithActor(ctx, boardMember()), projectID, ticketID, teamstate.ReleaseOpts{Owner: owner})
				var moved *teamstate.ClaimNotOwnedError
				if errors.As(err, &moved) {
					return claimHeldBy(ctx, repo, &teamstate.Claim{ClaimedBy: moved.Owner}, boardMember())
				}
				return err
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
				me := boardMember()
				if toMember == me {
					if c, gerr := repo.GetClaim(ctx, projectID, ticketID); gerr == nil {
						if err := checkTransferToSelfRoom(ctx, repo, trackerLocalOf(ctx, a), me, c); err != nil {
							return err
						}
					}
				}
				// The claim, the takeover brief and the claim.transferred event
				// (signed by the member who hands it over) are one commit.
				_, err := repo.TransferClaimWith(teamstate.WithActor(ctx, me), projectID, ticketID, toMember,
					teamstate.TransferOpts{BriefReason: "transfer"})
				return explainHandOver(err, toMember)
			},
			OnReassign: func(from, to string) (int, error) {
				repo := resolveRepo()
				if repo == nil {
					return 0, errors.New(i18n.T("tui.team_board.team_not_configured"))
				}
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				me := boardMember()
				if to == me && to != "" {
					open, _, err := repo.OpenClaimsOf(ctx, from)
					if err != nil {
						return 0, err
					}
					if err := checkBulkRoom(ctx, repo, a, me, open); err != nil {
						return 0, err
					}
				}
				// All the open claims, the takeover briefs and one event per claim
				// signed by the member of the board are one commit.
				res, err := repo.ReassignClaims(teamstate.WithActor(ctx, me), teamstate.ReassignOpts{
					From: from, To: to, Release: to == "", Brief: to != "", ReleaseReason: teamstate.ReleaseBulk,
				})
				return len(res.Transferred) + len(res.Released), explainHandOver(err, to)
			},
			OnStatus: func(cardProject, ticketID, newStatus string) error {
				repo := resolveRepoWithBoard()
				if repo == nil {
					return errors.New(i18n.T("tui.team_board.team_not_configured"))
				}
				projectID := boardProjectID(a, cardProject)
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				// The column the card is dropped on stands for a status of the
				// board (the TODO column of the default board is the status
				// planned): the team-state knows the statuses, not the columns.
				status, _, known := layout.Load().Board.ResolveStatus(newStatus)
				if !known {
					return fmt.Errorf("%w: %q", teamstate.ErrInvalidStatus, newStatus)
				}
				// A card of the member moved to a column of work in progress is one
				// more ticket in progress.
				if c, gerr := repo.GetClaim(ctx, projectID, ticketID); gerr == nil {
					if err := checkStatusMoveRoom(ctx, repo, trackerLocalOf(ctx, a), boardMember(), c, status); err != nil {
						return err
					}
				}
				return repo.UpdateClaimStatus(ctx, projectID, ticketID, status)
			},
		},
		QuickActions: buildBoardQuickActions(a),
	}
}

// claimHeldBy is what taking a ticket answers when a claim holds it: nothing
// when the claim is the member's own, else a *views.ClaimHeldError that names
// the owner (the display name of their card) and says whether the claim has
// expired, so that the board tells who holds the ticket instead of announcing
// a claim that was not made.
func claimHeldBy(ctx context.Context, repo teamBoardRepo, held *teamstate.Claim, me string) error {
	if held.ClaimedBy == me {
		return nil
	}
	name := held.ClaimedBy
	if m, err := repo.GetMember(ctx, held.ClaimedBy); err == nil && m != nil && m.DisplayName != "" {
		name = m.DisplayName
	}
	cfg, _ := repo.LoadConfig(ctx)
	return &views.ClaimHeldError{Owner: name, Expired: held.HasLease(cfg) && held.IsExpired(time.Now(), cfg)}
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
