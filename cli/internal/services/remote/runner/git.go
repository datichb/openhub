package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Git runs git in the job (as root) with the project token.
//
// The work tree belongs to the agent account once the session has started:
// nothing the agent controls is ever executed or read as configuration by
// these git processes (S-01). Finish never runs git in the agent's
// repository: it copies the object database and the refs into a fresh
// repository of its own and reads the work tree through it (--git-dir /
// --work-tree), with hooks and fsmonitor disabled, no global or system
// configuration, and the token handed over by a one-shot GIT_ASKPASS
// script (never in a command line, a URL, an environment variable or a
// configuration file).
type Git struct {
	Token   string
	Secrets Secrets // redacted from errors
	// Scratch holds the private files of the git steps (askpass, Finish
	// repository); "" = the system temporary directory. It must not be
	// reachable by the agent account.
	Scratch string
	// Agent owns the work tree during the session (nil: same account as
	// the runner). Files of the work tree not owned by it are left out of
	// the final commit: root would read them on the agent's behalf.
	Agent *User
}

// gitSafeArgs disable what a repository could make git execute.
var gitSafeArgs = []string{
	"-c", "core.hooksPath=/dev/null",
	"-c", "core.fsmonitor=false",
	"-c", "credential.helper=",
	"-c", "protocol.ext.allow=never",
}

// gitEnv is the environment of the git processes: the runner's, without
// any GIT_* variable, with no global or system configuration and no
// terminal prompt.
func gitEnv(extra ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "GIT_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, append([]string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=" + os.DevNull,
	}, extra...)...)
}

func (g Git) runEnv(ctx context.Context, dir string, env []string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append(append([]string{}, gitSafeArgs...), args...)...)
	cmd.Dir = dir
	cmd.Env = env
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		return out.String() + errOut.String(), &gitError{args: args, msg: g.Secrets.Redact(strings.TrimSpace(errOut.String() + " " + err.Error())), err: err}
	}
	return out.String() + errOut.String(), nil
}

type gitError struct {
	args []string
	msg  string
	err  error
}

func (e *gitError) Error() string { return "git " + e.args[0] + ": " + e.msg }
func (e *gitError) Unwrap() error { return e.err }

// scratch creates a private directory (0700) for one git step.
func (g Git) scratch(prefix string) (string, error) {
	base := g.Scratch
	if base != "" {
		if err := os.MkdirAll(base, 0o700); err != nil {
			return "", err
		}
	}
	return os.MkdirTemp(base, prefix)
}

// askpass writes a one-shot GIT_ASKPASS script that answers the user name
// and the token (read from a 0600 file next to it), and returns the
// environment entry; cleanup removes both.
func (g Git) askpass() (env string, cleanup func(), err error) {
	dir, err := g.scratch("oh-askpass-")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { _ = os.RemoveAll(dir) }
	tokenFile := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenFile, []byte(g.Token), 0o600); err != nil {
		cleanup()
		return "", nil, err
	}
	script := filepath.Join(dir, "askpass.sh")
	body := "#!/bin/sh\ncase \"$1\" in\n[Uu]sername*) echo oauth2 ;;\n*) cat " + shellQuote(tokenFile) + " ;;\nesac\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		cleanup()
		return "", nil, err
	}
	return "GIT_ASKPASS=" + script, cleanup, nil
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// authed runs git with the project token.
func (g Git) authed(ctx context.Context, dir string, args ...string) (string, error) {
	ask, cleanup, err := g.askpass()
	if err != nil {
		return "", err
	}
	defer cleanup()
	return g.runEnv(ctx, dir, gitEnv(ask), args...)
}

// Clone clones ref of cloneURL into dir and starts branch at commit (ref's
// head when empty). It runs before the session: dir is still the
// runner's.
func (g Git) Clone(ctx context.Context, cloneURL, ref, commit, branch, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if _, err := g.authed(ctx, dir, "clone", "--quiet", "--single-branch", "--branch", ref, cloneURL, "."); err != nil {
		return "", err
	}
	start := commit
	if start == "" {
		start = "HEAD"
	}
	env := gitEnv()
	if _, err := g.runEnv(ctx, dir, env, "checkout", "--quiet", "-B", branch, start); err != nil {
		return "", err
	}
	for _, kv := range [][2]string{{"user.name", runnerName}, {"user.email", runnerEmail}} {
		if _, err := g.runEnv(ctx, dir, env, "config", kv[0], kv[1]); err != nil {
			return "", err
		}
	}
	head, err := g.runEnv(ctx, dir, env, "rev-parse", "HEAD")
	return strings.TrimSpace(head), err
}

const (
	runnerName  = "oh runner"
	runnerEmail = "oh-runner@users.noreply.oh"
)

// PushResult is the outcome of Finish.
type PushResult struct {
	Commit string // pushed commit ("" = nothing to push)
	MRURL  string
	// Skipped are the work tree paths left out of the commit (nested
	// repositories, files not owned by the agent account).
	Skipped []string
}

var mrURLRe = regexp.MustCompile(`https?://\S+/-/merge_requests/\d+`)

// Finish commits what the session left uncommitted, then pushes the branch
// to cloneURL and opens a draft merge request to ref through push options
// (no API scope needed). Nothing is pushed when the branch has no commit
// over base. The agent processes must be stopped before (KillUser).
func (g Git) Finish(ctx context.Context, cloneURL, dir, base, ref, branch, title, description string) (*PushResult, error) {
	priv, err := g.scratch("oh-finish-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(priv) }()
	gitDir, err := g.isolate(ctx, dir, priv)
	if err != nil {
		return nil, err
	}
	skipped, err := g.unsafePaths(dir)
	if err != nil {
		return nil, err
	}
	res := &PushResult{Skipped: skipped}
	if len(skipped) > 0 {
		slog.Warn("runner: work tree paths left out of the commit", "paths", skipped)
	}
	env := gitEnv()
	wt := []string{"--git-dir=" + gitDir, "--work-tree=" + dir}
	run := func(args ...string) (string, error) {
		return g.runEnv(ctx, priv, env, append(append([]string{}, wt...), args...)...)
	}

	if _, err := run("read-tree", "HEAD"); err != nil {
		return nil, err
	}
	spec := filepath.Join(priv, "pathspec")
	if err := writePathspec(spec, skipped); err != nil {
		return nil, err
	}
	if _, err := run("add", "-A", "--pathspec-from-file="+spec, "--pathspec-file-nul"); err != nil {
		return nil, err
	}
	if _, err := run("diff", "--cached", "--quiet"); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) || ee.ExitCode() != 1 {
			return nil, err
		}
		if _, err := run("-c", "user.name="+runnerName, "-c", "user.email="+runnerEmail, "-c", "commit.gpgSign=false",
			"commit", "--quiet", "--no-verify", "-m", title); err != nil {
			return nil, err
		}
	}
	head, err := run("rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	head = strings.TrimSpace(head)
	if head == base {
		return res, nil
	}
	args := []string{"--git-dir=" + gitDir, "push", "--force-with-lease",
		"-o", "merge_request.create", "-o", "merge_request.target=" + ref, "-o", "merge_request.draft",
		"-o", "merge_request.title=" + oneLine(title), "-o", "merge_request.remove_source_branch"}
	if description != "" {
		args = append(args, "-o", "merge_request.description="+oneLine(description))
	}
	args = append(args, cloneURL, "HEAD:refs/heads/"+branch)
	out, err := g.authed(ctx, priv, args...)
	if err != nil {
		return nil, err
	}
	res.Commit, res.MRURL = head, mrURLRe.FindString(out)
	return res, nil
}

// writePathspec writes the pathspec of `git add -A`: the whole tree but
// the skipped paths (NUL separated).
func writePathspec(path string, skipped []string) error {
	parts := []string{"."}
	for _, s := range skipped {
		parts = append(parts, ":(exclude,literal)"+s)
	}
	return os.WriteFile(path, []byte(strings.Join(parts, "\x00")), 0o600)
}

// isolate creates a fresh repository under priv holding a copy of the
// object database, the refs and HEAD of the agent's repository (dir/.git),
// never its configuration, hooks, index or alternates; it returns its git
// directory.
func (g Git) isolate(ctx context.Context, dir, priv string) (string, error) {
	src := filepath.Join(dir, ".git")
	st, err := os.Lstat(src)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return "", fmt.Errorf("%s is not a directory", src)
	}
	repo := filepath.Join(priv, "repo")
	if _, err := g.runEnv(ctx, priv, gitEnv(), "init", "--quiet", repo); err != nil {
		return "", err
	}
	dst := filepath.Join(repo, ".git")
	skip := func(rel string) bool {
		switch rel {
		case filepath.Join("objects", "info", "alternates"), filepath.Join("objects", "info", "http-alternates"):
			return true
		}
		return false
	}
	for _, name := range []string{"objects", "refs"} {
		if err := g.copyTree(filepath.Join(src, name), filepath.Join(dst, name), name, skip); err != nil {
			return "", err
		}
	}
	for _, name := range []string{"HEAD", "packed-refs", filepath.Join("info", "exclude")} {
		if err := g.copyFile(filepath.Join(src, name), filepath.Join(dst, name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
	}
	return dst, nil
}

// copyTree copies the directories and the regular files of the agent
// (symbolic links and other files skipped, never followed).
func (g Git) copyTree(src, dst, prefix string, skip func(rel string) bool) error {
	st, err := os.Lstat(src)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return nil
	}
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if skip(filepath.Join(prefix, rel)) {
			return nil
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o700)
		case d.Type().IsRegular():
			return g.copyFile(p, target)
		}
		return nil
	})
}

// copyFile copies a regular file owned by the agent, without following a
// symbolic link; other files are ignored.
func (g Git) copyFile(src, dst string) error {
	f, err := openNoFollow(src)
	if err != nil {
		if isSymlinkErr(err) {
			return nil
		}
		return err
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || !ownedBy(st, g.Agent) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, f); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// unsafePaths lists the work tree paths git must not read as root: nested
// repositories (their .git could point anywhere) and regular files not
// owned by the agent account (hard links to files the agent cannot read).
func (g Git) unsafePaths(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if rel == ".git" {
			return fs.SkipDir
		}
		if d.Name() == ".git" {
			out = append(out, filepath.ToSlash(filepath.Dir(rel)))
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() && g.Agent != nil {
			info, err := d.Info()
			if err != nil {
				return err
			}
			if !ownedBy(info, g.Agent) {
				out = append(out, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	return dedupe(out), err
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 900 {
		s = s[:900]
	}
	return s
}
