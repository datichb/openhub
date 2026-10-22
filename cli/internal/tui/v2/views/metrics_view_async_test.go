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
	assert.Equal(t, []string{"all", "7d", "30d"}, stats.seen())
	assert.Contains(t, metricsText(app, v), "222", "the last request is shown, not the first answer")
}
