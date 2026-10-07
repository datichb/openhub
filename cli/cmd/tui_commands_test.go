package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
)

// QB2: every omnibar term (command ID or alias) has a single meaning: `q`
// (quit / quick), `hub` (settings / hub mode), `team config` (team detail /
// configure) and `tokens` (secrets / metrics) had two.
func TestOmnibarAliasesAreUnique(t *testing.T) {
	a := &app.App{Config: &config.Config{Teams: []config.TeamConfig{{ID: "t", Enabled: true, StateRepo: "git@x:t.git", MemberID: "me"}}}}
	cmds := buildCommands(a)
	files, err := filepath.Glob(filepath.Join("..", "..", "workflows", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("shipped workflows: %v", err)
	}
	for _, f := range files {
		id := strings.TrimSuffix(filepath.Base(f), ".yaml")
		cmds = append(cmds, shell.Command{ID: workflowCommandPrefix + id, Aliases: append([]string{id}, workflowCommandAliases[id]...)})
	}
	owner := map[string]string{}
	for _, c := range cmds {
		terms := map[string]bool{strings.ToLower(c.ID): true}
		for _, al := range c.Aliases {
			terms[strings.ToLower(al)] = true
		}
		for term := range terms {
			if prev, ok := owner[term]; ok && prev != c.ID {
				t.Errorf("omnibar term %q used by %s and %s", term, prev, c.ID)
			}
			owner[term] = c.ID
		}
	}
}
