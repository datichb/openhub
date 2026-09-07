package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// buildTeamBoardViewConfig builds the full TeamBoardViewConfig for the shell TUI,
// wiring the refresh, sync and action callbacks to the live team-state repo.
func buildTeamBoardViewConfig(a *app.App) views.TeamBoardViewConfig {
	// resolveRepo is a helper that returns the active team repo (or nil).
	// Resolution order (ADR-032):
	//   1. Project-level team config (TeamID or legacy TeamConfig)
	//   2. Hub-level ActiveTeam() — matches CLI sync-tracker behavior
	// This ensures tickets synced by the CLI are visible in the TUI.
	resolveRepo := func() *teamstate.Repo {
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

	return views.TeamBoardViewConfig{
		RefreshRate: 5 * time.Second,
		RefreshFunc: func() []views.TeamTicket {
			repo := resolveRepo()
			if repo == nil {
				return nil
			}
			return views.FetchTeamTickets(repo)
		},
		SyncFunc: func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			repo := resolveRepo()
			if repo == nil {
				return nil
			}
			if err := repo.Pull(ctx); err != nil {
				return err
			}
			// Tracker sync is best-effort — don't block on errors.
			if engine := resolveTrackerEngine(ctx, a); engine != nil && engine.ShouldAutoSync() {
				_, _ = engine.Run(ctx)
			}
			return nil
		},
		Actions: &views.BoardActions{
			Members: func() []views.SelectOption {
				repo := resolveRepo()
				if repo == nil {
					return nil
				}
				members, err := repo.ListMembers()
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
					return "", "", fmt.Errorf("tracker non configuré")
				}
				return engine.FetchTicketDetail(ctx, project, ticketID)
			},
			OnClaim: func(ticketID string) error {
				repo := resolveRepo()
				if repo == nil {
					return fmt.Errorf("team non configurée")
				}
				project, _ := resolveActiveProject(a)
				projectID := ""
				if project != nil {
					projectID = project.ID
				}
				memberID := a.Config.ActiveTeam().MemberID
				ctx := context.Background()
				_, err := repo.CreateClaim(ctx, teamstate.Claim{
					TicketID:  ticketID,
					Project:   projectID,
					ClaimedBy: memberID,
					Status:    teamstate.ClaimStatusInProgress,
				})
				if err == teamstate.ErrClaimExists {
					return nil // idempotent
				}
				return err
			},
			OnRelease: func(ticketID string) error {
				repo := resolveRepo()
				if repo == nil {
					return fmt.Errorf("team non configurée")
				}
				project, _ := resolveActiveProject(a)
				projectID := ""
				if project != nil {
					projectID = project.ID
				}
				ctx := context.Background()
				return repo.ReleaseClaim(ctx, projectID, ticketID)
			},
			OnTransfer: func(ticketID, toMember string) error {
				repo := resolveRepo()
				if repo == nil {
					return fmt.Errorf("team non configurée")
				}
				project, _ := resolveActiveProject(a)
				projectID := ""
				if project != nil {
					projectID = project.ID
				}
				ctx := context.Background()
				return repo.TransferClaim(ctx, projectID, ticketID, toMember)
			},
			OnStatus: func(ticketID, newStatus string) error {
				repo := resolveRepo()
				if repo == nil {
					return fmt.Errorf("team non configurée")
				}
				project, _ := resolveActiveProject(a)
				projectID := ""
				if project != nil {
					projectID = project.ID
				}
				ctx := context.Background()
				return repo.UpdateClaimStatus(ctx, projectID, ticketID, newStatus)
			},
		},
	}
}
