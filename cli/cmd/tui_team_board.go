package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
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

	// Build a project name resolver: maps directory IDs to human-friendly names.
	projectNameResolver := buildProjectNameResolver(a)

	// Load tickets from the local clone immediately — no network call.
	// The async SyncFunc will refresh after pulling remote changes.
	// Use a recover guard for safety (resolveRepo may panic with minimal test fixtures).
	var initialTickets []views.TeamTicket
	func() {
		defer func() { recover() }()
		if repo := resolveRepo(); repo != nil {
			initialTickets = views.FetchTeamTickets(repo, projectNameResolver)
		}
	}()

	// Load label_status_mapping from team config for label filtering in the board view.
	var labelStatusMapping map[string]string
	func() {
		defer func() { recover() }()
		if repo := resolveRepo(); repo != nil {
			if cfg, err := repo.LoadConfig(); err == nil && cfg != nil {
				labelStatusMapping = cfg.Tracker.LabelStatusMapping
			}
		}
	}()

	return views.TeamBoardViewConfig{
		Tickets:            initialTickets,
		RefreshRate:        5 * time.Second,
		LabelStatusMapping: labelStatusMapping,
		IsConfigured: func() bool {
			return resolveRepo() != nil
		},
		BeadsSummaryFunc: buildBeadsSummaryFunc(a),
		RefreshFunc: func() []views.TeamTicket {
			repo := resolveRepo()
			if repo == nil {
				return nil
			}
			return views.FetchTeamTickets(repo, projectNameResolver)
		},
		SyncFunc: func() error {
			// Git pull only — fetches colleagues' changes from the shared state repo.
			// Tracker sync (GitLab/Jira API) is intentionally NOT triggered here to
			// avoid slow API calls and timeout cascades on every board entry.
			// Use the "Sync Tracker" omnibar command for a full tracker reconciliation.
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			repo := resolveRepo()
			if repo == nil {
				return nil
			}
			return repo.Pull(ctx)
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
		QuickActions: buildBoardQuickActions(a),
	}
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
		for _, p := range projects {
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

// buildProjectNameResolver creates a resolver that maps team-state project directory
// IDs to human-friendly project names from the hub's project registry.
func buildProjectNameResolver(a *app.App) views.ProjectNameResolver {
	if a.Projects == nil {
		return nil
	}
	// Pre-load the mapping once — project names don't change during a TUI session.
	projects, _ := a.Projects.List(context.Background(), domain.ProjectStatusActive)
	nameByID := make(map[string]string, len(projects))
	for _, p := range projects {
		nameByID[p.ID] = p.Name
	}
	return func(dirID string) string {
		if name, ok := nameByID[dirID]; ok {
			return name
		}
		return dirID // fallback to directory name
	}
}
