// Package workflow is the WorkflowService (03 §2.4): catalogue of the
// declarative workflows of every available layer, resolution with the
// origin of each value, validation against the brick catalogue and
// rendering of the initial prompt. Shared by the CLI and the TUI; no UI code.
//
// The team-state layers (team, project, drafts) and the editing methods of
// phase 2 are in separate files (teamstate.go, draft.go, publish.go…).
package workflow

import (
	"context"
	"os"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	wf "github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/workflow/hubcat"
)

// Service reads, resolves and validates workflows.
type Service struct {
	// HubDir is the hub content directory (agents/, skills/, workflows/).
	HubDir string
	// HubWorkflowsDir replaces <HubDir>/workflows as the hub layer (tests,
	// workflows under development). Empty: <HubDir>/workflows.
	HubWorkflowsDir string
	// Isolation is the closed-world level of the current tool adapter
	// (empty: unknown, the `isolation: strict` rule is not checked).
	Isolation sessionspec.IsolationLevel
	// Lang selects the language of labels ("" = current locale).
	Lang string
	// TeamState returns the team-state of a context (phase 2, teamstate.go);
	// nil: hub layer only.
	TeamState TeamStateFunc
	// BricksCacheDir holds the hub merged with team catalogues (bricks.go);
	// empty: <HubDir>/../cache/bricks.
	BricksCacheDir string
	// InputSources compute the inputs declared with `from:` at launch, by
	// source name (wf.InputSources); a missing source is not computed.
	InputSources map[string]InputSource
}

// Context is the scope a workflow is listed or resolved in. The project and
// team select the team-state layers (phase 2).
type Context struct {
	ProjectID string
	TeamID    string
}

// catalog holds the documents of every available layer and the validation
// environment.
type catalog struct {
	docs   *wf.MemCatalog
	diags  wf.Diagnostics // load problems (files that could not be read)
	env    wf.Env
	team   *TeamState // team-state layers loaded (nil: hub only)
	bricks *Bricks    // hub merged with the team catalogue (nil: hub only)
}

// load reads the available layers. A missing hub directory gives an empty
// catalogue.
func (s *Service) load(ctx context.Context, c Context) (*catalog, error) {
	cat, err := s.loadHub()
	if err != nil {
		return nil, err
	}
	if err := s.addTeamLayers(ctx, c, cat); err != nil {
		return nil, err
	}
	return cat, nil
}

// loadHub reads the hub layer and the validation environment.
func (s *Service) loadHub() (*catalog, error) {
	c := &catalog{docs: wf.NewMemCatalog()}
	if dir := s.hubWorkflowsDir(); dir != "" {
		if s.HubWorkflowsDir != "" {
			c.diags = c.docs.LoadDir(dir, wf.LayerHub)
		} else {
			c.docs, c.diags = hubcat.LoadWorkflows(s.HubDir)
		}
	}
	if s.HubDir != "" && dirExists(filepath.Join(s.HubDir, "agents")) {
		hc, err := hubcat.New(s.HubDir)
		if err != nil {
			return nil, err
		}
		c.env = hc.Env()
		c.env.Skills = bundle.NewSkillCatalog(s.HubDir)
	}
	c.env.Isolation = s.Isolation
	return c, nil
}

func (s *Service) hubWorkflowsDir() string {
	if s.HubWorkflowsDir != "" {
		return s.HubWorkflowsDir
	}
	if s.HubDir == "" {
		return ""
	}
	return filepath.Join(s.HubDir, hubcat.WorkflowsDir)
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func (s *Service) lang() string {
	if s.Lang != "" {
		return s.Lang
	}
	return i18n.Locale()
}
