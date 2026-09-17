package views

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// ─────────────────────────────────────────────────────────────────────────────
// Quick action types
// ─────────────────────────────────────────────────────────────────────────────

// QuickActionType identifies a launchable action from the board.
type QuickActionType string

const (
	QuickActionReview QuickActionType = "review"
	QuickActionDev    QuickActionType = "dev"
	QuickActionAudit  QuickActionType = "audit"
	QuickActionDebug  QuickActionType = "debug"
)

// AuditType identifies a specific audit variant.
type AuditType string

const (
	AuditSecurity      AuditType = "security"
	AuditPerformance   AuditType = "performance"
	AuditArchitecture  AuditType = "architecture"
	AuditAccessibility AuditType = "accessibility"
	AuditEcodesign     AuditType = "ecodesign"
	AuditObservability AuditType = "observability"
	AuditPrivacy       AuditType = "privacy"
	AuditComplete      AuditType = "complete"
)

// TicketContext holds the context of the selected ticket for prompt injection.
type TicketContext struct {
	ID          string
	Title       string
	Description string // best-effort, may be empty
	Project     string // project directory ID (for team board multi-project resolution)
	ProjectPath string // resolved filesystem path
	ProjectID   string // hub project ID
}

// WorktreeEntry represents an available worktree for the env selection modal.
type WorktreeEntry struct {
	Path   string
	Branch string
}

// ActiveSessionInfo holds the relevant info for displaying active session badges.
type ActiveSessionInfo struct {
	SessionID  string
	StartedAt  time.Time
	LaunchPath string
}

// BoardQuickActions provides callbacks for launching agent sessions from boards.
// If nil in the view config, the 'a' key action is disabled.
type BoardQuickActions struct {
	// OnLaunch is called when the user picks an action + environment.
	// It receives the action type, optional audit type, ticket context, and the launch path.
	OnLaunch func(action QuickActionType, auditType AuditType, ticket TicketContext, launchPath string)

	// ListWorktrees returns available worktrees for the given project path.
	ListWorktrees func(projectPath string) []WorktreeEntry

	// CheckActiveSessions returns a map of path → active sessions for the project.
	CheckActiveSessions func(projectID string) map[string][]ActiveSessionInfo

	// ProjectPath returns the base project path. Used by BoardView (single project).
	ProjectPath func() string

	// ProjectID returns the hub project ID. Used by BoardView (single project).
	ProjectID func() string

	// ResolveProjectByDirID resolves a project path and ID from a team-state directory ID.
	// Used by TeamBoardView where tickets span multiple projects.
	ResolveProjectByDirID func(dirID string) (projectID, projectPath string, ok bool)

	// CreateWorktree creates a new worktree with the given branch name.
	// Returns the worktree filesystem path. Should be called in a goroutine.
	CreateWorktree func(projectPath, branch string) (string, error)

	// EnsureWorktreeConfig links a worktree to the project's deployed config.
	EnsureWorktreeConfig func(wtPath, projectPath string) error

	// BranchPattern returns the configured branch pattern (e.g. "feat/%s").
	// If empty, the ticket ID is used as-is.
	BranchPattern func() string

	// FetchDescription fetches the full ticket description (best-effort, for BoardView).
	// Returns empty string on failure. May be nil.
	FetchDescription func(projectPath, ticketID string) string

	// ResolveBranch returns the git branch name associated with a ticket.
	// Resolution cascade: Claim.Worktree → BranchPattern + ticket ID → sanitized ticket ID.
	// Returns empty string if no branch can be determined. May be nil.
	ResolveBranch func(ticket TicketContext) string

	// CurrentBranch returns the current git branch for the given path.
	// Returns empty string on error. May be nil.
	CurrentBranch func(path string) string

	// IsDirty reports whether the working tree at path has uncommitted changes.
	// Returns false on error (fail-open). May be nil.
	IsDirty func(path string) bool

	// CheckoutBranch switches the working tree at path to the given branch.
	// May be nil.
	CheckoutBranch func(path, branch string) error

	// StashAndCheckoutBranch stashes uncommitted changes then checks out the branch.
	// May be nil.
	StashAndCheckoutBranch func(path, branch string) error
}

// ─────────────────────────────────────────────────────────────────────────────
// Quick action modal — main entry point
// ─────────────────────────────────────────────────────────────────────────────

// showQuickActionModal shows the 4 main action choices for a ticket.
// This is the entry point called by both BoardView and TeamBoardView on 'a' key.
func showQuickActionModal(shell ShellAccess, ticket TicketContext, qa *BoardQuickActions) {
	if shell == nil || qa == nil {
		return
	}

	title := fmt.Sprintf("Actions · %s", ticket.ID)
	if ticket.Title != "" {
		truncated := ticket.Title
		if len(truncated) > 30 {
			truncated = truncated[:27] + "..."
		}
		title = fmt.Sprintf("Actions · %s · %s", ticket.ID, truncated)
	}

	options := []SelectOption{
		{Label: i18n.T("tui.board.action.review_label"), Value: string(QuickActionReview)},
		{Label: i18n.T("tui.board.action.dev_label"), Value: string(QuickActionDev)},
		{Label: i18n.T("tui.board.action.audit_label"), Value: string(QuickActionAudit)},
		{Label: i18n.T("tui.board.action.debug_label"), Value: string(QuickActionDebug)},
	}

	shell.ShowSelectModal(title, options, "", func(selected string) {
		action := QuickActionType(selected)
		if action == QuickActionAudit {
			showAuditSubMenu(shell, ticket, qa)
		} else {
			showLaunchEnvModal(shell, action, "", ticket, qa)
		}
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Audit sub-menu
// ─────────────────────────────────────────────────────────────────────────────

// showAuditSubMenu shows the 8 audit type options.
func showAuditSubMenu(shell ShellAccess, ticket TicketContext, qa *BoardQuickActions) {
	options := []SelectOption{
		{Label: i18n.T("tui.board.action.audit_security_label"), Value: string(AuditSecurity)},
		{Label: i18n.T("tui.board.action.audit_performance_label"), Value: string(AuditPerformance)},
		{Label: i18n.T("tui.board.action.audit_architecture_label"), Value: string(AuditArchitecture)},
		{Label: i18n.T("tui.board.action.audit_accessibility_label"), Value: string(AuditAccessibility)},
		{Label: i18n.T("tui.board.action.audit_ecodesign_label"), Value: string(AuditEcodesign)},
		{Label: i18n.T("tui.board.action.audit_observability_label"), Value: string(AuditObservability)},
		{Label: i18n.T("tui.board.action.audit_privacy_label"), Value: string(AuditPrivacy)},
		{Label: i18n.T("tui.board.action.audit_complete_label"), Value: string(AuditComplete)},
	}

	shell.ShowSelectModal(i18n.T("tui.board.action.audit_type_title"), options, "", func(selected string) {
		showLaunchEnvModal(shell, QuickActionAudit, selected, ticket, qa)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Launch environment modal — base / worktree picker
// ─────────────────────────────────────────────────────────────────────────────

// showLaunchEnvModal shows the environment picker: base, existing worktrees, new worktree.
// Each option is annotated with an [active] badge if a session is running on that path.
func showLaunchEnvModal(shell ShellAccess, action QuickActionType, auditType string, ticket TicketContext, qa *BoardQuickActions) {
	projectPath := ticket.ProjectPath
	projectID := ticket.ProjectID
	if projectPath == "" {
		// Fallback for BoardView: use the callback
		if qa.ProjectPath != nil {
			projectPath = qa.ProjectPath()
		}
		if qa.ProjectID != nil {
			projectID = qa.ProjectID()
		}
	}

	if projectPath == "" {
		shell.ShowToastMsg(i18n.T("tui.board.action.no_active_project"), false)
		return
	}

	// Gather active sessions for badge display.
	activeSessions := make(map[string][]ActiveSessionInfo)
	if qa.CheckActiveSessions != nil {
		activeSessions = qa.CheckActiveSessions(projectID)
	}

	// Build option list.
	var options []SelectOption

	// 1. Base project
	baseBranch := filepath.Base(projectPath)
	baseLabel := fmt.Sprintf("Base (%s)", baseBranch)
	if sessions, ok := activeSessions[projectPath]; ok && len(sessions) > 0 {
		baseLabel += i18n.T("tui.board.action.session_active_badge")
	}
	options = append(options, SelectOption{Label: baseLabel, Value: projectPath})

	// 2. Existing worktrees
	if qa.ListWorktrees != nil {
		worktrees := qa.ListWorktrees(projectPath)
		for _, wt := range worktrees {
			// Skip the main worktree (same as base path)
			if wt.Path == projectPath {
				continue
			}
			label := fmt.Sprintf("wt: %s", wt.Branch)
			if sessions, ok := activeSessions[wt.Path]; ok && len(sessions) > 0 {
				label += i18n.T("tui.board.action.session_active_badge")
			}
			options = append(options, SelectOption{Label: label, Value: wt.Path})
		}
	}

	// 3. New worktree option
	options = append(options, SelectOption{Label: i18n.T("tui.board.action.new_worktree"), Value: "__new_worktree__"})

	shell.ShowSelectModal(i18n.T("tui.board.action.where_to_run"), options, "", func(selected string) {
		if selected == "__new_worktree__" {
			showNewWorktreeModal(shell, action, auditType, ticket, qa, projectPath, projectID, activeSessions)
			return
		}

		// Wrap launch logic to handle session warnings uniformly.
		launchOnPath := func(launchPath string) {
			if sessions, ok := activeSessions[launchPath]; ok && len(sessions) > 0 {
				showActiveSessionWarning(shell, sessions, func() {
					qa.OnLaunch(action, AuditType(auditType), ticket, launchPath)
				})
				return
			}
			qa.OnLaunch(action, AuditType(auditType), ticket, launchPath)
		}

		// If the user selected the base project, attempt branch checkout for the ticket.
		if selected == projectPath {
			handleBaseWithBranchSwitch(shell, action, auditType, ticket, qa, projectPath, projectID, activeSessions, launchOnPath)
			return
		}

		// Worktree selected — launch directly (worktrees are already on their branch).
		launchOnPath(selected)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Base checkout logic — switch to ticket branch before launching
// ─────────────────────────────────────────────────────────────────────────────

// handleBaseWithBranchSwitch resolves the ticket's branch and attempts to
// checkout on the base project before launching. If the tree is dirty, shows
// a modal offering to stash+checkout, use a worktree, or cancel.
func handleBaseWithBranchSwitch(
	shell ShellAccess,
	action QuickActionType,
	auditType string,
	ticket TicketContext,
	qa *BoardQuickActions,
	projectPath, projectID string,
	activeSessions map[string][]ActiveSessionInfo,
	launchOnPath func(string),
) {
	// Resolve the target branch for this ticket.
	targetBranch := ""
	if qa.ResolveBranch != nil {
		targetBranch = qa.ResolveBranch(ticket)
	}
	if targetBranch == "" {
		// No branch to switch to — launch on base as-is.
		launchOnPath(projectPath)
		return
	}

	// Check if we're already on the target branch.
	if qa.CurrentBranch != nil {
		current := qa.CurrentBranch(projectPath)
		if current == targetBranch {
			// Already on the right branch — no checkout needed.
			launchOnPath(projectPath)
			return
		}
	}

	// Check for dirty state.
	dirty := false
	if qa.IsDirty != nil {
		dirty = qa.IsDirty(projectPath)
	}

	if !dirty {
		// Clean tree — attempt direct checkout in a goroutine.
		shell.ShowToastMsg(i18n.Tf("tui.board.action.checkout_progress", targetBranch), true)
		go func() {
			if qa.CheckoutBranch == nil {
				shell.ShowToastMsg(i18n.T("tui.board.action.checkout_not_configured"), false)
				return
			}
			if err := qa.CheckoutBranch(projectPath, targetBranch); err != nil {
				slog.Warn("quick-action: checkout failed", "branch", targetBranch, "error", err)
				shell.ShowToastMsg(i18n.Tf("tui.board.action.checkout_failed", err), false)
				return
			}
			shell.ShowToastMsg(i18n.Tf("tui.board.action.on_branch", targetBranch), true)
			launchOnPath(projectPath)
		}()
		return
	}

	// Dirty tree — show options to the user.
	showDirtyStateModal(shell, action, auditType, ticket, qa, projectPath, projectID, targetBranch, activeSessions, launchOnPath)
}

// showDirtyStateModal shows a modal when the base tree has uncommitted changes
// and a branch switch is needed. Offers three options:
// - Stash & Checkout: stash changes, checkout branch, launch
// - Utiliser un worktree: create a worktree for the branch instead
// - Annuler: cancel
func showDirtyStateModal(
	shell ShellAccess,
	action QuickActionType,
	auditType string,
	ticket TicketContext,
	qa *BoardQuickActions,
	projectPath, projectID, targetBranch string,
	activeSessions map[string][]ActiveSessionInfo,
	launchOnPath func(string),
) {
	content := i18n.Tf("tui.board.action.dirty_state_msg", targetBranch)

	shell.ShowScrollableModal(i18n.T("tui.board.action.dirty_state_title"), content, []ModalAction{
		{
			Label: "Stash & Checkout",
			Callback: func() {
				shell.ShowToastMsg(i18n.T("tui.board.action.stash_checkout_progress"), true)
				go func() {
					if qa.StashAndCheckoutBranch == nil {
						shell.ShowToastMsg(i18n.T("tui.board.action.stash_not_configured"), false)
						return
					}
					if err := qa.StashAndCheckoutBranch(projectPath, targetBranch); err != nil {
						slog.Warn("quick-action: stash+checkout failed", "branch", targetBranch, "error", err)
						shell.ShowToastMsg(i18n.Tf("tui.board.action.stash_checkout_failed", err), false)
						return
					}
					shell.ShowToastMsg(i18n.Tf("tui.board.action.on_branch_stashed", targetBranch), true)
					launchOnPath(projectPath)
				}()
			},
		},
		{
			Label: i18n.T("tui.board.action.use_worktree"),
			Callback: func() {
				showNewWorktreeModal(shell, action, auditType, ticket, qa, projectPath, projectID, activeSessions)
			},
		},
		{
			Label:    i18n.T("tui.board.action.cancel"),
			Callback: func() {},
		},
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// New worktree creation modal
// ─────────────────────────────────────────────────────────────────────────────

// showNewWorktreeModal shows an input modal for the new worktree branch name,
// pre-filled with the ticket ID using the configured branch pattern.
func showNewWorktreeModal(
	shell ShellAccess,
	action QuickActionType,
	auditType string,
	ticket TicketContext,
	qa *BoardQuickActions,
	projectPath, projectID string,
	activeSessions map[string][]ActiveSessionInfo,
) {
	// Build pre-filled branch name from ticket ID + pattern.
	defaultBranch := sanitizeBranchName(ticket.ID)
	if qa.BranchPattern != nil {
		pattern := qa.BranchPattern()
		if pattern != "" && strings.Contains(pattern, "%s") {
			defaultBranch = fmt.Sprintf(pattern, sanitizeBranchName(ticket.ID))
		}
	}

	shell.ShowInputModal(i18n.T("tui.board.action.branch_name_title"), defaultBranch, func(branch string) {
		if branch == "" {
			return
		}

		shell.ShowToastMsg(i18n.T("tui.board.action.worktree_creating"), true)

		// Run worktree creation in a goroutine with timeout to avoid blocking the TUI.
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			var wtPath string
			var err error

			// Create worktree (with timeout via context).
			done := make(chan struct{})
			go func() {
				defer close(done)
				if qa.CreateWorktree != nil {
					wtPath, err = qa.CreateWorktree(projectPath, branch)
				} else {
					err = fmt.Errorf("worktree creation not configured")
				}
			}()

			select {
			case <-done:
				// Completed normally.
			case <-ctx.Done():
				err = errors.New(i18n.T("tui.board.action.worktree_timeout"))
			}

			if err != nil {
				slog.Warn("quick-action: worktree creation failed", "branch", branch, "error", err)
				shell.ShowToastMsg(i18n.Tf("tui.board.action.worktree_failed", err), false)
				return
			}

			// Link worktree config.
			if qa.EnsureWorktreeConfig != nil {
				if cfgErr := qa.EnsureWorktreeConfig(wtPath, projectPath); cfgErr != nil {
					slog.Warn("quick-action: worktree config failed", "path", wtPath, "error", cfgErr)
					shell.ShowToastMsg(i18n.Tf("tui.board.action.worktree_config_failed", cfgErr), false)
					// Continue anyway — the worktree exists, user can fix config later.
				}
			}

			shell.ShowToastMsg(i18n.Tf("tui.board.action.worktree_created", filepath.Base(wtPath)), true)

			// Launch the action on the new worktree.
			// No session warning needed — it's brand new.
			qa.OnLaunch(action, AuditType(auditType), ticket, wtPath)
		}()
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Active session warning
// ─────────────────────────────────────────────────────────────────────────────

// showActiveSessionWarning shows a non-blocking warning before launching on a path
// that has an active session.
func showActiveSessionWarning(shell ShellAccess, sessions []ActiveSessionInfo, onConfirm func()) {
	warning := formatSessionWarning(sessions)

	shell.ShowScrollableModal(i18n.T("tui.board.action.active_session_title"), warning, []ModalAction{
		{Label: i18n.T("tui.board.action.continue_btn"), Callback: onConfirm},
		{Label: i18n.T("tui.board.action.cancel"), Callback: func() {}}, // dismiss only
	})
}

// formatSessionWarning produces a readable warning for the active session modal.
func formatSessionWarning(sessions []ActiveSessionInfo) string {
	if len(sessions) == 0 {
		return ""
	}
	if len(sessions) == 1 {
		elapsed := time.Since(sessions[0].StartedAt).Truncate(time.Minute)
		return i18n.Tf("tui.board.action.session_warning_single", formatDurationFr(elapsed))
	}
	return i18n.Tf("tui.board.action.session_warning_multi", len(sessions))
}

// formatDurationFr formats a duration in French.
func formatDurationFr(d time.Duration) string {
	if d < time.Minute {
		return i18n.T("tui.board.action.duration_less_than_minute")
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 0 && minutes > 0 {
		return fmt.Sprintf("%dh%02dmin", hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dmin", minutes)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// sanitizeBranchName removes characters that are invalid in git branch names.
func sanitizeBranchName(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '/' || r == '_' {
			return r
		}
		return '-'
	}, s)
	// Collapse multiple dashes.
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	return s
}

// BuildTicketPrompt builds the --prompt argument for ticket-contextualized sessions.
func BuildTicketPrompt(action QuickActionType, auditType AuditType, ticket TicketContext) string {
	var prefix string
	switch action {
	case QuickActionReview:
		prefix = "Review"
	case QuickActionDev:
		prefix = i18n.T("tui.board.action.prompt_dev")
	case QuickActionAudit:
		switch auditType {
		case AuditSecurity:
			prefix = i18n.T("tui.board.action.prompt_audit_security")
		case AuditPerformance:
			prefix = i18n.T("tui.board.action.prompt_audit_performance")
		case AuditArchitecture:
			prefix = i18n.T("tui.board.action.prompt_audit_architecture")
		case AuditAccessibility:
			prefix = i18n.T("tui.board.action.prompt_audit_accessibility")
		case AuditEcodesign:
			prefix = i18n.T("tui.board.action.prompt_audit_ecodesign")
		case AuditObservability:
			prefix = i18n.T("tui.board.action.prompt_audit_observability")
		case AuditPrivacy:
			prefix = i18n.T("tui.board.action.prompt_audit_privacy")
		case AuditComplete:
			prefix = i18n.T("tui.board.action.prompt_audit_complete")
		default:
			prefix = i18n.T("tui.board.action.prompt_audit_generic")
		}
	case QuickActionDebug:
		prefix = "Debug"
	default:
		prefix = i18n.T("tui.board.action.prompt_default")
	}

	prompt := i18n.Tf("tui.board.action.prompt_ticket", prefix, ticket.ID)
	if ticket.Title != "" {
		prompt += fmt.Sprintf(" (%s)", ticket.Title)
	}
	if ticket.Description != "" {
		// Limit description to avoid excessively long prompts.
		desc := ticket.Description
		if len(desc) > 500 {
			desc = desc[:497] + "..."
		}
		prompt += fmt.Sprintf("\n\nDescription:\n%s", desc)
	}
	return prompt
}
