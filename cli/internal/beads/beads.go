// Package beads provides an interface to the bd (Beads) CLI for ticket management.
package beads

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Ticket represents a bd ticket.
type Ticket struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	Type        string   `json:"type"`
	Parent      string   `json:"parent,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	ExternalRef string   `json:"external_ref,omitempty"`
}

// ReadyOpts configures the ListReady query.
type ReadyOpts struct {
	Label    string // filter by label (default: "ai-delegated")
	Assignee string // filter by assignee
}

// Available checks if bd is in PATH.
func Available() error {
	_, err := exec.LookPath("bd")
	if err != nil {
		return fmt.Errorf("bd not found in PATH: install with: brew install datichb/tap/bd")
	}
	return nil
}

// BdCommand creates an exec.Cmd for the bd binary with the
// BD_ALLOW_REMOTE_MIGRATE=1 env var set, so that schema migrations
// are never blocked when oh calls bd internally. This prevents the
// "refusing to auto-apply pending schema migrations" error that occurs
// when the bd binary is upgraded and the local database needs migration.
func BdCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "bd", args...)
	cmd.Env = append(os.Environ(), "BD_ALLOW_REMOTE_MIGRATE=1")
	return cmd
}

// ListReady returns tickets ready for implementation.
// Runs: bd -C <path> ready [--label <l> | --assignee <a>] --json
func ListReady(projectPath string, opts ReadyOpts) ([]Ticket, error) {
	args := []string{"-C", projectPath, "ready", "--json"}
	if opts.Assignee != "" {
		args = append(args[:len(args)-1], "--assignee", opts.Assignee, "--json")
	} else if opts.Label != "" {
		args = append(args[:len(args)-1], "--label", opts.Label, "--json")
	}

	return runBdJSON(args)
}

// ListAll returns all tickets (flat, no tree nesting).
// Runs: bd -C <path> list --json --flat
func ListAll(projectPath string) ([]Ticket, error) {
	args := []string{"-C", projectPath, "list", "--json", "--flat"}
	return runBdJSON(args)
}

// ListCWD returns all tickets from the current working directory (flat, no tree nesting).
// Runs: bd list --json --flat
// This is used by the CLI board command which operates in the cwd.
func ListCWD() ([]Ticket, error) {
	return runBdJSON([]string{"list", "--json", "--flat"})
}

// IsInitialized reports whether the project at projectPath has a .beads/ directory.
func IsInitialized(projectPath string) bool {
	_, err := os.Stat(filepath.Join(projectPath, ".beads"))
	return err == nil
}

// zeroImpactFlags are the bd init flags that prevent side effects on the host project.
var zeroImpactFlags = []string{"--skip-hooks", "--skip-agents", "--setup-exclude"}

// EnsureInitFlags ensures that zero-impact flags (--skip-hooks, --skip-agents,
// --setup-exclude) are present on a bd init argument list.
// If --stealth is already present, no flags are added (stealth disables everything).
// The input slice is never mutated; a new slice is returned when flags are appended.
func EnsureInitFlags(args []string) []string {
	present := make(map[string]bool, len(zeroImpactFlags))
	for _, a := range args {
		if a == "--stealth" {
			return args // stealth already disables hooks + uses global gitignore
		}
		for _, f := range zeroImpactFlags {
			if a == f {
				present[f] = true
			}
		}
	}

	var missing []string
	for _, f := range zeroImpactFlags {
		if !present[f] {
			missing = append(missing, f)
		}
	}
	if len(missing) == 0 {
		return args
	}

	out := make([]string, len(args), len(args)+len(missing))
	copy(out, args)
	return append(out, missing...)
}

// Init initializes beads in the given project directory.
// prefix is used for ticket ID prefixes (e.g. project ID or short name).
// It also registers the default labels used by the hub agents.
func Init(projectPath, prefix string) error {
	if err := Available(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := BdCommand(ctx, "-C", projectPath, "init",
		"--prefix", prefix,
		"--skip-hooks", "--skip-agents", "--setup-exclude")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("bd init: %s", strings.TrimSpace(string(out)))
	}

	// Ensure local-only mode so schema migrations auto-apply on bd upgrades.
	// This prevents the "refusing to auto-apply pending schema migrations to a
	// remote-backed database" error. Best-effort: ignore errors from older bd
	// versions that may not support this config key.
	localCtx, localCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer localCancel()
	_ = BdCommand(localCtx, "-C", projectPath, "config", "set", "dolt.local-only", "true").Run()

	// Belt & suspenders: sanitize in case flags were ignored or bd evolved.
	_ = SanitizeBeadsInit(projectPath)

	// Register default labels used by the hub agents
	for _, label := range []string{"ai-delegated", "feature", "fix"} {
		lctx, lcancel := context.WithTimeout(context.Background(), 10*time.Second)
		BdCommand(lctx, "-C", projectPath, "label", "create", label).Run() //nolint:errcheck // best-effort label creation, failure is non-fatal
		lcancel()
	}
	return nil
}

// ListEpics returns tickets of type "epic" from the full ticket list.
func ListEpics(projectPath string) ([]Ticket, error) {
	all, err := ListAll(projectPath)
	if err != nil {
		return nil, err
	}

	var epics []Ticket
	for _, t := range all {
		if t.Type == "epic" {
			epics = append(epics, t)
		}
	}
	return epics, nil
}

// Children returns child tickets of an epic.
// Runs: bd -C <path> children <epicID> --json
func Children(projectPath, epicID string) ([]Ticket, error) {
	args := []string{"-C", projectPath, "children", epicID, "--json"}
	return runBdJSON(args)
}

// ReadyChildren returns children of an epic that have a "ready" or "open" status.
func ReadyChildren(projectPath, epicID string) ([]Ticket, error) {
	children, err := Children(projectPath, epicID)
	if err != nil {
		return nil, err
	}

	var ready []Ticket
	for _, t := range children {
		if isReadyStatus(t.Status) {
			ready = append(ready, t)
		}
	}
	return ready, nil
}

// EpicWithCount represents an epic with its count of ready children.
type EpicWithCount struct {
	Ticket     Ticket
	ReadyCount int
}

// ListEpicsWithReadyChildren returns epics that have at least one ready child ticket.
func ListEpicsWithReadyChildren(projectPath string) ([]EpicWithCount, error) {
	epics, err := ListEpics(projectPath)
	if err != nil {
		return nil, err
	}

	var result []EpicWithCount
	for _, epic := range epics {
		children, err := ReadyChildren(projectPath, epic.ID)
		if err != nil {
			continue // skip epics where children query fails
		}
		if len(children) > 0 {
			result = append(result, EpicWithCount{
				Ticket:     epic,
				ReadyCount: len(children),
			})
		}
	}
	return result, nil
}

// OrphanTickets returns ready tickets that have no parent epic.
// If labelFilter is non-empty, only returns tickets matching that label.
func OrphanTickets(projectPath, labelFilter string) (withLabel, withoutLabel []Ticket, err error) {
	all, err := ListAll(projectPath)
	if err != nil {
		return nil, nil, err
	}

	defaultLabel := "ai-delegated"
	if labelFilter != "" {
		defaultLabel = labelFilter
	}

	for _, t := range all {
		// Skip epics and non-ready tickets
		if t.Type == "epic" || !isReadyStatus(t.Status) {
			continue
		}
		// Skip tickets with a parent (they belong to an epic)
		if t.Parent != "" {
			continue
		}

		if hasLabel(t, defaultLabel) {
			withLabel = append(withLabel, t)
		} else {
			withoutLabel = append(withoutLabel, t)
		}
	}
	return withLabel, withoutLabel, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Dev picker variants — include both TODO and IN_PROGRESS tickets.
// ─────────────────────────────────────────────────────────────────────────────

// DevPickableChildren returns children of an epic that are either ready or in-progress.
func DevPickableChildren(projectPath, epicID string) ([]Ticket, error) {
	children, err := Children(projectPath, epicID)
	if err != nil {
		return nil, err
	}

	var pickable []Ticket
	for _, t := range children {
		if isDevPickableStatus(t.Status) {
			pickable = append(pickable, t)
		}
	}
	return pickable, nil
}

// ListEpicsWithDevPickableChildren returns epics that have at least one child
// ticket in a dev-pickable status (ready or in-progress).
func ListEpicsWithDevPickableChildren(projectPath string) ([]EpicWithCount, error) {
	epics, err := ListEpics(projectPath)
	if err != nil {
		return nil, err
	}

	var result []EpicWithCount
	for _, epic := range epics {
		children, err := DevPickableChildren(projectPath, epic.ID)
		if err != nil {
			continue
		}
		if len(children) > 0 {
			result = append(result, EpicWithCount{
				Ticket:     epic,
				ReadyCount: len(children),
			})
		}
	}
	return result, nil
}

// DevPickableOrphanTickets returns tickets (ready or in-progress) that have no parent epic.
// If labelFilter is non-empty, only returns tickets matching that label.
func DevPickableOrphanTickets(projectPath, labelFilter string) (withLabel, withoutLabel []Ticket, err error) {
	all, err := ListAll(projectPath)
	if err != nil {
		return nil, nil, err
	}

	defaultLabel := "ai-delegated"
	if labelFilter != "" {
		defaultLabel = labelFilter
	}

	for _, t := range all {
		if t.Type == "epic" || !isDevPickableStatus(t.Status) {
			continue
		}
		if t.Parent != "" {
			continue
		}

		if hasLabel(t, defaultLabel) {
			withLabel = append(withLabel, t)
		} else {
			withoutLabel = append(withoutLabel, t)
		}
	}
	return withLabel, withoutLabel, nil
}

// TicketDetail holds the full content of a bd ticket as returned by bd show --json.
// Fields are optional — they are empty when not set on the ticket.
// Field names match the exact JSON keys returned by bd (snake_case).
type TicketDetail struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Status      string          `json:"status"`
	Priority    json.RawMessage `json:"priority"`
	Type        string          `json:"issue_type"`
	Parent      string          `json:"parent,omitempty"`
	Labels      []string        `json:"labels,omitempty"`
	Description string          `json:"description,omitempty"`
	Acceptance  string          `json:"acceptance_criteria,omitempty"`
	Notes       string          `json:"notes,omitempty"`
	Design      string          `json:"design,omitempty"`
	Estimate    int             `json:"estimated_minutes,omitempty"`
	Assignee    string          `json:"owner,omitempty"`
	ExternalRef string          `json:"external_ref,omitempty"`
	CloseReason string          `json:"close_reason,omitempty"`
}

// PriorityString returns the priority as a human-readable string (e.g. "P0").
func (d *TicketDetail) PriorityString() string {
	return normalizePriority(d.Priority)
}

// Show returns the full detail of a ticket by ID.
// Runs: bd -C <path> show <id> --json
// Note: bd show returns a JSON array containing a single ticket object.
func Show(projectPath, ticketID string) (*TicketDetail, error) {
	if err := Available(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := BdCommand(ctx, "-C", projectPath, "show", ticketID, "--json")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("bd show: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("bd show: %w", err)
	}
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return nil, fmt.Errorf("bd show: empty response for ticket %s", ticketID)
	}
	// bd show returns a JSON array with a single element — not a bare object.
	var results []TicketDetail
	if err := json.Unmarshal([]byte(trimmed), &results); err != nil {
		return nil, fmt.Errorf("bd show: parsing response: %w", err)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("bd show: no result for ticket %s", ticketID)
	}
	return &results[0], nil
}

// runBdJSON executes a bd command and parses the JSON output.
func runBdJSON(args []string) ([]Ticket, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := BdCommand(ctx, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("bd %s: %s", strings.Join(args[2:], " "), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("bd command failed: %w", err)
	}

	// Handle empty output
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" || trimmed == "[]" {
		return nil, nil
	}

	// Use an intermediate struct because bd may return priority as a JSON number
	// (0-3) rather than a string ("P0"-"P3").
	// Field names match the exact JSON keys returned by bd (snake_case).
	var raw []struct {
		ID          string          `json:"id"`
		Title       string          `json:"title"`
		Status      string          `json:"status"`
		Priority    json.RawMessage `json:"priority"`
		Type        string          `json:"issue_type"`
		Parent      string          `json:"parent,omitempty"`
		Labels      []string        `json:"labels,omitempty"`
		ExternalRef string          `json:"external_ref,omitempty"`
	}
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return nil, fmt.Errorf("parsing bd output: %w", err)
	}

	tickets := make([]Ticket, len(raw))
	for i, r := range raw {
		tickets[i] = Ticket{
			ID:          r.ID,
			Title:       r.Title,
			Status:      r.Status,
			Priority:    normalizePriority(r.Priority),
			Type:        r.Type,
			Parent:      r.Parent,
			Labels:      r.Labels,
			ExternalRef: r.ExternalRef,
		}
	}
	return tickets, nil
}

// normalizePriority converts a bd priority value (number or string) to a
// canonical string form. bd may return either a numeric value (0, 1, 2, 3)
// or a string ("P0", "critical", etc.).
func normalizePriority(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	s := strings.Trim(string(raw), `"`)
	switch s {
	case "0":
		return "P0"
	case "1":
		return "P1"
	case "2":
		return "P2"
	case "3":
		return "P3"
	default:
		return s // already a string ("P0", "critical", "high", …)
	}
}

// isReadyStatus returns true if the status indicates a ticket is ready to work on.
func isReadyStatus(status string) bool {
	s := strings.ToLower(status)
	switch s {
	case "open", "ready", "todo", "to_do", "backlog":
		return true
	default:
		return false
	}
}

// IsInProgressStatus returns true if the status indicates a ticket is currently being worked on.
func IsInProgressStatus(status string) bool {
	s := strings.ToLower(status)
	switch s {
	case "in_progress", "in-progress", "doing", "wip":
		return true
	default:
		return false
	}
}

// IsClosedStatus reports whether a ticket status means the work is over.
func IsClosedStatus(status string) bool {
	switch strings.ToLower(status) {
	case "closed", "done", "resolved":
		return true
	}
	return false
}

// isDevPickableStatus returns true if the status indicates a ticket that can be
// selected in the dev picker — either ready to start (todo) or resumable (in_progress).
func isDevPickableStatus(status string) bool {
	return isReadyStatus(status) || IsInProgressStatus(status)
}

// IsPickableStatus reports whether a ticket can be picked for a session
// (ready or in progress).
func IsPickableStatus(status string) bool { return isDevPickableStatus(status) }

// hasLabel checks if a ticket has a specific label.
func hasLabel(t Ticket, label string) bool {
	for _, l := range t.Labels {
		if strings.EqualFold(l, label) {
			return true
		}
	}
	return false
}

// HasLabelExported checks if a ticket has a specific label (exported for use in cmd/).
func HasLabelExported(t Ticket, label string) bool {
	return hasLabel(t, label)
}

// CreateFromGitLab creates a bead ticket with a GitLab reference in the title
// and stores the structured external_ref for cross-board linking (ADR-032).
// Convention: title is prefixed with [GITLAB-REF] for correlation.
// Runs: bd -C <path> create "[<ref>] <title>" -p <priority> --external-ref gitlab-<ref>
func CreateFromGitLab(projectPath, gitlabRef, title string, priority int) (string, error) {
	fullTitle := fmt.Sprintf("[%s] %s", gitlabRef, title)
	extRef := BuildExternalRef(ProviderGitLab, gitlabRef)
	args := []string{"-C", projectPath, "create", fullTitle, "-p", fmt.Sprintf("%d", priority)}
	if extRef != "" {
		args = append(args, "--external-ref", extRef)
	}
	args = append(args, "--json")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := BdCommand(ctx, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("bd create failed: %s: %w", strings.TrimSpace(string(output)), err)
	}

	// Parse the created ticket ID from JSON output
	var created struct {
		ID string `json:"id"`
	}
	if unmarshalErr := json.Unmarshal(output, &created); unmarshalErr != nil {
		// Fallback: raw output is valid enough as an ID when JSON parsing fails.
		return strings.TrimSpace(string(output)), nil //nolint:nilerr // intentional fallback
	}
	return created.ID, nil
}

// CreateSubtask creates a child bead linked to a parent.
// Runs: bd -C <path> create "<title>" -p <priority> then bd -C <path> dep add <child> <parent>
func CreateSubtask(projectPath, parentID, title string, priority int) (string, error) {
	// Create the ticket
	args := []string{"-C", projectPath, "create", title, "-p", fmt.Sprintf("%d", priority), "--json"}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := BdCommand(ctx, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("bd create subtask failed: %s: %w", strings.TrimSpace(string(output)), err)
	}

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(output, &created); err != nil {
		return "", fmt.Errorf("parsing created ticket: %w", err)
	}

	// Link as dependency
	depArgs := []string{"-C", projectPath, "dep", "add", created.ID, parentID}
	depCtx, depCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer depCancel()
	depCmd := BdCommand(depCtx, depArgs...)
	if depOut, err := depCmd.CombinedOutput(); err != nil {
		return created.ID, fmt.Errorf("bd dep add failed: %s: %w", strings.TrimSpace(string(depOut)), err)
	}

	return created.ID, nil
}

// RememberGitLabContext stores a persistent memory associating the session with a GitLab ticket.
// Runs: bd -C <path> remember "<message>"
func RememberGitLabContext(projectPath, gitlabRef, title, description string) error {
	msg := fmt.Sprintf("Working on GitLab ticket %s: %s", gitlabRef, title)
	if description != "" {
		// Truncate description to keep it concise
		if len(description) > 200 {
			description = description[:200] + "..."
		}
		msg += "\nContext: " + description
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := BdCommand(ctx, "-C", projectPath, "remember", msg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd remember failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// AddNote adds a note to an existing bead ticket.
// Runs: bd -C <path> update <ticketID> --note "<note>"
func AddNote(projectPath, ticketID, note string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := BdCommand(ctx, "-C", projectPath, "update", ticketID, "--note", note)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd update note failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// ClaimTicket atomically claims a bead ticket (sets assignee + in_progress).
// Runs: bd -C <path> update <ticketID> --claim
func ClaimTicket(projectPath, ticketID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := BdCommand(ctx, "-C", projectPath, "update", ticketID, "--claim")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd claim failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// CloseTicket closes a bead ticket with a message.
// Runs: bd -C <path> close <ticketID> "<message>"
func CloseTicket(projectPath, ticketID, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := BdCommand(ctx, "-C", projectPath, "close", ticketID, message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd close failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// LinkToTracker sets the external_ref on a bead ticket to link it to an
// external tracker ticket (ADR-032).
// Runs: bd -C <path> update <ticketID> --external-ref <ref>
func LinkToTracker(projectPath, ticketID, externalRef string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := BdCommand(ctx, "-C", projectPath, "update", ticketID, "--external-ref", externalRef)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd link failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Zero-impact sanitization
// ---------------------------------------------------------------------------

// beadsSectionRe matches a BEADS INTEGRATION section in a git hook file.
// It captures everything from the BEGIN marker to the END marker (inclusive),
// plus any trailing blank line.
var beadsSectionRe = regexp.MustCompile(
	`(?ms)^[^\S\n]*# --- BEGIN BEADS INTEGRATION[^\n]*---\n.*?# --- END BEADS INTEGRATION[^\n]*---[^\S\n]*\n?`,
)

// beadsHookNames are the git hooks that bd may install.
var beadsHookNames = []string{
	"post-checkout",
	"post-merge",
	"pre-commit",
	"pre-push",
	"prepare-commit-msg",
}

// beadsGitignorePatterns are the entries bd adds to .gitignore.
var beadsGitignorePatterns = []string{".beads/", ".beads"}

// beadsAgentFiles are files that bd may generate in the project root.
var beadsAgentFiles = []string{"AGENTS.md"}

// beadsAgentDirs are directories that bd may generate in the project root.
var beadsAgentDirs = []string{".claude", ".codex", ".agents"}

// beadsDiagAgentFiles and beadsDiagAgentDirs are what DiagnoseBeadsImpact
// reports: the files of a `bd init` without --skip-agents (bd 1.3). They are
// wider than the lists SanitizeBeadsInit removes on its own (a CLAUDE.md or a
// .cursor/ folder of the user is never deleted, only reported).
var (
	beadsDiagAgentFiles = []string{"AGENTS.md", "CLAUDE.md"}
	beadsDiagAgentDirs  = []string{".claude", ".codex", ".agents", ".cursor"}
)

// beadsManagedBlock opens the block bd writes in AGENTS.md / CLAUDE.md.
const beadsManagedBlock = "<!-- BEGIN BEADS INTEGRATION"

// beadsDirMarkers are written by bd in the agent tool folders (hooks,
// rules, skills).
var beadsDirMarkers = []string{"bd prime", "bd cursor-hook", "bd codex-hook", beadsManagedBlock}

// hasBeadsGeneratedFile reports a file written by bd in an agent tool folder
// (searched two levels deep).
func hasBeadsGeneratedFile(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found {
			return nil //nolint:nilerr // unreadable entry: skipped
		}
		rel, _ := filepath.Rel(dir, path)
		if d.IsDir() {
			if strings.Count(rel, string(filepath.Separator)) >= 3 {
				return filepath.SkipDir
			}
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil //nolint:nilerr // unreadable file: skipped
		}
		text := string(content)
		if isBeadsAgentContent(text) {
			found = true
			return filepath.SkipAll
		}
		for _, m := range beadsDirMarkers {
			if strings.Contains(text, m) {
				found = true
				return filepath.SkipAll
			}
		}
		return nil
	})
	return found
}

// SanitizeReport describes the actions taken by SanitizeBeadsInit.
type SanitizeReport struct {
	HooksCleaned   []string // hook file names that were cleaned or removed
	GitignoreMoved []string // patterns moved from .gitignore to .git/info/exclude
	AgentsRemoved  []string // agent files/dirs removed
}

// HasChanges reports whether any sanitization was performed.
func (r SanitizeReport) HasChanges() bool {
	return len(r.HooksCleaned) > 0 || len(r.GitignoreMoved) > 0 || len(r.AgentsRemoved) > 0
}

// SanitizeBeadsInit detects and reverses side effects of a bd init that was run
// without --skip-hooks, --skip-agents, --setup-exclude. It is safe to call even
// if no side effects exist (idempotent).
//
// Actions performed:
//   - Removes BEADS INTEGRATION sections from git hooks (preserving non-beads content).
//   - Moves .beads/ entries from .gitignore into .git/info/exclude.
//   - Removes agent files (AGENTS.md) and directories (.claude/, .codex/, .agents/)
//     that were generated by bd.
func SanitizeBeadsInit(projectPath string) error {
	sanitizeHooks(projectPath)
	if err := sanitizeGitignore(projectPath); err != nil {
		return err
	}
	sanitizeAgentFiles(projectPath)
	return nil
}

// SanitizeBeadsInitReport is like SanitizeBeadsInit but returns a detailed report.
func SanitizeBeadsInitReport(projectPath string) SanitizeReport {
	var report SanitizeReport
	report.HooksCleaned = sanitizeHooks(projectPath)
	moved, _ := sanitizeGitignoreReport(projectPath)
	report.GitignoreMoved = moved
	report.AgentsRemoved = sanitizeAgentFiles(projectPath)
	return report
}

// sanitizeHooks removes BEADS INTEGRATION sections from git hooks.
// If a hook file becomes empty (only shebang + whitespace), the file is deleted.
// Returns the list of hook names that were modified or removed.
func sanitizeHooks(projectPath string) []string {
	hooksDir := filepath.Join(projectPath, ".git", "hooks")
	var cleaned []string

	for _, name := range beadsHookNames {
		hookPath := filepath.Join(hooksDir, name)
		content, err := os.ReadFile(hookPath)
		if err != nil {
			continue // hook doesn't exist — nothing to do
		}

		original := string(content)
		if !beadsSectionRe.MatchString(original) {
			continue // no beads section in this hook
		}

		result := removeBeadsSection(original)
		if isEmptyHook(result) {
			// The hook was entirely beads-generated — remove the file
			_ = os.Remove(hookPath)
			cleaned = append(cleaned, name)
		} else {
			// Preserve non-beads content
			_ = os.WriteFile(hookPath, []byte(result), 0o755)
			cleaned = append(cleaned, name)
		}
	}
	return cleaned
}

// removeBeadsSection strips all BEADS INTEGRATION sections from hook content.
// It also collapses runs of 3+ blank lines that may remain after removal.
func removeBeadsSection(content string) string {
	result := beadsSectionRe.ReplaceAllString(content, "")
	// Collapse excessive blank lines left behind (3+ → 2)
	result = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result, "\n\n")
	return result
}

// isEmptyHook reports whether a hook file contains only a shebang line and whitespace.
func isEmptyHook(content string) bool {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#!") {
			continue
		}
		// A non-empty, non-shebang line means there is real content
		return false
	}
	return true
}

// sanitizeGitignore moves .beads/ entries from .gitignore to .git/info/exclude.
func sanitizeGitignore(projectPath string) error {
	_, err := sanitizeGitignoreReport(projectPath)
	return err
}

// sanitizeGitignoreReport moves .beads/ entries from .gitignore to .git/info/exclude
// and returns the list of patterns that were moved.
func sanitizeGitignoreReport(projectPath string) ([]string, error) {
	gitignorePath := filepath.Join(projectPath, ".gitignore")
	excludePath := filepath.Join(projectPath, ".git", "info", "exclude")

	content, err := os.ReadFile(gitignorePath)
	if err != nil {
		return nil, nil //nolint:nilerr // no .gitignore — nothing to do
	}

	lines := strings.Split(string(content), "\n")
	var cleanedLines []string
	var movedPatterns []string
	// Track whether we're inside a beads-added comment block
	skipNextBlank := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect beads-related comment lines (e.g. "# beads", "# Added by bd")
		if isBeadsCommentLine(trimmed) {
			skipNextBlank = true
			continue
		}

		if isBeadsGitignoreLine(trimmed) {
			movedPatterns = append(movedPatterns, trimmed)
			skipNextBlank = true
			continue
		}

		// Skip a blank line immediately following a removed beads block
		if skipNextBlank && trimmed == "" {
			skipNextBlank = false
			continue
		}
		skipNextBlank = false

		cleanedLines = append(cleanedLines, line)
	}

	if len(movedPatterns) == 0 {
		return nil, nil
	}

	// 1. Add patterns to .git/info/exclude (avoiding duplicates)
	if err := addToExclude(excludePath, movedPatterns); err != nil {
		return movedPatterns, fmt.Errorf("adding to git exclude: %w", err)
	}

	// 2. Rewrite .gitignore without the beads lines
	newContent := strings.Join(cleanedLines, "\n")
	// Ensure file ends with a single newline
	newContent = strings.TrimRight(newContent, "\n") + "\n"
	if err := os.WriteFile(gitignorePath, []byte(newContent), 0o644); err != nil {
		return movedPatterns, fmt.Errorf("rewriting .gitignore: %w", err)
	}

	return movedPatterns, nil
}

// isBeadsGitignoreLine reports whether a trimmed .gitignore line is a beads entry.
func isBeadsGitignoreLine(trimmed string) bool {
	for _, p := range beadsGitignorePatterns {
		if trimmed == p {
			return true
		}
	}
	return false
}

// isBeadsCommentLine reports whether a trimmed line is a comment added by bd.
func isBeadsCommentLine(trimmed string) bool {
	if !strings.HasPrefix(trimmed, "#") {
		return false
	}
	lower := strings.ToLower(trimmed)
	return strings.Contains(lower, "beads") || strings.Contains(lower, "added by bd")
}

// addToExclude appends patterns to .git/info/exclude, skipping duplicates.
func addToExclude(excludePath string, patterns []string) error {
	_ = os.MkdirAll(filepath.Dir(excludePath), 0o755)

	existing, _ := os.ReadFile(excludePath)
	content := string(existing)

	var toAdd []string
	for _, p := range patterns {
		if !strings.Contains(content, p) {
			toAdd = append(toAdd, p)
		}
	}
	if len(toAdd) == 0 {
		return nil
	}

	f, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	if len(existing) > 0 && !strings.HasSuffix(content, "\n") {
		_, _ = f.WriteString("\n")
	}
	_, _ = f.WriteString("\n# beads — moved from .gitignore by oh\n")
	for _, p := range toAdd {
		_, _ = f.WriteString(p + "\n")
	}
	return nil
}

// sanitizeAgentFiles removes agent instruction files and directories generated by bd.
// Only removes files/dirs that contain beads-specific content (to avoid removing
// user-created files with the same name).
// Returns the list of removed file/dir names.
func sanitizeAgentFiles(projectPath string) []string {
	var removed []string

	for _, name := range beadsAgentFiles {
		p := filepath.Join(projectPath, name)
		content, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if isBeadsAgentContent(string(content)) {
			if os.Remove(p) == nil {
				removed = append(removed, name)
			}
		}
	}

	for _, name := range beadsAgentDirs {
		p := filepath.Join(projectPath, name)
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			continue
		}
		if isBeadsAgentDir(p) {
			if os.RemoveAll(p) == nil {
				removed = append(removed, name+"/")
			}
		}
	}

	return removed
}

// isBeadsAgentContent reports whether a file's content was generated by bd.
// We look for distinctive beads markers in the content.
func isBeadsAgentContent(content string) bool {
	lower := strings.ToLower(content)
	markers := []string{
		"beads",
		"bd prime",
		"bd ready",
		"bd show",
		"bd close",
		"bd update",
		"bd create",
	}
	matches := 0
	for _, m := range markers {
		if strings.Contains(lower, m) {
			matches++
		}
	}
	// Require at least 2 distinct beads markers to avoid false positives
	return matches >= 2
}

// isBeadsAgentDir reports whether a directory was generated by bd.
// We check if it contains files with beads-specific content.
func isBeadsAgentDir(dirPath string) bool {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dirPath, e.Name()))
		if err != nil {
			continue
		}
		if isBeadsAgentContent(string(content)) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Diagnostics (used by oh doctor)
// ---------------------------------------------------------------------------

// DiagIssue describes a single zero-impact violation found in a project.
type DiagIssue struct {
	Kind   string // "hook", "gitignore", "exclude_missing", "agent_file"
	Detail string // human-readable description
}

// DiagnoseBeadsImpact checks a project for beads side effects that should not be present.
// Returns nil when everything is clean.
func DiagnoseBeadsImpact(projectPath string) []DiagIssue {
	if !IsInitialized(projectPath) {
		return nil
	}

	var issues []DiagIssue

	// Check hooks for BEADS INTEGRATION sections
	hooksDir := filepath.Join(projectPath, ".git", "hooks")
	for _, name := range beadsHookNames {
		content, err := os.ReadFile(filepath.Join(hooksDir, name))
		if err != nil {
			continue
		}
		if beadsSectionRe.MatchString(string(content)) {
			issues = append(issues, DiagIssue{
				Kind:   "hook",
				Detail: fmt.Sprintf("hook %s contains BEADS INTEGRATION section", name),
			})
		}
	}

	// Check .gitignore for beads entries (and the block bd 1.3 adds under
	// its own comment line)
	gitignorePath := filepath.Join(projectPath, ".gitignore")
	if content, err := os.ReadFile(gitignorePath); err == nil {
		for _, line := range strings.Split(string(content), "\n") {
			trimmed := strings.TrimSpace(line)
			switch {
			case isBeadsGitignoreLine(trimmed):
				issues = append(issues, DiagIssue{
					Kind:   "gitignore",
					Detail: fmt.Sprintf(".gitignore contains beads entry: %s", trimmed),
				})
			case strings.Contains(strings.ToLower(trimmed), "added by bd init") && isBeadsCommentLine(trimmed):
				issues = append(issues, DiagIssue{
					Kind:   "gitignore",
					Detail: fmt.Sprintf(".gitignore contains a block added by bd init: %s", trimmed),
				})
			}
		}
	}

	// Check that .git/info/exclude has .beads/
	excludePath := filepath.Join(projectPath, ".git", "info", "exclude")
	if content, err := os.ReadFile(excludePath); err != nil || !strings.Contains(string(content), ".beads") {
		issues = append(issues, DiagIssue{
			Kind:   "exclude_missing",
			Detail: ".git/info/exclude does not contain .beads/ entry",
		})
	}

	// Check for agent files (bd 1.3 also writes CLAUDE.md and .cursor/)
	for _, name := range beadsDiagAgentFiles {
		p := filepath.Join(projectPath, name)
		if content, err := os.ReadFile(p); err == nil && (isBeadsAgentContent(string(content)) || strings.Contains(string(content), beadsManagedBlock)) {
			issues = append(issues, DiagIssue{
				Kind:   "agent_file",
				Detail: fmt.Sprintf("bd-generated agent file present: %s", name),
			})
		}
	}
	for _, name := range beadsDiagAgentDirs {
		p := filepath.Join(projectPath, name)
		if info, err := os.Stat(p); err == nil && info.IsDir() && hasBeadsGeneratedFile(p) {
			issues = append(issues, DiagIssue{
				Kind:   "agent_file",
				Detail: fmt.Sprintf("bd-generated agent directory present: %s/", name),
			})
		}
	}

	return issues
}
