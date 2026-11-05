package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// What `oh teams archive|restore|remove` and the Teams view of the TUI both
// do to hub.toml, in one place: the default team is kept coherent and the
// projects of a removed team are detached.

// defaultTeamMovedNote says where the default team went once team id, which
// was it, stopped being it (archived or removed): "" when it was not.
func defaultTeamMovedNote(a *app.App, id string, wasDefault bool) string {
	if !wasDefault {
		return ""
	}
	if next := a.Config.DefaultTeam(); next != nil {
		return i18n.Tf("v54r.teams.archive.default_moved", id, next.ID)
	}
	return i18n.Tf("v54r.teams.archive.default_none", id)
}

// isDefaultTeam reports whether id is the default team now.
func isDefaultTeam(a *app.App, id string) bool {
	def := a.Config.DefaultTeam()
	return def != nil && def.ID == id
}

// archiveTeam sets enabled = false on team id; an archived team is never the
// default team. The note tells where the default went when it was that team.
func archiveTeam(a *app.App, id string) (note string, err error) {
	if a.Config.FindTeam(id) == nil {
		return "", &config.UnknownTeamError{ID: id}
	}
	wasDefault := isDefaultTeam(a, id)
	if err := updateHubConfig(a, func(c *config.Config) error {
		for i := range c.Teams {
			if c.Teams[i].ID == id {
				c.Teams[i].Enabled = false
				break
			}
		}
		c.ClearDefaultTeam(id)
		return nil
	}); err != nil {
		return "", fmt.Errorf("%s: %w", i18n.T("cmd.teams.save_error"), err)
	}
	return defaultTeamMovedNote(a, id, wasDefault), nil
}

// restoreTeam sets enabled = true on team id.
func restoreTeam(a *app.App, id string) error {
	if a.Config.FindTeam(id) == nil {
		return &config.UnknownTeamError{ID: id}
	}
	if err := updateHubConfig(a, func(c *config.Config) error {
		for i := range c.Teams {
			if c.Teams[i].ID == id {
				c.Teams[i].Enabled = true
				break
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.teams.save_error"), err)
	}
	return nil
}

// teamProjectNames are the names of the projects attached to team id.
func teamProjectNames(ctx context.Context, a *app.App, id string) []string {
	if a.Projects == nil {
		return nil
	}
	projects, err := a.Projects.List(ctx, "")
	if err != nil {
		return nil
	}
	var names []string
	for _, p := range projects {
		if p.TeamID != nil && *p.TeamID == id {
			names = append(names, p.Name)
		}
	}
	return names
}

// removeTeam takes team id out of hub.toml (its default mark with it) and
// detaches its projects, which become projects of no team. It returns the
// names of the projects it detached and a note on the default team.
func removeTeam(ctx context.Context, a *app.App, id string) (detached []string, note string, err error) {
	if a.Config.FindTeam(id) == nil {
		return nil, "", &config.UnknownTeamError{ID: id}
	}
	wasDefault := isDefaultTeam(a, id)
	if err := updateHubConfig(a, func(c *config.Config) error {
		c.ClearDefaultTeam(id)
		kept := make([]config.TeamConfig, 0, len(c.Teams))
		for _, t := range c.Teams {
			if t.ID != id {
				kept = append(kept, t)
			}
		}
		c.Teams = kept
		return nil
	}); err != nil {
		return nil, "", fmt.Errorf("%s: %w", i18n.T("cmd.teams.save_error"), err)
	}
	var errs []error
	if a.Projects != nil {
		projects, lerr := a.Projects.List(ctx, "")
		if lerr != nil {
			errs = append(errs, lerr)
		}
		for i := range projects {
			if projects[i].TeamID == nil || *projects[i].TeamID != id {
				continue
			}
			projects[i].TeamID = nil
			if uerr := a.Projects.Update(ctx, &projects[i]); uerr != nil {
				errs = append(errs, fmt.Errorf("%s: %w", projects[i].Name, uerr))
				continue
			}
			detached = append(detached, projects[i].Name)
		}
	}
	return detached, defaultTeamMovedNote(a, id, wasDefault), errors.Join(errs...)
}
