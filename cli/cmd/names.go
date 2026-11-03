package cmd

import (
	"context"
	"log/slog"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The one place that turns the IDs of the registry and of the team-state into
// the names shown to people (the name of a project, the display name of a
// member). The views take a views.NamesFunc and call it with each refresh, off
// the event loop, so that a project added or renamed after the start, or a
// member who changed their name, shows at the next refresh. The ID stays what
// it was wherever it is an argument (-p, a proposed command).

// registryNames reads the name of every project of the registry, by ID, whatever
// its status (an archived project keeps its name). Empty without a registry.
func registryNames(ctx context.Context, a *app.App) map[string]string {
	if a == nil || a.Projects == nil {
		return nil
	}
	projects, err := a.Projects.List(ctx, "")
	if err != nil {
		slog.Debug("names: projects not listed", "error", err)
		return nil
	}
	names := make(map[string]string, len(projects))
	for _, p := range projects {
		names[p.ID] = p.Name
	}
	return names
}

// readNames is the names of the registry with the members of the team-state
// of members (nil: no member name). A team-state that cannot be read leaves the
// IDs as they are.
func readNames(ctx context.Context, a *app.App, members teamstate.MemberReader) views.Names {
	var list []teamstate.Member
	if members != nil {
		var err error
		if list, err = members.ListMembers(ctx); err != nil {
			slog.Debug("names: members not listed", "error", err)
		}
	}
	return views.NewNames(registryNames(ctx, a), list)
}

// teamName is the name of a team of the hub config, or its ID when the hub does
// not know the team (nil config: the ID).
func teamName(cfg *config.Config, id string) string {
	if cfg != nil {
		if t := cfg.FindTeam(id); t != nil {
			return t.DisplayName()
		}
	}
	return id
}

// hubNames is the NamesFunc of the views: the registry, and the members of the
// team-state the view reads.
func hubNames(a *app.App) views.NamesFunc {
	return func(ctx context.Context, members teamstate.MemberReader) views.Names {
		return readNames(ctx, a, members)
	}
}
