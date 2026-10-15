package container

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// fanout-container: a one-off run in a fresh container of a group (oh's
// verification of a checkpoint), its exit code, its output, and nothing
// left behind.
func TestExecOnce(t *testing.T) {
	_, project, worktree, _, data := layout(t)
	st := &fakeStore{images: map[string]string{}}
	r := newImageEngine(t, st)
	cli := filepath.Join(t.TempDir(), "docker")
	require.NoError(t, os.WriteFile(cli, []byte(`#!/bin/sh
echo "args: $*"
case "$*" in
  *engine-down*) exit 125 ;;
  *slow*) exec sleep 30 ;;
  *fail*) exit 3 ;;
esac
`), 0o755))
	r.bins["docker"] = cli
	r.on("docker rm -f", "", nil).on("docker volume rm -f", "", nil)
	r.on("docker run --rm --entrypoint sh --add-host host.docker.internal:host-gateway", "172.17.0.1      host.docker.internal\n", nil)
	rt := newImageRuntime(t, r)
	g := ohruntime.Group{Key: sessionspec.GroupKey{ProjectID: "app", Runtime: sessionspec.RuntimeContainer}, GroupID: "verify-1",
		ProjectID: "app", ProjectDir: project, Locations: []string{worktree}, DataDir: data, Tool: &fakeTool{ver: "2.0.20", bin: writeBin(t)}}
	pg, err := rt.Prepare(context.Background(), g)
	require.NoError(t, err)
	inner, ok := pg.Paths.ToInner(worktree)
	require.True(t, ok)

	var out bytes.Buffer
	exit, err := rt.Exec(context.Background(), pg, ohruntime.Proc{Argv: []string{"make", "check"}, Dir: inner, Env: map[string]string{"CI": "true"}}, &out)
	require.NoError(t, err)
	assert.Equal(t, 0, exit)
	assert.Contains(t, out.String(), "-w "+inner)
	assert.Contains(t, out.String(), "--cap-drop=ALL", "as confined as the agents")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(out.String()), "make check"))
	assert.NotEmpty(t, r.called("docker rm -f oh-verify-1"), "the container is removed")
	assert.NotEmpty(t, r.called("docker volume rm -f oh-app-"), "its HOME volume too")
	_, err = os.Stat(filepath.Join(filepath.Dir(data), "container.env"))
	assert.True(t, os.IsNotExist(err), "no env file left")

	exit, err = rt.Exec(context.Background(), pg, ohruntime.Proc{Argv: []string{"fail"}, Dir: inner}, &out)
	require.NoError(t, err)
	assert.Equal(t, 3, exit, "the exit code of the command")

	_, err = rt.Exec(context.Background(), pg, ohruntime.Proc{Argv: []string{"engine-down"}, Dir: inner}, &out)
	assert.Error(t, err, "an engine failure is not a failed command")

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = rt.Exec(ctx, pg, ohruntime.Proc{Argv: []string{"slow"}, Dir: inner}, &out)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 10*time.Second, "killed at the deadline")
}
