package cmd

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// brickCatalogChecks reports, for every configured team-state with team
// bricks, the invalid bricks, the bricks refused by the integrity check and
// a catalogue never sealed (ADR-054).
func brickCatalogChecks() []doctor.Result {
	a := TryApp()
	if a == nil || a.Config == nil {
		return nil
	}
	var out []doctor.Result
	for _, t := range a.Config.Teams {
		if !t.Enabled {
			continue
		}
		path := t.StatePath
		if path == "" {
			path = config.DefaultTeamStatePath()
		}
		repo := teamstate.NewRepo(t.StateRepo, path)
		if !repo.IsCloned() {
			continue
		}
		if c, ok := brickCatalogCheck(t.DisplayName(), repo, findHubDir()); ok {
			out = append(out, c)
		}
	}
	return out
}

// brickCatalogCheck checks the team catalogue of repo; ok is false when it
// has no brick (no check shown).
func brickCatalogCheck(name string, repo *teamstate.Repo, hubDir string) (doctor.Result, bool) {
	svc := &workflowsvc.Service{HubDir: hubDir, TeamState: func(context.Context, workflowsvc.Context) (*workflowsvc.TeamState, error) {
		return &workflowsvc.TeamState{Repo: repo}, nil
	}}
	check := doctor.Result{Name: i18n.Tf("cmd.doctor.bricks.name", name), OK: true}
	b, probs, err := svc.CheckTeamBricks(context.Background(), workflowsvc.Context{})
	if err != nil {
		check.OK, check.Detail = false, err.Error()
		return check, true
	}
	if b == nil || (len(b.Team) == 0 && len(b.Diagnostics) == 0) {
		return check, false
	}
	var invalid, refused []string
	for key, ds := range probs {
		if key != "" && ds.HasErrors() {
			invalid = append(invalid, key)
		}
	}
	for _, d := range b.Diagnostics {
		refused = append(refused, filepath.Base(d.Source))
	}
	sort.Strings(invalid)
	sort.Strings(refused)
	var parts []string
	parts = append(parts, i18n.Tf("cmd.doctor.bricks.count", len(b.Team)))
	if len(invalid) > 0 {
		check.OK = false
		parts = append(parts, i18n.Tf("cmd.doctor.bricks.invalid", len(invalid), strings.Join(invalid, ", ")))
	}
	if len(refused) > 0 {
		check.OK = false
		parts = append(parts, i18n.Tf("cmd.doctor.bricks.refused", len(refused), strings.Join(refused, ", ")))
	}
	if len(b.Unsealed) > 0 {
		check.Warn = true
		parts = append(parts, i18n.Tf("cmd.doctor.bricks.unsealed", len(b.Unsealed)))
	}
	if len(invalid) > 0 || len(refused) > 0 {
		parts = append(parts, i18n.T("cmd.doctor.bricks.hint"))
	}
	check.Detail = strings.Join(parts, " · ")
	return check, true
}
