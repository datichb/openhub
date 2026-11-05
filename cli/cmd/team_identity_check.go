package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// remoteRef stands for a team-state that is not cloned yet when only its remote
// is needed (the GitLab the identity is verified on).
type remoteRef string

func (r remoteRef) Path() string      { return "" }
func (r remoteRef) Remote() string    { return string(r) }
func (r remoteRef) IsCloned() bool    { return false }
func (r remoteRef) HasConfig() bool   { return false }
func (r remoteRef) HasPolicies() bool { return false }

// precheckIdentity verifies the GitLab identity of the card m before it is
// published in the team-state: the token oh holds for the GitLab of the
// repository must be the one of the handle on the card. It is the check
// teamRejoinCore makes after the rejoin, moved before the write: a card with
// somebody else's handle is not pushed without the person knowing. The result
// is validateGitLabIdentityAt's: nil (verified, or nothing to verify with),
// *identityMismatchError, *identityUnverifiedError.
func precheckIdentity(ctx context.Context, a *app.App, remote, gitlabURL string, m teamstate.Member) error {
	return validateGitLabIdentityAt(ctx, a, remoteRef(remote), &m, firstNonEmpty(gitlabURL, registeredGitLabURL(a, remote)))
}
