package views

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/platform"
)

func TestProjectsViewScopedToTeamInTeamMode(t *testing.T) {
	v := NewProjectsView(ProjectsViewConfig{})
	all := []ProjectItem{{ID: "a", TeamID: "t1"}, {ID: "b"}, {ID: "c", TeamID: "t2"}}
	v.SetShell(&mockShell{})
	assert.Len(t, v.scoped(all), 3)

	v.SetShell(&mockShell{team: &ActiveTeam{ID: "t1", Name: "T1"}})
	got := v.scoped(all)
	assert.Len(t, got, 1)
	assert.Equal(t, "a", got[0].ID)
}

func TestMetricsScopeFollowsMode(t *testing.T) {
	v := NewMetricsView(MetricsViewConfig{TeamProjects: func(id string) []ActiveProject {
		return []ActiveProject{{ID: "p1", Path: "/p1"}, {ID: "p2", Path: "/p2"}}
	}})
	v.SetShell(&mockShell{})
	assert.Nil(t, v.scope().projects)

	v.SetShell(&mockShell{project: &ActiveProject{ID: "p", Name: "P"}})
	assert.Equal(t, "P", v.scope().label)
	assert.Len(t, v.scope().projects, 1)

	v.SetShell(&mockShell{team: &ActiveTeam{ID: "t", Name: "T"}})
	assert.Len(t, v.scope().projects, 2)
}

func TestMergeAgentMetrics(t *testing.T) {
	got := mergeAgentMetrics(
		[]domain.AgentMetrics{{AgentName: "dev", TotalRuns: 2, SuccessCount: 2, AvgDurationSec: 10, TotalCostUSD: 1}},
		[]domain.AgentMetrics{{AgentName: "dev", TotalRuns: 2, SuccessCount: 0, FailureCount: 2, AvgDurationSec: 20, TotalCostUSD: 2},
			{AgentName: "rev", TotalRuns: 1, SuccessCount: 1}},
	)
	assert.Len(t, got, 2)
	assert.Equal(t, "dev", got[0].AgentName)
	assert.Equal(t, 4, got[0].TotalRuns)
	assert.InDelta(t, 50, got[0].SuccessRate, 0.01)
	assert.InDelta(t, 15, got[0].AvgDurationSec, 0.01)
	assert.InDelta(t, 3, got[0].TotalCostUSD, 0.01)
}

func TestAddStats(t *testing.T) {
	a := &platform.AggregateStats{TotalSessions: 1, TotalCost: 1}
	addStats(a, &platform.AggregateStats{TotalSessions: 2, TotalCost: 0.5, ActiveProjects: 1})
	assert.Equal(t, 3, a.TotalSessions)
	assert.InDelta(t, 1.5, a.TotalCost, 0.001)
	assert.Equal(t, 1, a.ActiveProjects)
}
