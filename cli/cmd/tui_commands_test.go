package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// shippedSummaries are the summaries of the shipped workflows with their
// aliases, read in the YAML.
func shippedSummaries(t *testing.T, files []string) []workflowsvc.Summary {
	t.Helper()
	var out []workflowsvc.Summary
	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)
		doc, diags := workflow.Parse(data, workflow.Source{Layer: workflow.LayerHub, Path: f})
		require.False(t, diags.HasErrors(), "%s: %v", f, diags)
		sum := workflowsvc.Summary{ID: doc.Spec.ID}
		for _, a := range doc.Spec.Aliases {
			sum.Aliases = append(sum.Aliases, workflowsvc.AliasSummary{ID: a.ID, With: a.With})
		}
		out = append(out, sum)
	}
	return out
}

// The omnibar terms of the workflows come from their `aliases:` (no table
// in the code): feature and cadrage open spec; feature with tickets has
// its own command, which opens implement-spec under the alias.
func TestWorkflowCommandsFromYAMLAliases(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "workflows", "*.yaml"))
	require.NoError(t, err)
	var opened []tuiLaunchRequest
	cmds := workflowCommands(shippedSummaries(t, files), func(r tuiLaunchRequest) { opened = append(opened, r) })
	byID := map[string]shell.Command{}
	for _, c := range cmds {
		byID[c.ID] = c
	}
	assert.ElementsMatch(t, []string{"spec", "cadrage", "feature"}, byID["run.spec"].Aliases)
	assert.Equal(t, []string{"implement-spec"}, byID["run.implement-spec"].Aliases)
	assert.ElementsMatch(t, []string{"review-feedback", "feedback", "retours"}, byID["run.review-feedback"].Aliases)
	for id, alias := range map[string]string{"security-fix": "secu", "benchmark": "perf", "debug": "dbg"} {
		assert.Contains(t, byID["run."+id].Aliases, alias)
	}
	assert.NotContains(t, byID["run.spec"].Aliases, "orchestrator", "orchestrator is the entry agent of libre, not an alias of spec")
	route, ok := byID["run.feature.implement-spec"]
	require.True(t, ok, "route of feature with tickets")
	assert.Equal(t, i18n.Tf("tui.launch.alias_command", "feature", "implement-spec", "tickets"), route.Label)
	route.Action()
	byID["run.spec"].Action()
	assert.Equal(t, []tuiLaunchRequest{{WorkflowID: "feature", AliasTarget: "implement-spec"}, {WorkflowID: "spec"}}, opened)
}

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
	cmds = append(cmds, workflowCommands(shippedSummaries(t, files), func(tuiLaunchRequest) {})...)
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
