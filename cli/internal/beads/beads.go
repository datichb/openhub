// Package beads provides an interface to the bd (Beads) CLI for ticket management.
package beads

import (
	"encoding/json"
	"fmt"
	"os/exec"
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

// SyncPull runs: bd -C <path> <tracker> sync pull
// Synchronizes tickets from the remote tracker.
func SyncPull(projectPath, tracker string) error {
	if tracker == "" || tracker == "none" {
		return nil // no tracker configured, skip silently
	}
	cmd := exec.Command("bd", "-C", projectPath, tracker, "sync", "pull")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bd sync pull failed: %s: %w", strings.TrimSpace(string(output)), err)
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
// Runs: bd -C <path> list --json --no-tree
func ListAll(projectPath string) ([]Ticket, error) {
	args := []string{"-C", projectPath, "list", "--json", "--no-tree"}
	return runBdJSON(args)
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

	var tickets []Ticket
	if err := json.Unmarshal([]byte(trimmed), &tickets); err != nil {
		return nil, fmt.Errorf("parsing bd output: %w", err)
	}
	return tickets, nil
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
	if err := json.Unmarshal(output, &created); err != nil {
		// Fallback: return raw output trimmed
		return strings.TrimSpace(string(output)), nil
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
