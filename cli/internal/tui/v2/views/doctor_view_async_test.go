package views

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// slowFirstChecks answers "old" to its first call, after release, and "new"
// to the next ones.
func slowFirstChecks() (run func() []DoctorCheck, started, release chan struct{}) {
	var calls atomic.Int32
	started, release = make(chan struct{}), make(chan struct{})
	run = func() []DoctorCheck {
		if calls.Add(1) == 1 {
			close(started)
			<-release
			return []DoctorCheck{{Name: "old", OK: true}}
		}
		return []DoctorCheck{{Name: "new", OK: true}}
	}
	return
}

func doctorChecks(app *tview.Application, v *DoctorView) (names []string, loading string) {
	onLoop(app, func() {
		for _, c := range v.checks {
			names = append(names, c.Name)
		}
		loading = v.loading
	})
	return
}

// The checks of a previous mount, slow to answer, do not replace those of the
// new mount.
func TestDoctorViewStaleChecksAfterRemountAreIgnored(t *testing.T) {
	run, started, release := slowFirstChecks()
	v := NewDoctorView(nil)
	v.run = run
	v.SetShell(&mockShell{})
	app := runApp(t, tview.NewFlex())

	onLoop(app, func() { v.Mount(tview.NewFlex(), app) })
	<-started
	onLoop(app, v.Unmount)
	onLoop(app, func() { v.Mount(tview.NewFlex(), app) })
	require.Eventually(t, func() bool {
		names, loading := doctorChecks(app, v)
		return loading == "" && len(names) == 1
	}, 2*time.Second, 10*time.Millisecond)

	close(release)
	time.Sleep(50 * time.Millisecond)
	names, _ := doctorChecks(app, v)
	assert.Equal(t, []string{"new"}, names)
}

// r while the first checks are still running: only the answer of the last
// request is shown.
func TestDoctorViewRecheckSupersedesThePendingChecks(t *testing.T) {
	run, started, release := slowFirstChecks()
	v := NewDoctorView(nil)
	v.run = run
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	<-started
	onLoop(app, func() { v.HandleKey(key('r')) })
	require.Eventually(t, func() bool {
		names, loading := doctorChecks(app, v)
		return loading == "" && len(names) == 1
	}, 2*time.Second, 10*time.Millisecond)
	close(release)
	time.Sleep(50 * time.Millisecond)
	names, loading := doctorChecks(app, v)
	assert.Equal(t, []string{"new"}, names)
	assert.Empty(t, loading)
}

// The mount does not run the checks on the event loop: it returns while they
// are still running.
func TestDoctorViewMountDoesNotBlockTheLoop(t *testing.T) {
	run, started, release := slowFirstChecks()
	defer close(release)
	v := NewDoctorView(nil)
	v.run = run
	v.SetShell(&mockShell{})
	content := tview.NewFlex()
	app := runApp(t, content)
	onLoop(app, func() { v.Mount(content, app) })
	<-started
	_, loading := doctorChecks(app, v) // the loop answers while a check blocks
	assert.NotEmpty(t, loading)
}
