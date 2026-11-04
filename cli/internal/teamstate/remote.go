package teamstate

import (
	"github.com/datichb/openhub/cli/internal/gitremote"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// RemoteMismatchError: the folder of a clone holds the clone of another
// repository than the one asked for. It is never reused: the wizards say which
// folder and which repositories, and leave the choice to the user.
type RemoteMismatchError struct {
	Path string // the folder of the clone
	Want string // the repository asked for
	Have string // the origin of the clone ("" when it has none)
}

func (e *RemoteMismatchError) Error() string {
	have := e.Have
	if have == "" {
		have = i18n.T("v532w.clone.no_origin")
	}
	return i18n.Tf("v532w.clone.other_remote", e.Path, have, e.Want)
}

// CheckRemote verifies that an existing clone is the clone of the repository
// of r: its origin is that repository (the same host and path, whatever the
// scheme, the .git suffix or the user). A repository made without a remote
// (a solo team-state) or not cloned yet always passes.
func (r *Repo) CheckRemote() error {
	if r.remote == "" || !r.IsCloned() {
		return nil
	}
	have, _ := originURL(r.path)
	if have != "" && SameRemote(have, r.remote) {
		return nil
	}
	return &RemoteMismatchError{Path: r.path, Want: r.remote, Have: have}
}

// originURL reads the URL of the origin of the clone at path from
// .git/config (no git process).
func originURL(path string) (string, bool) { return gitremote.Origin(path) }

// SameRemote reports whether a and b designate the same repository: same host
// and same path, whatever the scheme (https, ssh, scp-like), the user, the
// .git suffix, the trailing slash or the case of the host. Local paths are
// compared as cleaned paths.
func SameRemote(a, b string) bool { return gitremote.Same(a, b) }
