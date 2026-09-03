// Package shell — command registry for the omnibar-first TUI.
// Commands are the single abstraction for all user-triggerable actions.
package shell

import (
	"sort"
	"strings"
	"sync"
)

// SessionLaunchConfig holds the options for a session launch dialog.
type SessionLaunchConfig struct {
	// Title is the dialog title (e.g., "Lancer une session").
	Title string
	// Options is the list of selectable session variants.
	Options []SessionOption
	// OnLaunch is called with the selected option when the user confirms.
	OnLaunch func(selected SessionOption)
}

// SessionOption represents a single launchable session variant.
type SessionOption struct {
	Label       string   // Display label
	Description string   // One-line description
	Agent       string   // opencode --agent value
	ExtraArgs   []string // Additional CLI arguments
}

// Command represents a user-triggerable action accessible via the omnibar.
type Command struct {
	// ID is the canonical identifier (e.g., "start", "audit.security", "board").
	ID string
	// Label is the display text in the omnibar suggestions.
	Label string
	// Aliases are additional fuzzy-match terms (e.g., "code", "session" for "start").
	Aliases []string
	// Description is a one-line help text shown in suggestions.
	Description string
	// Category groups commands visually in the suggestion list.
	Category string
	// ViewID navigates to a registered view when executed. Mutually exclusive with Action.
	ViewID string
	// Action is called when the command is executed. Mutually exclusive with ViewID.
	Action func()
	// Enabled returns whether this command is currently available. Nil means always enabled.
	Enabled func() bool
	// Priority controls ordering when the omnibar query is empty.
	// Higher values appear first. Default (0) puts the command at the end.
	Priority int
	// RunsDirect marks actions that MUST execute synchronously on the tview event loop
	// and MUST NOT be deferred via QueueUpdateDraw. Set this to true for any action
	// that calls SuspendAndExec — app.Suspend() deadlocks when invoked from inside a
	// QueueUpdateDraw callback because it waits for the event loop to be idle, but the
	// loop is blocked waiting for the callback to return.
	//
	// All other actions (ShowModal, ShowToast, NavigateTo, App.Stop) should leave this
	// false so the omnibar defers them to the next draw cycle, avoiding the separate
	// deadlock caused by calling pages.AddPage inside an InputCapture handler.
	RunsDirect bool
}

// IsEnabled returns whether this command is currently available.
func (c *Command) IsEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return c.Enabled()
}

// CommandRegistry holds a flat list of commands with fuzzy-search capability.
type CommandRegistry struct {
	mu       sync.Mutex
	commands []Command
	recent   []string // last N command IDs used, most recent first
}

// NewCommandRegistry creates a registry from a list of commands.
func NewCommandRegistry(commands []Command) *CommandRegistry {
	return &CommandRegistry{commands: commands}
}

// All returns all registered commands.
func (r *CommandRegistry) All() []Command {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]Command, len(r.commands))
	copy(result, r.commands)
	return result
}

// RecordUsage records a command ID as recently used.
func (r *CommandRegistry) RecordUsage(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove if already present
	for i, rid := range r.recent {
		if rid == id {
			r.recent = append(r.recent[:i], r.recent[i+1:]...)
			break
		}
	}
	// Prepend
	r.recent = append([]string{id}, r.recent...)
	// Cap at 20
	if len(r.recent) > 20 {
		r.recent = r.recent[:20]
	}
}

// recentBonus returns a score boost for recently used commands.
// The most recent command gets +20, decreasing by 1 per position down to +1.
func (r *CommandRegistry) recentBonus(id string) int {
	for i, rid := range r.recent {
		if rid == id {
			bonus := 20 - i
			if bonus < 1 {
				bonus = 1
			}
			return bonus
		}
	}
	return 0
}

// Search returns commands matching the query, sorted by relevance.
// Matches against ID, Label, Aliases, and Category using fuzzy matching.
func (r *CommandRegistry) Search(query string) []Command {
	r.mu.Lock()
	defer r.mu.Unlock()

	if query == "" {
		// Return all enabled commands sorted by Priority descending.
		// Higher priority = shown first when omnibar is empty.
		var result []Command
		for _, c := range r.commands {
			if c.IsEnabled() {
				result = append(result, c)
			}
		}
		sort.SliceStable(result, func(i, j int) bool {
			pi := result[i].Priority + r.recentBonus(result[i].ID)
			pj := result[j].Priority + r.recentBonus(result[j].ID)
			return pi > pj
		})
		return result
	}

	query = strings.ToLower(query)
	type scored struct {
		cmd   Command
		score int
	}

	var results []scored
	for _, c := range r.commands {
		if !c.IsEnabled() {
			continue
		}

		score := matchScore(query, c)
		if score > 0 {
			score += r.recentBonus(c.ID)
			results = append(results, scored{cmd: c, score: score})
		}
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	out := make([]Command, len(results))
	for i, r := range results {
		out[i] = r.cmd
	}
	return out
}

// matchScore returns a relevance score (0 = no match).
// Higher = better match.
func matchScore(query string, c Command) int {
	id := strings.ToLower(c.ID)
	label := strings.ToLower(c.Label)
	cat := strings.ToLower(c.Category)

	// Exact ID match = highest score
	if id == query {
		return 100
	}

	// ID starts with query
	if strings.HasPrefix(id, query) {
		return 90
	}

	// Label starts with query
	if strings.HasPrefix(label, query) {
		return 80
	}

	// Exact alias match
	for _, alias := range c.Aliases {
		if strings.ToLower(alias) == query {
			return 85
		}
	}

	// Alias starts with query
	for _, alias := range c.Aliases {
		if strings.HasPrefix(strings.ToLower(alias), query) {
			return 75
		}
	}

	// Contains match (substring)
	if strings.Contains(id, query) {
		return 60
	}
	if strings.Contains(label, query) {
		return 55
	}
	for _, alias := range c.Aliases {
		if strings.Contains(strings.ToLower(alias), query) {
			return 50
		}
	}
	if strings.Contains(cat, query) {
		return 40
	}

	// Fuzzy match (chars in order)
	if fuzzyMatch(query, id) {
		return 30
	}
	if fuzzyMatch(query, label) {
		return 25
	}
	for _, alias := range c.Aliases {
		if fuzzyMatch(query, strings.ToLower(alias)) {
			return 20
		}
	}

	return 0
}

// fuzzyMatch checks if all runes in pattern appear in str in order.
func fuzzyMatch(pattern, str string) bool {
	pRunes := []rune(pattern)
	sRunes := []rune(str)
	pi := 0
	for si := 0; si < len(sRunes) && pi < len(pRunes); si++ {
		if sRunes[si] == pRunes[pi] {
			pi++
		}
	}
	return pi == len(pRunes)
}

// MatchesQuery returns true if a command matches the given query string.
// Used by the omnibar to filter contextual commands from views.
func MatchesQuery(query string, id, label string, aliases []string, category string) bool {
	if query == "" {
		return true
	}
	query = strings.ToLower(query)
	idL := strings.ToLower(id)
	labelL := strings.ToLower(label)
	catL := strings.ToLower(category)

	if strings.Contains(idL, query) || strings.Contains(labelL, query) || strings.Contains(catL, query) {
		return true
	}
	for _, alias := range aliases {
		if strings.Contains(strings.ToLower(alias), query) {
			return true
		}
	}
	if fuzzyMatch(query, idL) || fuzzyMatch(query, labelL) {
		return true
	}
	for _, alias := range aliases {
		if fuzzyMatch(query, strings.ToLower(alias)) {
			return true
		}
	}
	return false
}
