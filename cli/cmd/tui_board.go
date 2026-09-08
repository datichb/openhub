package cmd

import (
	"context"
	"log/slog"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// buildBoardQuickActions builds the BoardQuickActions struct for the project-level BoardView.
func buildBoardQuickActions(a *app.App) *views.BoardQuickActions {
	return &views.BoardQuickActions{
		OnLaunch: func(action views.QuickActionType, auditType views.AuditType, ticket views.TicketContext, launchPath string) {
			agent, extraArgs := resolveAgentForAction(action, auditType)
			prompt := views.BuildTicketPrompt(action, auditType, ticket)

			projectID := ticket.ProjectID
			if projectID == "" {
				if p, err := resolveActiveProject(a); err == nil && p != nil {
					projectID = p.ID
				}
			}

			launchOpcodeAtPath(launchPath, projectID, agent, append([]string{"--prompt", prompt}, extraArgs...)...)
		},

		ListWorktrees: func(projectPath string) []views.WorktreeEntry {
			entries, err := worktree.List(projectPath)
			if err != nil {
				slog.Debug("quick-action: failed to list worktrees", "error", err)
				return nil
			}
			result := make([]views.WorktreeEntry, 0, len(entries))
			for _, e := range entries {
				if e.IsBare {
					continue
				}
				result = append(result, views.WorktreeEntry{
					Path:   e.Path,
					Branch: e.Branch,
				})
			}
			return result
		},

		CheckActiveSessions: func(projectID string) map[string][]views.ActiveSessionInfo {
			if a.Sessions == nil {
				return nil
			}
			ctx := context.Background()
			byPath, err := opencode.FindAllActiveSessionsByPath(ctx, a.Sessions, projectID)
			if err != nil {
				slog.Debug("quick-action: failed to check active sessions", "error", err)
				return nil
			}
			// Convert opencode.ActiveSessionInfo → views.ActiveSessionInfo
			result := make(map[string][]views.ActiveSessionInfo, len(byPath))
			for path, sessions := range byPath {
				vs := make([]views.ActiveSessionInfo, len(sessions))
				for i, s := range sessions {
					vs[i] = views.ActiveSessionInfo{
						SessionID:  s.SessionID,
						StartedAt:  s.StartedAt,
						LaunchPath: s.LaunchPath,
					}
				}
				result[path] = vs
			}
			return result
		},

		ProjectPath: func() string {
			return resolveActiveProjectPath(a)
		},

		ProjectID: func() string {
			p, err := resolveActiveProject(a)
			if err != nil || p == nil {
				return ""
			}
			return p.ID
		},

		ResolveProjectByDirID: buildResolveProjectByDirID(a),

		CreateWorktree: func(projectPath, branch string) (string, error) {
			return worktree.ResolveOrCreate(projectPath, branch)
		},

		EnsureWorktreeConfig: func(wtPath, projectPath string) error {
			return worktree.EnsureWorktreeConfig(wtPath, projectPath)
		},

		BranchPattern: func() string {
			return a.Config.Worktree.BranchPattern
		},

		FetchDescription: func(projectPath, ticketID string) string {
			detail, err := beads.Show(projectPath, ticketID)
			if err != nil {
				return ""
			}
			if detail == nil {
				return ""
			}
			return detail.Description
		},

		ResolveBranch: func(ticket views.TicketContext) string {
			// Cascade 1: try Claim.Worktree from teamstate (if team is configured).
			if ticket.Project != "" && ticket.ID != "" {
				if repo := resolveTeamRepo(a); repo != nil {
					claim, err := repo.GetClaim(ticket.Project, ticket.ID)
					if err == nil && claim != nil && claim.Worktree != "" {
						return claim.Worktree
					}
				}
			}
			// Cascade 2: generate from BranchPattern + ticket ID.
			return worktree.BranchName(a.Config.Worktree.BranchPattern, sanitizeForBranch(ticket.ID))
		},

		CurrentBranch: func(path string) string {
			branch, err := worktree.CurrentBranch(path)
			if err != nil {
				return ""
			}
			return branch
		},

		IsDirty: func(path string) bool {
			return worktree.IsDirty(path)
		},

		CheckoutBranch: func(path, branch string) error {
			return worktree.Checkout(path, branch)
		},

		StashAndCheckoutBranch: func(path, branch string) error {
			return worktree.StashAndCheckout(path, branch)
		},
	}
}

// buildResolveProjectByDirID builds a resolver that maps team-state directory IDs
// to project IDs and paths from the hub's project registry.
func buildResolveProjectByDirID(a *app.App) func(dirID string) (string, string, bool) {
	if a.Projects == nil {
		return nil
	}
	// Pre-load the mapping once — project list doesn't change during a TUI session.
	projects, _ := a.Projects.List(context.Background(), domain.ProjectStatusActive)
	byID := make(map[string]*domain.Project, len(projects))
	for i := range projects {
		byID[projects[i].ID] = &projects[i]
	}
	return func(dirID string) (string, string, bool) {
		// dirID from team-state is usually the project's directory name / ID.
		if p, ok := byID[dirID]; ok {
			return p.ID, p.Path, true
		}
		return "", "", false
	}
}

// resolveAgentForAction maps a QuickActionType + AuditType to the agent name and extra args
// that opencode expects.
func resolveAgentForAction(action views.QuickActionType, auditType views.AuditType) (string, []string) {
	switch action {
	case views.QuickActionReview:
		return "reviewer", nil
	case views.QuickActionDev:
		return "", []string{"--dev"}
	case views.QuickActionAudit:
		agent := "auditor"
		switch auditType {
		case views.AuditComplete:
			return agent, []string{"--type", "all"}
		case views.AuditSecurity, views.AuditPerformance, views.AuditArchitecture,
			views.AuditAccessibility, views.AuditEcodesign, views.AuditObservability,
			views.AuditPrivacy:
			return agent, []string{"--type", string(auditType)}
		default:
			return agent, nil
		}
	case views.QuickActionDebug:
		return "debugger", nil
	default:
		return "", nil
	}
}

// resolveTeamRepo returns the active team-state repo, or nil if team is not configured.
// Mirrors the resolution logic in buildTeamBoardViewConfig.
func resolveTeamRepo(a *app.App) *teamstate.Repo {
	// Try project-level first
	project, _ := resolveActiveProject(a)
	tc := resolvedTeamConfig(a, project)
	if tc.Enabled && tc.StateRepo != "" {
		repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
		if repo.IsCloned() {
			return repo
		}
	}
	// Fallback: hub-level
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

// sanitizeForBranch normalises a ticket ID for use as a git branch name segment.
func sanitizeForBranch(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}
