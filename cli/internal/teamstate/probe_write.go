package teamstate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// A read probe (ProbeRemote) says the repository answers; it does not say the
// token may write: a GitLab token with read_repository only is accepted by
// ls-remote and refused by the first push, long after the assistant of « Créer
// une équipe » asked everything. ProbeRemoteWrite asks the server the way a push
// does (`git push --dry-run`: the same authentication of the receive-pack
// service, nothing is sent, nothing is created).

// ProbeRemoteWrite checks that remote accepts a push, with what git already
// has and the token oh holds for it. The repository can be empty. A refusal
// of the access, read-only included, is an *AuthError.
func ProbeRemoteWrite(ctx context.Context, remote string) error {
	return probeRemoteWrite(ctx, remote, nil)
}

// ProbeRemoteWriteAs is ProbeRemoteWrite with the credentials given instead of
// the ones oh holds: a token is checked before it is kept.
func ProbeRemoteWriteAs(ctx context.Context, remote string, creds GitCredentials) error {
	return probeRemoteWrite(ctx, remote, &creds)
}

// probeIdentity is the author of the empty commit the dry run pushes: git
// refuses to commit without one, and the commit never leaves the machine.
var probeIdentity = []string{
	"GIT_AUTHOR_NAME=oh", "GIT_AUTHOR_EMAIL=oh@localhost",
	"GIT_COMMITTER_NAME=oh", "GIT_COMMITTER_EMAIL=oh@localhost",
}

func probeRemoteWrite(ctx context.Context, remote string, creds *GitCredentials) error {
	if strings.TrimSpace(remote) == "" {
		return errors.New("no repository")
	}
	dir, err := os.MkdirTemp("", "oh-probe-")
	if err != nil {
		return fmt.Errorf("probe folder: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	pctx, cancel := context.WithTimeout(ctx, remoteProbeTimeout)
	defer cancel()
	local := func(args ...string) error {
		cmd, cleanup := gitCommand(pctx, "", dir, nil, probeSSHOptions, probeIdentity, args...)
		defer cleanup()
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
		}
		return nil
	}
	if err := local("init", "-q"); err != nil {
		return err
	}
	if err := local("-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "--no-verify", "-m", "oh probe"); err != nil {
		return err
	}
	cmd, cleanup := gitCommand(pctx, remote, dir, creds, probeSSHOptions, probeHTTPEnv,
		"push", "--dry-run", "--no-verify", "--", remote, "HEAD:refs/heads/oh-probe")
	defer cleanup()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return probeError(pctx, ctx, remote, "push --dry-run", out, err)
	}
	return nil
}
