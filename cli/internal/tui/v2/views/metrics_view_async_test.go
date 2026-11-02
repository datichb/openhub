package views

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/platform"
)

// fakeStats answers AggregateStats with the number of the call as the count
// of sessions; the first call waits for release.
type fakeStats struct {
	platform.StatsProvider
	mu      sync.Mutex
	periods []string
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
}

func newFakeStats() *fakeStats {
	return &fakeStats{started: make(chan struct{}), release: make(chan struct{})}
}

func (f *fakeStats) Available() bool { return true }

func (f *fakeStats) AggregateStats(period string) (*platform.AggregateStats, error) {
	f.mu.Lock()
	f.periods = append(f.periods, period)
	f.mu.Unlock()
	n := f.calls.Add(1)
	if n == 1 {
		close(f.started)
		<-f.release
		return &platform.AggregateStats{TotalSessions: 111}, nil
	}
	return &platform.AggregateStats{TotalSessions: 222}, nil
}

func (f *fakeStats) seen() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.periods...)
}

func metricsText(app *tview.Application, v *MetricsView) string {
	var s string
	onLoop(app, func() {
		if v.tv != nil {
			s = v.tv.GetText(true)
		}
	})
	return s
}

// The answer of a load started by a previous mount does not overwrite the
// text of the new mount.
func TestMetricsViewStaleLoadAfterRemountIsIgnored(t *testing.T) {
	frLocale(t)
	stats := newFakeStats()
	v := NewMetricsView(MetricsViewConfig{Stats: stats})
	v.SetShell(&mockShell{})
	app := runApp(t, tview.NewFlex())

	first := tview.NewFlex()
	onLoop(app, func() { v.Mount(first, app) })
	<-stats.started
	onLoop(app, v.Unmount)

	second := tview.NewFlex()
	onLoop(app, func() { v.Mount(second, app) })
	require.Eventually(t, func() bool { return strings.Contains(metricsText(app, v), "222") }, 2*time.Second, 10*time.Millisecond)

	close(stats.release) // the old load answers last
	time.Sleep(50 * time.Millisecond)
	text := metricsText(app, v)
	assert.Contains(t, text, "222")
	assert.NotContains(t, text, "111")
}

// The period (and the mode) used by the load are the ones of the key press,
// read on the event loop; a second key supersedes the first.
func TestMetricsViewKeysReloadWithTheirPeriod(t *testing.T) {
	frLocale(t)
	stats := newFakeStats()
	v := NewMetricsView(MetricsViewConfig{Stats: stats})
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	<-stats.started
	onLoop(app, func() { v.HandleKey(key('7')) })
	onLoop(app, func() { v.HandleKey(key('3')) })
	require.Eventually(t, func() bool { return len(stats.seen()) == 3 }, 2*time.Second, 10*time.Millisecond)
	close(stats.release)
	time.Sleep(50 * time.Millisecond)
	seen := stats.seen()
	require.Len(t, seen, 3)
	assert.Equal(t, "all", seen[0], "the mount loads first")
	// The two reloads run concurrently: their order of arrival is free, the
	// periods are those of the keys.
	assert.ElementsMatch(t, []string{"7d", "30d"}, seen[1:])
	assert.Contains(t, metricsText(app, v), "222", "the last request is shown, not the first answer")
}

// staticStats answers the same totals for every period and project.
type staticStats struct {
	platform.StatsProvider
	stats platform.AggregateStats
}

func (s staticStats) Available() bool { return true }
func (s staticStats) AggregateStats(string) (*platform.AggregateStats, error) {
	st := s.stats
	return &st, nil
}

// The usage text carries what `oh dashboard` and `oh metrics` showed: the
// sessions of the day, the active projects (hub scope only), the savings of
// the cache and the share of reasoning.
func TestMetricsUsageTextCoversDashboardAndMetrics(t *testing.T) {
	frLocale(t)
	v := NewMetricsView(MetricsViewConfig{Stats: staticStats{stats: platform.AggregateStats{
		TotalSessions: 12, TodaySessions: 3, ActiveProjects: 4,
		TotalTokensIn: 1_000_000, TotalTokensOut: 200_000, CacheReadTokens: 500_000, ReasoningTokens: 50_000,
		TotalCost: 10,
	}}})

	hub := v.buildUsageText(metricsScope{label: "hub"}, "all")
	assert.Contains(t, hub, "Aujourd'hui")
	assert.Contains(t, hub, "Projets actifs")
	assert.Contains(t, hub, "Économies cache")
	assert.Contains(t, hub, "~3.75 $", "10 × 50% of the input read from the cache × 0.75")
	assert.Contains(t, hub, "Raisonnement :")
	assert.Contains(t, hub, "25%")

	project := v.buildUsageText(metricsScope{label: "p", projects: []ActiveProject{}}, "all")
	assert.NotContains(t, project, "Projets actifs", "the active projects are a hub figure")

	week := v.buildUsageText(metricsScope{label: "hub"}, "7d")
	assert.NotContains(t, week, "Aujourd'hui", "today is shown with all periods only")
}
