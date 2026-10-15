package container

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
)

// engineExitError is the first exit code an engine CLI uses for its own
// failures (`run`: 125 the engine, 126 the command cannot be invoked, 127
// not found); only 125 says nothing of the command.
const engineExitError = 125

// Exec runs p once in a fresh container of the prepared group pg and waits
// for it (fanout-container: oh's verification of a checkpoint runs where
// the agents run). Same image, mounts, user and container environment as
// the server of a group; stdin is empty, the output goes to out. The
// container and its HOME volume are removed when it ends; when ctx ends,
// the container is removed (stopped) first. exit is the exit code of p; err
// is set when it could not run (engine failure included).
func (r *Runtime) Exec(ctx context.Context, pg *ohruntime.Prepared, p ohruntime.Proc, out io.Writer) (exit int, err error) {
	cmd, err := r.Command(ctx, pg, p)
	if err != nil {
		return -1, err
	}
	cmd.Stdout, cmd.Stderr = out, out
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Start(); err != nil {
		r.removeOnce(pg)
		return -1, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var werr error
	select {
	case werr = <-done:
	case <-ctx.Done():
		r.removeOnce(pg)
		_ = cmd.Process.Kill()
		werr = <-done
	}
	r.removeOnce(pg)
	if ctx.Err() != nil {
		return -1, ctx.Err()
	}
	var ee *exec.ExitError
	switch {
	case errors.As(werr, &ee) && ee.ExitCode() == engineExitError:
		return -1, fmt.Errorf("container engine failed (exit %d)", engineExitError)
	case errors.As(werr, &ee):
		return ee.ExitCode(), nil
	case werr != nil:
		return -1, werr
	}
	return 0, nil
}

// removeOnce removes the container of a one-off run of pg, its HOME volume
// and its env file (own context: ctx may be over).
func (r *Runtime) removeOnce(pg *ohruntime.Prepared) {
	s, ok := pg.Spec.(*Spec)
	if !ok || s == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	e, av := r.Engine(ctx)
	if !av.OK {
		return
	}
	_, _ = r.opts.Runner.Run(ctx, e.CLI, e.Command("rm", "-f", s.Name)[1:]...)
	for _, m := range s.Mounts {
		if m.Volume && m.Target == InnerHome {
			_, _ = r.opts.Runner.Run(ctx, e.CLI, e.Command("volume", "rm", "-f", m.Source)[1:]...)
		}
	}
	if pg.Group.DataDir != "" {
		_ = os.Remove(filepath.Join(filepath.Dir(specFile(pg.Group.DataDir)), "container.env"))
	}
}
