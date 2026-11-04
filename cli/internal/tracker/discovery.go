package tracker

import (
	"strings"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

// SuggestedMapping holds a proposed label→column or status→column mapping.
type SuggestedMapping struct {
	// Source is the tracker label or status name (e.g., "READY TO DEV", "In Progress").
	Source string
	// ColumnID is the proposed value for the mapping: the claim status of the
	// column (e.g. "planned", "in_progress"), which is the ID of the column
	// when the board has its own columns. "todo" is the ID of the initial
	// column of the default board, but not a claim status: on a board without
	// columns of its own the initial column is "planned".
	ColumnID string
	// Confidence is "high" (pattern match) or "medium" (category fallback).
	Confidence string
	// Reason explains why this mapping was suggested.
	Reason string
	// Origin is "label" or "status" — where the mapping came from.
	Origin string
}

// ── Heuristic pattern table ─────────────────────────────────────────────────

// mappingRule maps substring patterns to a board column ID.
type mappingRule struct {
	patterns []string
	columnID string
	role     string // expected column role
}

// heuristicRules maps label/status name patterns to board columns.
// columnID is the ID of the column of the default board; the value a mapping
// gets is the claim status of the column (see buildColumnLookup).
// Patterns are matched as lowercased substrings.
// Order matters: first match wins.
var heuristicRules = []mappingRule{
	{
		patterns: []string{
			"ready to dev", "to do", "todo", "to_do", "backlog",
			"à faire", "a faire", "open", "prêt",
		},
		columnID: "todo",
		role:     teamstate.ColumnRoleInitial,
	},
	{
		patterns: []string{
			"doing", "in progress", "in_progress", "dev doing",
			"en cours", "development", "in dev", "coding", "wip",
		},
		columnID: "in_progress",
		role:     teamstate.ColumnRoleActive,
	},
	{
		patterns: []string{
			"review", "to review", "dev to review", "code review",
			"merge request", "pull request", "revue",
		},
		columnID: "review",
		role:     teamstate.ColumnRoleActive,
	},
	{
		patterns: []string{
			"test", "testing", "to test", "qa", "validation",
			"recette", "staging", "to mep", "mep", "preprod",
			"pre-prod", "homologation",
		},
		columnID: "validation",
		role:     teamstate.ColumnRoleActive,
	},
	{
		patterns: []string{
			"block", "bloqué", "bloque", "blocked", "impediment",
			"waiting", "on hold", "en attente",
		},
		columnID: "blocked",
		role:     teamstate.ColumnRoleBlocked,
	},
	{
		patterns: []string{
			"done", "closed", "terminé", "termine", "resolved",
			"released", "deployed", "livré", "livre", "completed",
		},
		columnID: "done",
		role:     teamstate.ColumnRoleTerminal,
	},
}

// Jira category → column role mapping (used as base, refined by heuristics).
var jiraCategoryMapping = map[string]string{
	"new":           "todo", // the initial column, see buildColumnLookup
	"indeterminate": "in_progress",
	"done":          "done",
}

// ── Public API ──────────────────────────────────────────────────────────────

// SuggestMappings proposes label→column or status→column mappings based on
// heuristic pattern matching against discovered project metadata.
//
// For GitLab: matches label names against the heuristic table.
// For Jira: uses statusCategory as base, then refines with heuristics.
//
// Each mapping gets the claim status of its column (see columnLookup), never
// a value the board does not have.
//
// boardColumns allows mapping to custom column IDs when the user has
// already configured their board. If empty, default column IDs are used.
func SuggestMappings(info *DiscoveryInfo, trackerType Type, boardColumns []teamstate.BoardColumnConfig) []SuggestedMapping {
	if info == nil {
		return nil
	}

	// Build a column lookup to match heuristic columnIDs to actual board columns.
	colLookup := buildColumnLookup(boardColumns)

	var mappings []SuggestedMapping

	// Labels (GitLab workflow labels, Jira labels).
	for _, label := range info.Labels {
		if m, ok := matchHeuristic(label.Name, colLookup); ok {
			m.Origin = "label"
			mappings = append(mappings, m)
		}
	}

	// Statuses (Jira only).
	for _, status := range info.Statuses {
		if m, ok := matchStatus(status, colLookup); ok {
			m.Origin = "status"
			mappings = append(mappings, m)
		}
	}

	// A label whose column the board does not have (no blocked column, say) is
	// left unmapped rather than mapped to a status that does not exist.
	kept, _ := RetainMappings(mappings, teamstate.BoardConfig{Columns: boardColumns})
	return kept
}

// SuggestColumnRole guesses the semantic role of a column from its name.
// Used by the wizard when creating new columns to pre-assign roles.
func SuggestColumnRole(name string) string {
	lower := strings.ToLower(name)
	for _, rule := range heuristicRules {
		for _, p := range rule.patterns {
			if strings.Contains(lower, p) {
				return rule.role
			}
		}
	}
	return teamstate.ColumnRoleActive // default
}

// SuggestPoolLabels returns label names that are mapped to the initial column.
// These are candidates for UnassignedLabels (tickets ready to be picked up).
func SuggestPoolLabels(mappings []SuggestedMapping, boardCfg teamstate.BoardConfig) []string {
	initialStatus := boardCfg.InitialStatus()
	var labels []string
	for _, m := range mappings {
		// "planned" is the initial column whatever the ID the board gives it.
		if (m.ColumnID == initialStatus || m.ColumnID == teamstate.ClaimStatusPlanned) && m.Origin == "label" {
			labels = append(labels, m.Source)
		}
	}
	return labels
}

// UnmappedItems returns label/status names that didn't match any heuristic.
func UnmappedItems(info *DiscoveryInfo, mappings []SuggestedMapping) []string {
	if info == nil {
		return nil
	}
	mapped := make(map[string]bool, len(mappings))
	for _, m := range mappings {
		mapped[m.Source] = true
	}
	var result []string
	for _, label := range info.Labels {
		if !mapped[label.Name] {
			result = append(result, label.Name)
		}
	}
	for _, status := range info.Statuses {
		if !mapped[status.Name] {
			result = append(result, status.Name)
		}
	}
	return result
}

// SuggestNewColumns proposes new board columns for tracker labels/statuses
// that don't match any existing column. Used by the wizard to enrich the
// board layout based on the tracker's workflow.
func SuggestNewColumns(info *DiscoveryInfo, existing []teamstate.BoardColumnConfig) []teamstate.BoardColumnConfig {
	if info == nil {
		return nil
	}
	// Build set of existing column IDs.
	existingIDs := make(map[string]bool, len(existing))
	for _, c := range existing {
		existingIDs[c.ID] = true
	}

	// A board has at most one initial column: whatever its ID, it is the one
	// that "todo" labels go to, so none is proposed when there is one.
	hasInitial := false
	for _, c := range existing {
		hasInitial = hasInitial || c.Role == teamstate.ColumnRoleInitial
	}

	// Find heuristic-matched items whose target column doesn't exist.
	suggested := make(map[string]teamstate.BoardColumnConfig)
	for _, label := range info.Labels {
		proposeColumn(label.Name, existingIDs, hasInitial, suggested)
	}
	for _, status := range info.Statuses {
		proposeColumn(status.Name, existingIDs, hasInitial, suggested)
	}

	var result []teamstate.BoardColumnConfig
	for _, c := range suggested {
		result = append(result, c)
	}
	return result
}

// ── Internal helpers ────────────────────────────────────────────────────────

// columnLookup maps the default column IDs of the heuristic table to the
// value a mapping gets on the board: the claim status of the column, which is
// the ID of the column when the board has its own columns.
//
// The initial column is "planned" on a board without columns of its own:
// "todo" is the ID of the initial column of the default board, not a claim
// status, and a mapping to it was read as in progress. On a board with its own
// columns it is the ID of the column that has the role initial (the first
// column when none has it).
type columnLookup map[string]string

func buildColumnLookup(boardColumns []teamstate.BoardColumnConfig) columnLookup {
	board := teamstate.BoardConfig{Columns: boardColumns}
	lookup := make(columnLookup)
	if len(boardColumns) == 0 {
		// The default board: its columns are the statuses, but for the
		// initial one.
		for _, rule := range heuristicRules {
			lookup[rule.columnID] = rule.columnID
			if rule.role == teamstate.ColumnRoleInitial {
				lookup[rule.columnID] = board.InitialStatus()
			}
		}
		return lookup
	}
	// Map each role to the first matching board column.
	roleToCol := make(map[string]string)
	for _, c := range boardColumns {
		role := c.Role
		if role == "" {
			role = teamstate.ColumnRoleActive
		}
		if _, ok := roleToCol[role]; !ok {
			roleToCol[role] = c.ID
		}
	}
	// Map heuristic columnIDs to board columns via roles.
	for _, rule := range heuristicRules {
		switch col, ok := roleToCol[rule.role]; {
		case ok:
			lookup[rule.columnID] = col
		case rule.role == teamstate.ColumnRoleInitial:
			lookup[rule.columnID] = board.InitialStatus()
		default:
			lookup[rule.columnID] = rule.columnID
		}
	}
	// Also allow direct ID match for custom columns.
	for _, c := range boardColumns {
		lookup[c.ID] = c.ID
	}
	return lookup
}

// RetainMappings keeps the suggested mappings whose value is a status of the
// board, with the value written as the status (the ID of a column, or the
// initial status for "todo"), and returns the others apart: a mapping to a
// column the team left out of its board would be refused when the config is
// written, and ignored by the sync.
func RetainMappings(mappings []SuggestedMapping, board teamstate.BoardConfig) (kept, dropped []SuggestedMapping) {
	for _, m := range mappings {
		status, _, ok := board.ResolveStatus(m.ColumnID)
		if !ok {
			dropped = append(dropped, m)
			continue
		}
		m.ColumnID = status
		kept = append(kept, m)
	}
	return kept, dropped
}

func matchHeuristic(name string, lookup columnLookup) (SuggestedMapping, bool) {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, rule := range heuristicRules {
		for _, p := range rule.patterns {
			if strings.Contains(lower, p) || lower == p {
				colID := rule.columnID
				if mapped, ok := lookup[colID]; ok {
					colID = mapped
				}
				return SuggestedMapping{
					Source:     name,
					ColumnID:   colID,
					Confidence: "high",
					Reason:     "matches pattern \"" + p + "\"",
				}, true
			}
		}
	}
	return SuggestedMapping{}, false
}

func matchStatus(status StatusInfo, lookup columnLookup) (SuggestedMapping, bool) {
	// First try heuristic name matching (higher precision).
	if m, ok := matchHeuristic(status.Name, lookup); ok {
		return m, true
	}
	// Fall back to Jira category mapping.
	if status.Category != "" {
		if defaultCol, ok := jiraCategoryMapping[status.Category]; ok {
			colID := defaultCol
			if mapped, ok := lookup[colID]; ok {
				colID = mapped
			}
			return SuggestedMapping{
				Source:     status.Name,
				ColumnID:   colID,
				Confidence: "medium",
				Reason:     "Jira category \"" + status.Category + "\"",
			}, true
		}
	}
	return SuggestedMapping{}, false
}

func proposeColumn(name string, existingIDs map[string]bool, hasInitial bool, suggested map[string]teamstate.BoardColumnConfig) {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, rule := range heuristicRules {
		for _, p := range rule.patterns {
			if strings.Contains(lower, p) {
				if rule.role == teamstate.ColumnRoleInitial && hasInitial {
					return
				}
				if !existingIDs[rule.columnID] && suggested[rule.columnID].ID == "" {
					suggested[rule.columnID] = teamstate.BoardColumnConfig{
						ID:   rule.columnID,
						Name: strings.ToUpper(rule.columnID),
						Role: rule.role,
					}
				}
				return
			}
		}
	}
}
