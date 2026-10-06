package runsvc

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

// The prompt says whether the session already has its own branch, so that
// the agent does not offer to create one (v5 finalisation, Q3-2).
func TestLocationWorkBranch(t *testing.T) {
	project := t.TempDir()
	git(t, project, "init", "-q", "-b", "main")
	git(t, project, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "init")

	base := Location{Path: project, Kind: LocationBase}
	assert.Empty(t, base.WorkBranch(project), "base branch: no dedicated branch")

	git(t, project, "checkout", "-q", "-b", "feat/x")
	assert.Equal(t, "feat/x", base.WorkBranch(project), "the project is on a branch of its own")

	wt := Location{Path: "/elsewhere", Kind: LocationWorktree, Branch: "oh/ticket-bd-1"}
	assert.Equal(t, "oh/ticket-bd-1", wt.WorkBranch(project))

	assert.Empty(t, Location{Path: t.TempDir(), Kind: LocationBase}.WorkBranch(project), "outside git")
}
