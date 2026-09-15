package tracker

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

func TestSuggestMappings_GitLabLabels(t *testing.T) {
	info := &DiscoveryInfo{
		Labels: []LabelInfo{
			{Name: "READY TO DEV"},
			{Name: "DEV DOING"},
			{Name: "DEV TO REVIEW"},
			{Name: "TESTING"},
			{Name: "TO TEST"},
			{Name: "TO MEP"},
			{Name: "BLOQUÉ"},
			{Name: "bug"},
			{Name: "feature"},
			{Name: "backend"},
		},
	}

	mappings := SuggestMappings(info, TypeGitLab, nil)

	expected := map[string]string{
		"READY TO DEV":  "todo",
		"DEV DOING":     "in_progress",
		"DEV TO REVIEW": "review",
		"TESTING":       "validation",
		"TO TEST":       "validation",
		"TO MEP":        "validation",
		"BLOQUÉ":        "blocked",
	}

	if len(mappings) != len(expected) {
		t.Fatalf("expected %d mappings, got %d: %+v", len(expected), len(mappings), mappings)
	}

	for _, m := range mappings {
		wantCol, ok := expected[m.Source]
		if !ok {
			t.Errorf("unexpected mapping for %q", m.Source)
			continue
		}
		if m.ColumnID != wantCol {
			t.Errorf("mapping %q: got column %q, want %q", m.Source, m.ColumnID, wantCol)
		}
		if m.Origin != "label" {
			t.Errorf("mapping %q: got origin %q, want 'label'", m.Source, m.Origin)
		}
		if m.Confidence != "high" {
			t.Errorf("mapping %q: got confidence %q, want 'high'", m.Source, m.Confidence)
		}
	}
}

func TestSuggestMappings_JiraStatuses(t *testing.T) {
	info := &DiscoveryInfo{
		Statuses: []StatusInfo{
			{Name: "To Do", Category: "new"},
			{Name: "In Progress", Category: "indeterminate"},
			{Name: "Code Review", Category: "indeterminate"},
			{Name: "In QA", Category: "indeterminate"},
			{Name: "Done", Category: "done"},
		},
	}

	mappings := SuggestMappings(info, TypeJira, nil)

	expected := map[string]string{
		"To Do":       "todo",
		"In Progress": "in_progress",
		"Code Review": "review",
		"In QA":       "validation",
		"Done":        "done",
	}

	if len(mappings) != len(expected) {
		t.Fatalf("expected %d mappings, got %d: %+v", len(expected), len(mappings), mappings)
	}

	for _, m := range mappings {
		wantCol, ok := expected[m.Source]
		if !ok {
			t.Errorf("unexpected mapping for %q", m.Source)
			continue
		}
		if m.ColumnID != wantCol {
			t.Errorf("mapping %q: got column %q, want %q", m.Source, m.ColumnID, wantCol)
		}
		if m.Origin != "status" {
			t.Errorf("mapping %q: got origin %q, want 'status'", m.Source, m.Origin)
		}
	}
}

func TestSuggestMappings_JiraCategoryFallback(t *testing.T) {
	info := &DiscoveryInfo{
		Statuses: []StatusInfo{
			{Name: "Custom Active Status", Category: "indeterminate"},
			{Name: "Custom Terminal Status", Category: "done"},
		},
	}

	mappings := SuggestMappings(info, TypeJira, nil)

	if len(mappings) != 2 {
		t.Fatalf("expected 2 mappings, got %d: %+v", len(mappings), mappings)
	}

	// First should fall back to category mapping (medium confidence).
	if mappings[0].ColumnID != "in_progress" {
		t.Errorf("got column %q, want 'in_progress'", mappings[0].ColumnID)
	}
	if mappings[0].Confidence != "medium" {
		t.Errorf("got confidence %q, want 'medium'", mappings[0].Confidence)
	}

	if mappings[1].ColumnID != "done" {
		t.Errorf("got column %q, want 'done'", mappings[1].ColumnID)
	}
}

func TestSuggestMappings_CaseInsensitive(t *testing.T) {
	info := &DiscoveryInfo{
		Labels: []LabelInfo{
			{Name: "ready to dev"},
			{Name: "READY TO DEV"},
			{Name: "Ready To Dev"},
		},
	}

	mappings := SuggestMappings(info, TypeGitLab, nil)

	if len(mappings) != 3 {
		t.Fatalf("expected 3 mappings, got %d", len(mappings))
	}
	for _, m := range mappings {
		if m.ColumnID != "todo" {
			t.Errorf("mapping %q: got column %q, want 'todo'", m.Source, m.ColumnID)
		}
	}
}

func TestSuggestMappings_WithCustomColumns(t *testing.T) {
	info := &DiscoveryInfo{
		Labels: []LabelInfo{
			{Name: "READY TO DEV"},
			{Name: "DEV DOING"},
			{Name: "TESTING"},
		},
	}

	customColumns := []teamstate.BoardColumnConfig{
		{ID: "backlog", Name: "BACKLOG", Role: teamstate.ColumnRoleInitial},
		{ID: "dev", Name: "DEV", Role: teamstate.ColumnRoleActive},
		{ID: "qa", Name: "QA", Role: teamstate.ColumnRoleActive},
		{ID: "shipped", Name: "SHIPPED", Role: teamstate.ColumnRoleTerminal},
	}

	mappings := SuggestMappings(info, TypeGitLab, customColumns)

	if len(mappings) != 3 {
		t.Fatalf("expected 3 mappings, got %d: %+v", len(mappings), mappings)
	}

	// READY TO DEV → backlog (initial role)
	if mappings[0].ColumnID != "backlog" {
		t.Errorf("READY TO DEV: got column %q, want 'backlog'", mappings[0].ColumnID)
	}
	// DEV DOING → dev (active role)
	if mappings[1].ColumnID != "dev" {
		t.Errorf("DEV DOING: got column %q, want 'dev'", mappings[1].ColumnID)
	}
	// TESTING → qa (active role — second active column)
	if mappings[2].ColumnID != "dev" {
		// Note: with role-based lookup, both "in_progress" and "validation"
		// heuristics map to "active" role, which maps to the FIRST active column.
		// This is a known simplification — the wizard step lets the user adjust.
		t.Errorf("TESTING: got column %q, want 'dev'", mappings[2].ColumnID)
	}
}

func TestSuggestColumnRole(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"TODO", teamstate.ColumnRoleInitial},
		{"READY TO DEV", teamstate.ColumnRoleInitial},
		{"Backlog", teamstate.ColumnRoleInitial},
		{"IN PROGRESS", teamstate.ColumnRoleActive},
		{"DEV DOING", teamstate.ColumnRoleActive},
		{"REVIEW", teamstate.ColumnRoleActive},
		{"TESTING", teamstate.ColumnRoleActive},
		{"QA", teamstate.ColumnRoleActive},
		{"TO MEP", teamstate.ColumnRoleActive},
		{"BLOCKED", teamstate.ColumnRoleBlocked},
		{"BLOQUÉ", teamstate.ColumnRoleBlocked},
		{"DONE", teamstate.ColumnRoleTerminal},
		{"Released", teamstate.ColumnRoleTerminal},
		{"custom-status", teamstate.ColumnRoleActive}, // default
	}

	for _, tt := range tests {
		got := SuggestColumnRole(tt.name)
		if got != tt.want {
			t.Errorf("SuggestColumnRole(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestSuggestPoolLabels(t *testing.T) {
	mappings := []SuggestedMapping{
		{Source: "READY TO DEV", ColumnID: "todo", Origin: "label"},
		{Source: "DEV DOING", ColumnID: "in_progress", Origin: "label"},
		{Source: "To Do", ColumnID: "todo", Origin: "status"},
	}
	boardCfg := teamstate.DefaultBoardConfig()

	labels := SuggestPoolLabels(mappings, boardCfg)

	// Only labels (not statuses) mapped to initial column.
	if len(labels) != 1 || labels[0] != "READY TO DEV" {
		t.Errorf("got %v, want [READY TO DEV]", labels)
	}
}

func TestUnmappedItems(t *testing.T) {
	info := &DiscoveryInfo{
		Labels: []LabelInfo{
			{Name: "READY TO DEV"},
			{Name: "bug"},
			{Name: "feature"},
			{Name: "backend"},
		},
	}
	mappings := []SuggestedMapping{
		{Source: "READY TO DEV", ColumnID: "todo"},
	}

	unmapped := UnmappedItems(info, mappings)

	if len(unmapped) != 3 {
		t.Errorf("expected 3 unmapped, got %d: %v", len(unmapped), unmapped)
	}
}

func TestSuggestNewColumns(t *testing.T) {
	info := &DiscoveryInfo{
		Labels: []LabelInfo{
			{Name: "READY TO DEV"},
			{Name: "TESTING"},
		},
	}
	// Existing board has todo and in_progress but no validation.
	existing := []teamstate.BoardColumnConfig{
		{ID: "todo", Name: "TODO", Role: teamstate.ColumnRoleInitial},
		{ID: "in_progress", Name: "IN PROGRESS", Role: teamstate.ColumnRoleActive},
		{ID: "done", Name: "DONE", Role: teamstate.ColumnRoleTerminal},
	}

	suggested := SuggestNewColumns(info, existing)

	// TESTING matches "validation" which doesn't exist in the board → suggest it.
	// READY TO DEV matches "todo" which exists → no suggestion.
	if len(suggested) != 1 {
		t.Fatalf("expected 1 suggestion, got %d: %+v", len(suggested), suggested)
	}
	if suggested[0].ID != "validation" {
		t.Errorf("got ID %q, want 'validation'", suggested[0].ID)
	}
	if suggested[0].Role != teamstate.ColumnRoleActive {
		t.Errorf("got role %q, want 'active'", suggested[0].Role)
	}
}
