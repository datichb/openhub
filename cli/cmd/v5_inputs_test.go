package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// QB2: every source of the workflow schema has an implementation on the
// machine.
func TestEveryInputSourceIsImplemented(t *testing.T) {
	got := inputSources(&app.App{Config: &config.Config{}})
	for _, src := range workflow.InputSources {
		assert.Contains(t, got, src)
	}
	assert.Len(t, got, len(workflow.InputSources), "no source outside the schema list")
}

// QB2: ticket.brief reads the takeover brief of the ticket in the team
// space of the project (the enriched one first).
func TestTicketBriefSource(t *testing.T) {
	space := t.TempDir()
	dir := filepath.Join(space, "projects", "web", "takeover-briefs")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "oh-12_2026-10-07.md"), []byte("# brief"), 0o644))
	team := "acme"
	a := &app.App{
		Config:   &config.Config{Teams: []config.TeamConfig{{ID: "acme", Enabled: true, StatePath: space}}},
		Projects: &mockProjectStore{projects: []domain.Project{{ID: "p1", Name: "web", TeamID: &team}}},
	}
	src := inputSources(a)["ticket.brief"]
	ctx := context.Background()
	got, err := src(ctx, workflowsvc.Context{ProjectID: "p1"}, "oh-12")
	require.NoError(t, err)
	assert.Equal(t, "# brief", got)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "oh-12_2026-10-07.enriched.md"), []byte("# enriched"), 0o644))
	got, _ = src(ctx, workflowsvc.Context{ProjectID: "p1"}, "oh-12")
	assert.Equal(t, "# enriched", got)

	_, err = src(ctx, workflowsvc.Context{ProjectID: "p1"}, "oh-99")
	assert.ErrorContains(t, err, "web/oh-99")
}
