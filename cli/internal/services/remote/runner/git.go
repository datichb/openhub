package runner

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Git runs git in the job with the project token. The token goes through
// the environment of the git process (GIT_CONFIG_*), never in a command
// line, a URL or a config file.
type Git struct {
	Token   string
	Secrets Secrets // redacted from errors
}

func (g Git) env() []string {
	auth := base64.StdEncoding.EncodeToString([]byte("oauth2:" + g.Token))
	return append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=http.extraHeader",
		"GIT_CONFIG_VALUE_0=Authorization: Basic "+auth,
		// The work tree belongs to the tool server account.
		"GIT_CONFIG_KEY_1=safe.directory",
		"GIT_CONFIG_VALUE_1=*",
	)
}

func (g Git) run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = g.env()
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		return out.String() + errOut.String(), fmt.Errorf("git %s: %s", args[0], g.Secrets.Redact(strings.TrimSpace(errOut.String()+" "+err.Error())))
	}
	return out.String() + errOut.String(), nil
}

// Clone clones ref of cloneURL into dir and starts branch at commit (ref's
// head when empty).
func (g Git) Clone(ctx context.Context, cloneURL, ref, commit, branch, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if _, err := g.run(ctx, dir, "clone", "--quiet", "--single-branch", "--branch", ref, cloneURL, "."); err != nil {
		return "", err
	}
	start := commit
	if start == "" {
		start = "HEAD"
	}
	if _, err := g.run(ctx, dir, "checkout", "--quiet", "-B", branch, start); err != nil {
		return "", err
	}
	for _, kv := range [][2]string{{"user.name", "oh runner"}, {"user.email", "oh-runner@users.noreply.oh"}} {
		if _, err := g.run(ctx, dir, "config", kv[0], kv[1]); err != nil {
			return "", err
		}
	}
	head, err := g.run(ctx, dir, "rev-parse", "HEAD")
	return strings.TrimSpace(head), err
}

// PushResult is the outcome of Finish.
type PushResult struct {
	Commit string // pushed commit ("" = nothing to push)
	MRURL  string
}

var mrURLRe = regexp.MustCompile(`https?://\S+/-/merge_requests/\d+`)

// Finish commits what the session left uncommitted, then pushes the branch
// and opens a draft merge request to ref through push options (no API
// scope needed). Nothing is pushed when the branch has no commit over base.
func (g Git) Finish(ctx context.Context, dir, base, ref, branch, title, description string) (*PushResult, error) {
	if out, err := g.run(ctx, dir, "status", "--porcelain"); err != nil {
		return nil, err
	} else if strings.TrimSpace(out) != "" {
		if _, err := g.run(ctx, dir, "add", "-A"); err != nil {
			return nil, err
		}
		if _, err := g.run(ctx, dir, "commit", "--quiet", "--no-verify", "-m", title); err != nil {
			return nil, err
		}
	}
	head, err := g.run(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	head = strings.TrimSpace(head)
	if head == base {
		return &PushResult{}, nil
	}
	args := []string{"push", "--force-with-lease",
		"-o", "merge_request.create", "-o", "merge_request.target=" + ref, "-o", "merge_request.draft",
		"-o", "merge_request.title=" + oneLine(title), "-o", "merge_request.remove_source_branch"}
	if description != "" {
		args = append(args, "-o", "merge_request.description="+oneLine(description))
	}
	args = append(args, "origin", "HEAD:refs/heads/"+branch)
	out, err := g.run(ctx, dir, args...)
	if err != nil {
		return nil, err
	}
	return &PushResult{Commit: head, MRURL: mrURLRe.FindString(out)}, nil
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 900 {
		s = s[:900]
	}
	return s
}
