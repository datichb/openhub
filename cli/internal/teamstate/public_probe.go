package teamstate

import (
	"context"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/datichb/openhub/cli/internal/gitremote"
)

// anonymousURL is the address a stranger would read the repository at: the
// HTTP(S) URL as it is, the https one of an SSH remote on the same server.
// ok is false for a local path (a repository of the machine is not public).
func anonymousURL(remote string) (string, bool) {
	remote = strings.TrimSpace(remote)
	if u, err := url.Parse(remote); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
		u.User = nil
		return u.String(), true
	}
	host, path, ok := gitremote.Split(remote)
	if !ok || path == "" {
		return "", false
	}
	return "https://" + host + "/" + path + ".git", true
}

// ProbePublic reports whether the repository can be read by anybody: a read
// with no credential at all (no token, no helper, no key, no prompt) answers.
// The team-state of a team that notifies through a webhook holds that webhook
// in clear text: a public repository publishes it. The error is the case where
// nothing could be said (no network, a local path): not an answer.
func ProbePublic(ctx context.Context, remote string) (bool, error) {
	target, ok := anonymousURL(remote)
	if !ok {
		return false, errors.New("no address to read anonymously")
	}
	pctx, cancel := context.WithTimeout(ctx, remoteProbeTimeout)
	defer cancel()
	cmd := exec.CommandContext(pctx, "git", "-c", "credential.helper=", "-c", "core.askPass=", "-c", "http.extraHeader=",
		"ls-remote", "--exit-code", "--", target, "HEAD")
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "SSH_ASKPASS=", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull,
		"LC_ALL=C")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}
	text := strings.ToLower(string(out))
	for _, refused := range []string{"terminal prompts disabled", "authentication failed", "could not read username", "not found", "403", "401", "access denied", "permission denied"} {
		if strings.Contains(text, refused) {
			return false, nil
		}
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 2 {
		return true, nil // reachable, no HEAD: an empty repository that is open all the same
	}
	return false, errors.New(strings.TrimSpace(string(out)))
}
