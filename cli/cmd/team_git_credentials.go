package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// The access of oh to the git repository of a team-state over HTTPS: the
// token is in the keychain of oh (config.TeamGitTokenKey) and git gets it for
// every operation of the team-state through a one-shot GIT_ASKPASS
// (teamstate.SetCredentialSource): clone, pull, push, ls-remote, and those of
// the daemon. Without a token (SSH, public repository, credential helper of
// the machine) git runs as it always did.

// defaultGitUser is the user name GitLab expects with a personal token.
const defaultGitUser = "oauth2"

// missTTL is how long the answer « no token for this remote » is remembered:
// the keychain is not read again for every pull of a public repository.
const missTTL = 30 * time.Second

// teamGitCredentials reads the git credentials of oh from the secret store.
type teamGitCredentials struct {
	store func() domain.SecretStore

	mu     sync.Mutex
	misses map[string]time.Time
}

// wireTeamGitCredentials makes the token of oh known to the git operations of
// the team-state. store is read at every call: the store of the app can be
// replaced.
func wireTeamGitCredentials(store func() domain.SecretStore) *teamGitCredentials {
	c := &teamGitCredentials{store: store, misses: map[string]time.Time{}}
	teamstate.SetCredentialSource(c.source)
	return c
}

// teamGitCreds is the source wired by the application (for the wizards, which
// forget what they know of a remote once they store a token).
var teamGitCreds *teamGitCredentials

func (c *teamGitCredentials) source(ctx context.Context, remote string) teamstate.GitCredentials {
	key := config.TeamGitTokenKey(remote)
	if key == "" {
		return teamstate.GitCredentials{}
	}
	store := c.store()
	if store == nil {
		return teamstate.GitCredentials{}
	}
	c.mu.Lock()
	missed, known := c.misses[key]
	c.mu.Unlock()
	if known && time.Since(missed) < missTTL {
		return teamstate.GitCredentials{}
	}
	token, err := store.Get(ctx, key)
	if err != nil {
		slog.Debug("team git token not read", "error", err)
		return teamstate.GitCredentials{}
	}
	if token == "" {
		c.mu.Lock()
		c.misses[key] = time.Now()
		c.mu.Unlock()
		return teamstate.GitCredentials{}
	}
	user, err := store.Get(ctx, config.TeamGitUserKey(remote))
	if err != nil || user == "" {
		user = defaultGitUser
	}
	return teamstate.GitCredentials{Username: user, Token: token}
}

// forget drops what is remembered of remote.
func (c *teamGitCredentials) forget(remote string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	delete(c.misses, config.TeamGitTokenKey(remote))
	c.mu.Unlock()
}

// saveTeamGitToken keeps the git token of the repository remote in the
// keychain of oh. On a GitLab host the same token also serves the GitLab API
// for the team (identity check, tracker): the key of the team is set too.
func saveTeamGitToken(ctx context.Context, a *app.App, remote, user, token string) error {
	key := config.TeamGitTokenKey(remote)
	if key == "" {
		return errors.New(i18n.T("v532w.access.not_https"))
	}
	if a == nil || a.Secrets == nil {
		return errors.New(i18n.T("v532w.access.no_keychain"))
	}
	user, token = strings.TrimSpace(user), strings.TrimSpace(token)
	if user == "" {
		user = defaultGitUser
	}
	if err := a.Secrets.Set(ctx, key, token); err != nil {
		return fmt.Errorf(i18n.T("v532w.access.store_failed"), err)
	}
	if err := a.Secrets.Set(ctx, config.TeamGitUserKey(remote), user); err != nil {
		return fmt.Errorf(i18n.T("v532w.access.store_failed"), err)
	}
	teamGitCreds.forget(remote)
	if teamstate.IsGitLabHost(remote) {
		teamID := teamIDFor(a, remote)
		if err := a.Secrets.Set(ctx, config.TeamGitLabTokenKey(teamID), token); err != nil {
			slog.Warn("failed to store GitLab token in keychain", "err", err)
		}
	}
	return nil
}
