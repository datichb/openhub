package tracker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
		"READY TO DEV":  "planned", // the initial column of a board without columns: not "todo"
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
		"To Do":       "planned",
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
		if m.ColumnID != "planned" {
			t.Errorf("mapping %q: got column %q, want 'planned'", m.Source, m.ColumnID)
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
		{"REVIEW", teamstate.ColumnRoleReview},
		{"Code review", teamstate.ColumnRoleReview},
		{"Dev to review", teamstate.ColumnRoleReview},
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

// "todo" is the ID of the initial column of the default board, not a claim
// status: a mapping to it on a board without columns was refused by the sync
// in silence and read as in progress. Whatever the board, what the assistant
// proposes is a status of that board.
func TestSuggestMappingsAreStatusesOfTheBoard(t *testing.T) {
	info := &DiscoveryInfo{
		Labels: []LabelInfo{
			{Name: "READY TO DEV"}, {Name: "Todo"}, {Name: "DEV DOING"}, {Name: "DEV TO REVIEW"},
			{Name: "TESTING"}, {Name: "BLOQUÉ"}, {Name: "done"},
		},
		Statuses: []StatusInfo{
			{Name: "To Do", Category: "new"}, {Name: "Odd", Category: "new"}, {Name: "Odd 2", Category: "indeterminate"}, {Name: "Odd 3", Category: "done"},
		},
	}
	boards := map[string][]teamstate.BoardColumnConfig{
		"no columns":          nil,
		"the default columns": teamstate.DefaultBoardConfig().Columns,
		"own columns":         {{ID: "backlog", Name: "B", Role: teamstate.ColumnRoleInitial}, {ID: "dev", Name: "D"}, {ID: "shipped", Name: "S", Role: teamstate.ColumnRoleTerminal}, {ID: "stuck", Name: "X", Role: teamstate.ColumnRoleBlocked}},
		"no initial role":     {{ID: "first", Name: "F"}, {ID: "second", Name: "S"}, {ID: "end", Name: "E", Role: teamstate.ColumnRoleTerminal}},
	}
	for name, columns := range boards {
		t.Run(name, func(t *testing.T) {
			board := teamstate.BoardConfig{Columns: columns}
			mappings := SuggestMappings(info, TypeJira, columns)
			require.NotEmpty(t, mappings)
			for _, m := range mappings {
				_, migrated, ok := board.ResolveStatus(m.ColumnID)
				assert.True(t, ok && !migrated, "%q → %q is a status of the board", m.Source, m.ColumnID)
			}
			// What the assistant writes is accepted when the config is saved.
			tc := teamstate.TrackerConfig{LabelStatusMapping: map[string]string{}, StatusMapping: map[string]string{}}
			for _, m := range mappings {
				if m.Origin == "status" {
					tc.StatusMapping[m.Source] = m.ColumnID
				} else {
					tc.LabelStatusMapping[m.Source] = m.ColumnID
				}
			}
			assert.NoError(t, tc.ValidateMappings(board))
		})
	}
}

func TestSuggestMappingsInitialColumn(t *testing.T) {
	info := &DiscoveryInfo{Labels: []LabelInfo{{Name: "READY TO DEV"}}}
	cases := []struct {
		name    string
		columns []teamstate.BoardColumnConfig
		want    string
	}{
		{"no columns: the claim status", nil, "planned"},
		{"a todo column with the initial role", teamstate.DefaultBoardConfig().Columns, "todo"},
		{"another initial column", []teamstate.BoardColumnConfig{{ID: "backlog", Name: "B", Role: teamstate.ColumnRoleInitial}, {ID: "end", Name: "E", Role: teamstate.ColumnRoleTerminal}}, "backlog"},
		{"no initial column: the first", []teamstate.BoardColumnConfig{{ID: "first", Name: "F"}, {ID: "end", Name: "E", Role: teamstate.ColumnRoleTerminal}}, "first"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := SuggestMappings(info, TypeGitLab, c.columns)
			require.Len(t, m, 1)
			assert.Equal(t, c.want, m[0].ColumnID)
		})
	}
}

func TestSuggestPoolLabelsOfABoardWithoutColumns(t *testing.T) {
	mappings := SuggestMappings(&DiscoveryInfo{Labels: []LabelInfo{{Name: "READY TO DEV"}, {Name: "DEV DOING"}}}, TypeGitLab, nil)
	assert.Equal(t, []string{"READY TO DEV"}, SuggestPoolLabels(mappings, teamstate.BoardConfig{}), "the initial column is planned")
	// Mappings of an older assistant (todo) on a board whose initial column is todo.
	old := []SuggestedMapping{{Source: "Ready", ColumnID: "todo", Origin: "label"}, {Source: "Fresh", ColumnID: "planned", Origin: "label"}}
	assert.Equal(t, []string{"Ready", "Fresh"}, SuggestPoolLabels(old, teamstate.DefaultBoardConfig()))
}

func TestRetainMappings(t *testing.T) {
	board := teamstate.BoardConfig{Columns: []teamstate.BoardColumnConfig{
		{ID: "backlog", Name: "B", Role: teamstate.ColumnRoleInitial},
		{ID: "dev", Name: "D"},
		{ID: "end", Name: "E", Role: teamstate.ColumnRoleTerminal},
	}}
	kept, dropped := RetainMappings([]SuggestedMapping{
		{Source: "a", ColumnID: "todo"},
		{Source: "b", ColumnID: "dev"},
		{Source: "c", ColumnID: "validation"}, // a column the team left out
	}, board)
	require.Len(t, kept, 2)
	assert.Equal(t, "backlog", kept[0].ColumnID, "todo is the initial column")
	assert.Equal(t, "dev", kept[1].ColumnID)
	require.Len(t, dropped, 1)
	assert.Equal(t, "c", dropped[0].Source)
}

// A board with an initial column of its own gets no second one.
func TestSuggestNewColumnsProposesNoSecondInitialColumn(t *testing.T) {
	info := &DiscoveryInfo{Labels: []LabelInfo{{Name: "READY TO DEV"}}}
	existing := []teamstate.BoardColumnConfig{
		{ID: "backlog", Name: "BACKLOG", Role: teamstate.ColumnRoleInitial},
		{ID: "end", Name: "END", Role: teamstate.ColumnRoleTerminal},
	}
	assert.Empty(t, SuggestNewColumns(info, existing))
}

// A column of review is proposed with the role review (not active), once: a
// board that already has a column of that role, whatever its ID, gets none.
func TestSuggestNewColumnsProposesTheReviewRole(t *testing.T) {
	info := &DiscoveryInfo{Labels: []LabelInfo{{Name: "DEV TO REVIEW"}, {Name: "Code Review"}}}
	own := []teamstate.BoardColumnConfig{
		{ID: "backlog", Name: "B", Role: teamstate.ColumnRoleInitial},
		{ID: "dev", Name: "D", Role: teamstate.ColumnRoleActive},
		{ID: "shipped", Name: "S", Role: teamstate.ColumnRoleTerminal},
	}
	suggested := SuggestNewColumns(info, own)
	if len(suggested) != 1 || suggested[0].ID != "review" || suggested[0].Role != teamstate.ColumnRoleReview {
		t.Fatalf("want one review column with the role review, got %+v", suggested)
	}

	withReview := append(append([]teamstate.BoardColumnConfig{}, own...), teamstate.BoardColumnConfig{ID: "mr", Name: "MR", Role: teamstate.ColumnRoleReview})
	if got := SuggestNewColumns(info, withReview); len(got) != 0 {
		t.Fatalf("a board with a review column gets no other: %+v", got)
	}
	// The default board keeps its review column as it is (active): nothing proposed.
	if got := SuggestNewColumns(info, teamstate.DefaultBoardConfig().Columns); len(got) != 0 {
		t.Fatalf("the default board has its review column: %+v", got)
	}
}

// The mappings of review labels go to the column of the review role; a board
// that has none (it predates the role) gets the column of the review status,
// else the first active column, as before.
func TestSuggestMappingsOfReviewLabelsFollowTheReviewRole(t *testing.T) {
	info := &DiscoveryInfo{Labels: []LabelInfo{{Name: "DEV TO REVIEW"}}}
	target := func(cols []teamstate.BoardColumnConfig) string {
		got := SuggestMappings(info, TypeGitLab, cols)
		if len(got) != 1 {
			t.Fatalf("want one mapping, got %+v", got)
		}
		return got[0].ColumnID
	}
	base := []teamstate.BoardColumnConfig{
		{ID: "backlog", Name: "B", Role: teamstate.ColumnRoleInitial},
		{ID: "dev", Name: "D", Role: teamstate.ColumnRoleActive},
		{ID: "shipped", Name: "S", Role: teamstate.ColumnRoleTerminal},
	}
	if got := target(append(append([]teamstate.BoardColumnConfig{}, base...), teamstate.BoardColumnConfig{ID: "mr", Name: "MR", Role: teamstate.ColumnRoleReview})); got != "mr" {
		t.Errorf("the column of the review role: got %q", got)
	}
	if got := target(teamstate.DefaultBoardConfig().Columns); got != "review" {
		t.Errorf("the default columns (review is active there): got %q", got)
	}
	if got := target(base); got != "dev" {
		t.Errorf("no review column at all: the first active one, as before the role, got %q", got)
	}
	if got := target(nil); got != "review" {
		t.Errorf("no columns: the review status, got %q", got)
	}
}
