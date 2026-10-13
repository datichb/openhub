package cmd

import (
	"context"
	"errors"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/services/wiki"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Metrics of the project wiki in the TUI (modernisation LD-I).

func newProjectWikiView(a *app.App) *views.ProjectWikiView {
	return views.NewProjectWikiView(views.ProjectWikiViewConfig{
		Project: func() (string, string) {
			p, err := resolveActiveProject(a)
			if err != nil || p == nil {
				return "", ""
			}
			return p.Name, p.Path
		},
		Metrics: func(ctx context.Context, path string) (views.ProjectWikiMetrics, error) {
			projectID := ""
			if a != nil && a.Projects != nil {
				if p, err := a.Projects.GetByPath(ctx, path); err == nil {
					projectID = p.ID
				}
			}
			svc := &wiki.Service{}
			if store != nil {
				svc = newWikiService()
			}
			m, err := svc.Metrics(ctx, path, projectID)
			if errors.Is(err, wiki.ErrNoWiki) {
				return views.ProjectWikiMetrics{NoWiki: true}, nil
			}
			if err != nil {
				return views.ProjectWikiMetrics{}, err
			}
			return projectWikiMetrics(m), nil
		},
	})
}

func projectWikiMetrics(m wiki.Metrics) views.ProjectWikiMetrics {
	return views.ProjectWikiMetrics{
		Pages: m.Pages, Specs: m.ByKind[wiki.KindSpec], Decisions: m.ByKind[wiki.KindDecision], Postmortems: m.ByKind[wiki.KindPostmortem],
		Domains: m.Domains, DomainsCovered: m.DomainsCovered, Uncovered: m.Uncovered,
		MedianAgeDays: m.MedianAgeDays, Dated: m.Dated, Oldest: m.Oldest, OldestDays: m.OldestDays,
		Tagged: m.TaggedEnrichments(), Uncertain: m.Tags[wiki.TagUncertain], Confirmed: m.Tags[wiki.TagConfirmed], Deduced: m.Tags[wiki.TagDeduced],
		Pending: m.Pending, Accepted: m.Accepted, Synced: m.Synced,
	}
}
