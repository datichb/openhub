package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// User is the account the tool server runs as in the job (the oh daemon and
// the runner stay root): the agents cannot read the job processes, their
// environment or the oh database.
type User struct {
	UID, GID uint32
	Home     string
}

// JobRuntime runs the tool server inside the job, next to the runner: the
// remote runtime kind, seen from the RunService of the job like a container
// runtime whose machine is the job itself (same paths, loopback address),
// so that the server gets the same minimal environment, the oh MCP servers
// through the daemon, and the Beads gateway.
type JobRuntime struct {
	// User runs the tool server (nil: the runner's user).
	User *User
	// Home is HOME / XDG_CONFIG_HOME / XDG_CACHE_HOME of the server.
	Home string
	// Root is the oh home of the job: the directories between it and the
	// server data are made traversable (not readable) for User.
	Root string
}

var _ ohruntime.Runtime = (*JobRuntime)(nil)

// Kind implements ohruntime.Runtime.
func (r *JobRuntime) Kind() sessionspec.RuntimeKind { return sessionspec.RuntimeRemote }

// Available implements ohruntime.Runtime.
func (r *JobRuntime) Available(context.Context) (ohruntime.Availability, error) {
	return ohruntime.Availability{OK: true, Engine: "oh-runner"}, nil
}

// Prepare implements ohruntime.Runtime: identity paths, loopback address.
func (r *JobRuntime) Prepare(_ context.Context, g ohruntime.Group) (*ohruntime.Prepared, error) {
	return &ohruntime.Prepared{Group: g, HostAddress: "127.0.0.1"}, nil
}

// Load implements ohruntime.Runtime.
func (r *JobRuntime) Load(ctx context.Context, g ohruntime.Group) (*ohruntime.Prepared, error) {
	return r.Prepare(ctx, g)
}

// HostAddress implements ohruntime.Runtime.
func (r *JobRuntime) HostAddress() string { return "127.0.0.1" }

// Teardown implements ohruntime.Runtime.
func (r *JobRuntime) Teardown(context.Context, *ohruntime.Prepared) error { return nil }

// Command implements ohruntime.Runtime: the server listens on the loopback
// only, with the environment given by the adapter plus HOME and PATH, as
// User when set (the directories it writes are handed over to it).
func (r *JobRuntime) Command(_ context.Context, _ *ohruntime.Prepared, p ohruntime.Proc) (*exec.Cmd, error) {
	if len(p.Argv) == 0 {
		return nil, fmt.Errorf("runner: empty command")
	}
	bin, err := exec.LookPath(p.Argv[0])
	if err != nil {
		return nil, err
	}
	args := append([]string{}, p.Argv[1:]...)
	for i, a := range args {
		if a == "0.0.0.0" {
			args[i] = "127.0.0.1"
		}
	}
	home := r.Home
	if home == "" {
		home = filepath.Join(os.TempDir(), "oh-server-home")
	}
	env := map[string]string{
		"HOME": home, "XDG_CONFIG_HOME": filepath.Join(home, ".config"), "XDG_CACHE_HOME": filepath.Join(home, ".cache"),
		"PATH": os.Getenv("PATH"), "LANG": "C.UTF-8", "TMPDIR": filepath.Join(home, "tmp"),
	}
	if v := os.Getenv("SSL_CERT_FILE"); v != "" {
		env["SSL_CERT_FILE"] = v
	}
	for k, v := range p.Env {
		env[k] = v
	}
	for _, d := range []string{home, env["XDG_CONFIG_HOME"], env["XDG_CACHE_HOME"], env["TMPDIR"]} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return nil, err
		}
	}
	// Not bound to ctx: the server outlives the start request (stopped by the RunService).
	cmd := exec.Command(bin, args...)
	cmd.Dir = p.Dir
	for k, v := range env {
		if strings.ContainsAny(k, "=\x00") {
			continue
		}
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	if r.User != nil {
		owned := []string{home}
		if p.Dir != "" {
			owned = append(owned, p.Dir)
		}
		if d := p.Env["XDG_DATA_HOME"]; d != "" {
			owned = append(owned, d)
			traversable(r.Root, d)
		}
		for _, d := range owned {
			if err := chownTree(d, r.User); err != nil {
				return nil, err
			}
		}
		runAs(cmd, r.User)
	}
	return cmd, nil
}

// traversable lets other users cross the directories from root (excluded)
// down to dir's parent.
func traversable(root, dir string) {
	if root == "" {
		return
	}
	root = filepath.Clean(root)
	for d := filepath.Dir(filepath.Clean(dir)); strings.HasPrefix(d, root+string(filepath.Separator)); d = filepath.Dir(d) {
		if st, err := os.Stat(d); err == nil {
			_ = os.Chmod(d, st.Mode().Perm()|0o111)
		}
	}
}

// chownTree gives a directory tree to u.
func chownTree(dir string, u *User) error {
	return filepath.Walk(dir, func(p string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Lchown(p, int(u.UID), int(u.GID))
	})
}
