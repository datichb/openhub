// Package sessionstats computes the session statistics shown by `oh metrics`,
// the dashboard, the API and the Metrics view from the oh session registry
// (oh.db `sessions`: cost and tokens kept up to date by the daemon). It
// replaces the reading of the opencode V1 database (v5, P3-T30): sessions
// run before v5 outside oh are no longer counted.
package sessionstats

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/platform"
)

// Provider implements platform.StatsProvider over the oh stores.
type Provider struct {
	Sessions domain.SessionStore
	Projects domain.ProjectStore
	// Now is the clock (tests).
	Now func() time.Time
}

var _ platform.StatsProvider = (*Provider)(nil)

// New returns a provider over the oh stores.
func New(sessions domain.SessionStore, projects domain.ProjectStore) *Provider {
	return &Provider{Sessions: sessions, Projects: projects}
}

func (p *Provider) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

// Available implements platform.StatsProvider.
func (p *Provider) Available() bool { return p != nil && p.Sessions != nil }

// since returns the start of a period ("7d", "30d", else all time).
func (p *Provider) since(period string) time.Time {
	switch period {
	case "7d":
		return p.now().AddDate(0, 0, -7)
	case "30d":
		return p.now().AddDate(0, 0, -30)
	}
	return time.Time{}
}

func (p *Provider) list(projectPath string, since time.Time) ([]domain.Session, error) {
	ctx := context.Background()
	all, err := p.Sessions.List(ctx, "")
	if err != nil {
		return nil, err
	}
	var paths map[string]string
	if projectPath != "" && p.Projects != nil {
		paths = map[string]string{}
		if projects, err := p.Projects.List(ctx, ""); err == nil {
			for _, pr := range projects {
				paths[pr.ID] = pr.Path
			}
		}
	}
	out := all[:0:0]
	for _, s := range all {
		if s.StartedAt.Before(since) {
			continue
		}
		if projectPath != "" && !inProject(s, paths[s.ProjectID], projectPath) {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

// inProject reports whether a session belongs to the project at path (its
// project, or a location below the path).
func inProject(s domain.Session, projectDir, path string) bool {
	path = filepath.Clean(path)
	if projectDir != "" && filepath.Clean(projectDir) == path {
		return true
	}
	for _, loc := range []string{s.Location, s.LaunchPath} {
		if loc != "" && (filepath.Clean(loc) == path || strings.HasPrefix(filepath.Clean(loc), path+string(filepath.Separator))) {
			return true
		}
	}
	return false
}

func (p *Provider) aggregate(list []domain.Session) *platform.AggregateStats {
	out := &platform.AggregateStats{}
	y, m, d := p.now().Date()
	projects := map[string]bool{}
	for _, s := range list {
		out.TotalSessions++
		out.TotalTokensIn += s.TokensIn
		out.TotalTokensOut += s.TokensOut
		out.TotalCost += s.Cost
		out.CacheReadTokens += s.TokensCacheRead
		out.ReasoningTokens += s.TokensReasoning
		if sy, sm, sd := s.StartedAt.Local().Date(); sy == y && sm == m && sd == d {
			out.TodaySessions++
		}
		projects[s.ProjectID] = true
	}
	out.ActiveProjects = len(projects)
	return out
}

// AggregateStats implements platform.StatsProvider.
func (p *Provider) AggregateStats(period string) (*platform.AggregateStats, error) {
	list, err := p.list("", p.since(period))
	if err != nil {
		return nil, err
	}
	return p.aggregate(list), nil
}

// ProjectStats implements platform.StatsProvider.
func (p *Provider) ProjectStats(projectPath, period string) (*platform.AggregateStats, error) {
	list, err := p.list(projectPath, p.since(period))
	if err != nil {
		return nil, err
	}
	return p.aggregate(list), nil
}

// RecentSessions implements platform.StatsProvider.
func (p *Provider) RecentSessions(limit int) ([]platform.SessionStat, error) {
	return p.recent("", limit)
}

// ProjectSessions implements platform.StatsProvider.
func (p *Provider) ProjectSessions(projectPath string, limit int) ([]platform.SessionStat, error) {
	return p.recent(projectPath, limit)
}

func (p *Provider) recent(projectPath string, limit int) ([]platform.SessionStat, error) {
	list, err := p.list(projectPath, time.Time{})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].StartedAt.After(list[j].StartedAt) })
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	out := make([]platform.SessionStat, 0, len(list))
	for _, s := range list {
		st := platform.SessionStat{ID: s.ID, ProjectID: s.ProjectID, Model: s.Model, Cost: s.Cost,
			TokensInput: s.TokensIn, TokensOutput: s.TokensOut, TokensReasoning: s.TokensReasoning,
			TokensCacheRead: s.TokensCacheRead, TimeCreated: s.StartedAt, TimeUpdated: s.StartedAt}
		if s.Title != nil {
			st.Title = *s.Title
		}
		if s.EndedAt != nil {
			st.TimeUpdated = *s.EndedAt
		} else if s.StateChangedAt != nil {
			st.TimeUpdated = *s.StateChangedAt
		}
		out = append(out, st)
	}
	return out, nil
}

// DailyCosts implements platform.StatsProvider (local days, oldest first).
func (p *Provider) DailyCosts(period string) ([]platform.DayCost, error) {
	list, err := p.list("", p.since(period))
	if err != nil {
		return nil, err
	}
	byDay := map[string]float64{}
	for _, s := range list {
		byDay[s.StartedAt.Local().Format("2006-01-02")] += s.Cost
	}
	out := make([]platform.DayCost, 0, len(byDay))
	for day, cost := range byDay {
		out = append(out, platform.DayCost{Day: day, Cost: cost})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Day < out[j].Day })
	return out, nil
}
