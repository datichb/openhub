package shell

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

type searchCase struct {
	name      string
	commands  []Command
	query     string
	mode      views.Mode
	wantFirst string // ID of the best match
	wantCount int    // number of results, 0 = not checked
}

func TestCommandRegistrySearch(t *testing.T) {
	disabled := func() bool { return false }
	modeCommands := []Command{
		{ID: "global", Label: "Global"},
		{ID: "team-only", Label: "Team Only", Modes: []views.Mode{views.ModeTeam}},
		{ID: "project-only", Label: "Project Only", Modes: []views.Mode{views.ModeProject}},
	}
	tests := []searchCase{
		{
			name: "empty query returns all",
			commands: []Command{
				{ID: "start", Label: "Start", Category: "Sessions"},
				{ID: "board", Label: "Board", Category: "Projets"},
				{ID: "config", Label: "Config", Category: "Configuration"},
			},
			wantFirst: "start", wantCount: 3,
		},
		{
			name:      "exact ID",
			commands:  []Command{{ID: "start", Label: "Start", Category: "Sessions"}, {ID: "board", Label: "Board", Category: "Projets"}},
			query:     "start",
			wantFirst: "start",
		},
		{
			name: "alias",
			commands: []Command{
				{ID: "start", Label: "Start", Aliases: []string{"session", "code"}, Category: "Sessions"},
				{ID: "board", Label: "Board", Aliases: []string{"kanban"}, Category: "Projets"},
			},
			query:     "kanban",
			wantFirst: "board",
		},
		{
			name: "second alias",
			commands: []Command{
				{ID: "start", Label: "Start", Aliases: []string{"session", "code"}, Category: "Sessions"},
				{ID: "board", Label: "Board", Aliases: []string{"kanban"}, Category: "Projets"},
			},
			query:     "session",
			wantFirst: "start",
		},
		{
			name: "fuzzy",
			commands: []Command{
				{ID: "audit.security", Label: "Audit Sécurité", Category: "Sessions"},
				{ID: "audit.performance", Label: "Audit Performance", Category: "Sessions"},
			},
			query:     "secu",
			wantFirst: "audit.security",
		},
		{
			name: "prefix of the ID ranks first",
			commands: []Command{
				{ID: "start", Label: "Start", Category: "Sessions"},
				{ID: "start.dev", Label: "Start Dev", Category: "Sessions"},
				{ID: "status", Label: "Status", Category: "Système"},
			},
			query:     "sta",
			wantFirst: "start",
		},
		{
			name: "category",
			commands: []Command{
				{ID: "deploy", Label: "Deploy", Category: "Projets"},
				{ID: "status", Label: "Status", Category: "Système"},
			},
			query:     "projet",
			wantFirst: "deploy",
		},
		{
			name: "disabled commands are skipped",
			commands: []Command{
				{ID: "start", Label: "Start", Category: "Sessions"},
				{ID: "hidden", Label: "Hidden", Category: "System", Enabled: disabled},
			},
			wantFirst: "start", wantCount: 1,
		},
		{name: "hub mode: only the global commands", commands: modeCommands, mode: views.ModeHub, wantFirst: "global", wantCount: 1},
		{name: "team mode: global and team commands", commands: modeCommands, mode: views.ModeTeam, wantFirst: "global", wantCount: 2},
		{name: "project mode: global and project commands", commands: modeCommands, mode: views.ModeProject, wantFirst: "global", wantCount: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mode := tc.mode
			if mode == "" {
				mode = views.ModeHub
			}
			results := NewCommandRegistry(tc.commands).Search(tc.query, mode)
			require.NotEmpty(t, results, "at least one result")
			assert.Equal(t, tc.wantFirst, results[0].ID, "best match")
			if tc.wantCount > 0 {
				assert.Len(t, results, tc.wantCount, "number of results")
			}
		})
	}
}

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		query, text string
		want        bool
	}{
		{"str", "start", true},
		{"brd", "board", true},
		{"xyz", "board", false},
		{"", "anything", true},
		{"long", "sh", false},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, fuzzyMatch(tc.query, tc.text), "fuzzyMatch(%q, %q)", tc.query, tc.text)
	}
}

func TestCommandVisibility(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		cmd := Command{ID: "test"}
		assert.True(t, cmd.IsEnabled(), "nil Enabled is always enabled")
		cmd.Enabled = func() bool { return false }
		assert.False(t, cmd.IsEnabled(), "explicit false")
		cmd.Enabled = func() bool { return true }
		assert.True(t, cmd.IsEnabled(), "explicit true")
	})

	t.Run("modes", func(t *testing.T) {
		tests := []struct {
			name  string
			modes []views.Mode
			want  map[views.Mode]bool
		}{
			{"no mode: global", nil, map[views.Mode]bool{views.ModeHub: true, views.ModeTeam: true, views.ModeProject: true}},
			{"project only", []views.Mode{views.ModeProject}, map[views.Mode]bool{views.ModeHub: false, views.ModeTeam: false, views.ModeProject: true}},
			{"project and team", []views.Mode{views.ModeProject, views.ModeTeam}, map[views.Mode]bool{views.ModeHub: false, views.ModeTeam: true, views.ModeProject: true}},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				cmd := Command{ID: "cmd", Modes: tc.modes}
				for mode, want := range tc.want {
					assert.Equal(t, want, cmd.IsVisibleInMode(mode), "visible in %s", mode)
				}
			})
		}
	})
}

func TestCommandRegistryReplaceGroup(t *testing.T) {
	r := NewCommandRegistry([]Command{{ID: "home"}, {ID: "run.old"}})
	r.ReplaceGroup("run.", []Command{{ID: "run.ticket"}, {ID: "run.review"}})
	var ids []string
	for _, c := range r.All() {
		ids = append(ids, c.ID)
	}
	assert.Equal(t, []string{"home", "run.ticket", "run.review"}, ids, "the group is replaced, the other commands stay")
}
