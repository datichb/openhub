package cmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
)

// Verification of a session that works in a container (fanout-container):
// oh runs its commands in a one-off container of the image of the project
// (the one of the server of the session), with the location of the session
// mounted as for a server group (the integration worktree for the merge
// checkpoint of a fan-out), then removes it. Never on the machine instead.

// containerVerify is the checkpoint.ContainerExecFunc of the app.
func containerVerify(a *app.App) checkpoint.ContainerExecFunc {
	return func(ctx context.Context, sess *domain.Session) (checkpoint.ExecFunc, func(), error) {
		rt := v5ContainerRuntime(a)
		if av, err := rt.Available(ctx); err != nil || !av.OK {
			reason := av.Message()
			if err != nil {
				reason = err.Error()
			}
			return nil, nil, errors.New(i18n.Tf("cmd.verify.container.engine", reason))
		}
		tool := containerVerifyTool(ctx)
		if tool == nil {
			return nil, nil, errors.New(i18n.T("cmd.verify.container.tool"))
		}
		project, err := a.Projects.Get(ctx, sess.ProjectID)
		if err != nil {
			return nil, nil, err
		}
		if err := os.MkdirAll(ohServersDir(), 0o700); err != nil {
			return nil, nil, err
		}
		// Below the servers: shared with the engine VM, as the data of a
		// group. Its name is the container's (one per verification).
		dir, err := os.MkdirTemp(ohServersDir(), "verify-")
		if err != nil {
			return nil, nil, err
		}
		done := func() { _ = os.RemoveAll(dir) }
		if err := os.MkdirAll(filepath.Join(dir, "data"), 0o700); err != nil {
			done()
			return nil, nil, err
		}
		ex := projectExec(project)
		pg, err := rt.Prepare(ctx, ohruntime.Group{
			GroupID: filepath.Base(dir), ProjectID: sess.ProjectID, ProjectDir: project.Path,
			Locations: []string{sess.LaunchPath}, DataDir: filepath.Join(dir, "data"), Tool: tool,
			Dockerfile: ex.Dockerfile, BuildArgs: ex.BuildArgs, Volumes: ex.Volumes,
		})
		if err != nil {
			done()
			return nil, nil, err
		}
		run := func(ctx context.Context, d string, args, env []string, out *checkpoint.TailBuffer) (int, error) {
			inner, ok := pg.Paths.ToInner(d)
			if !ok {
				out.WriteString("\n" + i18n.Tf("cmd.verify.container.not_mounted", d))
				return -1, nil
			}
			exit, err := rt.Exec(ctx, pg, ohruntime.Proc{Argv: args, Dir: inner, Env: envMap(env)}, out)
			if err != nil && ctx.Err() == nil {
				out.WriteString("\n" + err.Error())
				return -1, nil
			}
			return exit, err
		}
		return run, done, nil
	}
}

// containerVerifyTool is the tool installed in the images (the image of a
// verification is the one of the servers: same tool, same version).
func containerVerifyTool(ctx context.Context) ohruntime.Tool {
	if !v5Available(ctx) {
		return nil
	}
	if t, ok := v5Adapter.(interface{ ContainerTool() ohruntime.Tool }); ok {
		return t.ContainerTool()
	}
	return nil
}

func envMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			out[k] = v
		}
	}
	return out
}
