package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
)

// testHubEnv points the hub at the repository content and the hub layer at
// the test workflows (no opencode V2: withoutV2).
func testHubEnv(t *testing.T) {
	t.Helper()
	root := repoRoot(t)
	home := t.TempDir()
	if err := os.Symlink(root, filepath.Join(home, "hub")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OH_HOME", home)
	withoutV2(t, nil)
	t.Setenv(workflowsDirEnv, filepath.Join(root, "cli", "internal", "services", "workflow", "testdata", "workflows"))
}

func TestLaunchFormConfig(t *testing.T) {
	testHubEnv(t)
	a := &app.App{Config: &config.Config{}}
	project := &domain.Project{ID: "p1", Name: "demo", Path: t.TempDir()}
	cfg, err := launchFormConfig(context.Background(), a, project, tuiLaunchRequest{WorkflowID: "ticket", Tickets: []string{"bd-1"}, AtOptions: true})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WorkflowID != "ticket" || cfg.Spec == nil || !cfg.AtOptions || len(cfg.Tickets) != 1 {
		t.Fatalf("cfg = %+v", cfg)
	}
	if len(cfg.Runtimes) != 2 || !cfg.Runtimes[0].Available || cfg.Runtimes[1].Available || cfg.Runtimes[1].Reason == "" {
		t.Fatalf("runtimes = %+v (container unavailable without V2)", cfg.Runtimes)
	}
	if len(cfg.Locations) != 1 || cfg.Locations[0].Value != "base" {
		t.Fatalf("locations = %+v (not a git repository: base only)", cfg.Locations)
	}
	if cfg.DefaultAttach != "auto" || len(cfg.Attach) == 0 {
		t.Fatalf("attach = %q %v", cfg.DefaultAttach, cfg.Attach)
	}
	if _, err := launchFormConfig(context.Background(), a, project, tuiLaunchRequest{WorkflowID: "nope"}); err == nil {
		t.Fatal("unknown workflow accepted")
	}
}

func TestHeadlessCheck(t *testing.T) {
	testHubEnv(t)
	t.Setenv(workflowsDirEnv, "") // shipped workflows
	ctx := context.Background()
	svc := newWorkflowService(ctx)
	brief, err := svc.Resolve(ctx, workflowsvc.Context{}, "brief-enrich", workflowsvc.ResolveOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if err := headlessCheck(&preparedRun{resolution: brief}); err != nil {
		t.Fatalf("brief-enrich runs without interface: %v", err)
	}
	ticket, err := svc.Resolve(ctx, workflowsvc.Context{}, "ticket", workflowsvc.ResolveOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if err := headlessCheck(&preparedRun{resolution: ticket}); err == nil || !strings.Contains(err.Error(), "cp-2") {
		t.Fatalf("ticket waits at cp-2: %v", err)
	}
}
