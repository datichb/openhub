package fanout

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

// fanout-container: a parent in a container. The driver (on the machine)
// works on the machine paths of the worktrees; each session it launches
// carries the parent (its runtime: the launcher starts it in a container);
// merges, budget and clean-up are those of a local fan-out.
func TestDriverContainerParent(t *testing.T) {
	f := newFixture(t, "ev-1", "ev-2")
	p, err := f.store.Get(context.Background(), "ses_parent")
	require.NoError(t, err)
	p.Runtime = "container"
	require.NoError(t, f.store.Update(context.Background(), p))
	f.launcher.hold = 150 * time.Millisecond
	f.launcher.work["ev-1"] = [2]string{"calc.go", "package calc\n\nfunc Sub() {}\n"}
	f.launcher.work["ev-2"] = [2]string{"calc.go", "package calc\n\nfunc Mul() {}\n"}
	f.launcher.resolve = "package calc\n\nfunc Sub() {}\n\nfunc Mul() {}\n"
	fo := f.runDriver(t)
	require.Equal(t, domain.FanoutReady, fo.Status)
	assert.Equal(t, domain.ChildMerged, conflicted(t, fo).State, "merger in the runtime of the parent")

	roles := map[domain.FanoutRole]int{}
	for _, r := range f.launcher.started {
		roles[r.Record.Role]++
		assert.Equal(t, "container", r.Parent.Runtime, "%s launched for a parent in a container", r.Record.Role)
		assert.Equal(t, filepath.Dir(f.project), filepath.Dir(r.Dir), "a worktree next to the project, at its machine path")
	}
	assert.Equal(t, 2, roles[domain.RoleChild])
	assert.Equal(t, 1, roles[domain.RoleMerger])

	in := &Integrator{Sessions: f.store, Close: func(context.Context, string, string, string) error { return nil }}
	p, _ = f.store.Get(context.Background(), "ses_parent")
	_, err = in.Integrate(context.Background(), p, "cp-merge")
	require.NoError(t, err, "oh merges on the machine")
	assert.FileExists(t, filepath.Join(f.project, "calc.go"))
	assert.NoDirExists(t, fo.Integration.Path, "cleaned up after cp-merge")
}
