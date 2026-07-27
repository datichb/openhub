// Package beads provides an interface to the bd (Beads) CLI for ticket management.
package beads

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Ticket represents a bd ticket.
type Ticket struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Status   string   `json:"status"`
	Priority string   `json:"priority"`
	Type     string   `json:"type"`
	Parent   string   `json:"parent,omitempty"`
	Labels   []string `json:"labels,omitempty"`
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

// IsInitialized reports whether the project at projectPath has a .beads/ directory.
func IsInitialized(projectPath string) bool {
	_, err := os.Stat(filepath.Join(projectPath, ".beads"))
	return err == nil
}

// Init initializes beads in the given project directory.
// prefix is used for ticket ID prefixes (e.g. project ID or short name).
// It also registers the default labels used by opencode agents.
func Init(projectPath, prefix string) error {
	if err := Available(); err != nil {
		return err
	}
	cmd := exec.Command("bd", "-C", projectPath, "init",
		"--prefix", prefix,
		"--skip-hooks", "--skip-agents", "--setup-exclude")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("bd init: %s", strings.TrimSpace(string(out)))
	}
	// Register default labels used by opencode agents
	for _, label := range []string{"ai-delegated", "feature", "fix"} {
		exec.Command("bd", "-C", projectPath, "label", "create", label).Run() //nolint:errcheck
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
	cmd := exec.Command("bd", "-C", projectPath, "show", ticketID, "--json")
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
	cmd := exec.Command("bd", args...)
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
		ID       string          `json:"id"`
		Title    string          `json:"title"`
		Status   string          `json:"status"`
		Priority json.RawMessage `json:"priority"`
		Type     string          `json:"issue_type"`
		Parent   string          `json:"parent,omitempty"`
		Labels   []string        `json:"labels,omitempty"`
	}
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return nil, fmt.Errorf("parsing bd output: %w", err)
	}

	tickets := make([]Ticket, len(raw))
	for i, r := range raw {
		tickets[i] = Ticket{
			ID:       r.ID,
			Title:    r.Title,
			Status:   r.Status,
			Priority: normalizePriority(r.Priority),
			Type:     r.Type,
			Parent:   r.Parent,
			Labels:   r.Labels,
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

// CreateFromGitLab creates a bead ticket with a GitLab reference in the title.
// Convention: title is prefixed with [GITLAB-REF] for correlation.
// Runs: bd -C <path> create "[<ref>] <title>" -p <priority>
func CreateFromGitLab(projectPath, gitlabRef, title string, priority int) (string, error) {
	fullTitle := fmt.Sprintf("[%s] %s", gitlabRef, title)
	args := []string{"-C", projectPath, "create", fullTitle, "-p", fmt.Sprintf("%d", priority), "--json"}

	cmd := exec.Command("bd", args...)
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
	cmd := exec.Command("bd", args...)
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
	depCmd := exec.Command("bd", depArgs...)
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

	cmd := exec.Command("bd", "-C", projectPath, "remember", msg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd remember failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// AddNote adds a note to an existing bead ticket.
// Runs: bd -C <path> update <ticketID> --note "<note>"
func AddNote(projectPath, ticketID, note string) error {
	cmd := exec.Command("bd", "-C", projectPath, "update", ticketID, "--note", note)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd update note failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// ClaimTicket atomically claims a bead ticket (sets assignee + in_progress).
// Runs: bd -C <path> update <ticketID> --claim
func ClaimTicket(projectPath, ticketID string) error {
	cmd := exec.Command("bd", "-C", projectPath, "update", ticketID, "--claim")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd claim failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

// CloseTicket closes a bead ticket with a message.
// Runs: bd -C <path> close <ticketID> "<message>"
func CloseTicket(projectPath, ticketID, message string) error {
	cmd := exec.Command("bd", "-C", projectPath, "close", ticketID, message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd close failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}
