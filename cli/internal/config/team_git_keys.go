package config

import (
	"net/url"
	"strings"
)

// Keychain keys of the access of oh to the git repository of a team-state
// over HTTPS. They are per repository (host and path), not per team: two teams
// whose repositories are named alike on different servers do not share a
// token, and the token of the git access is not the GitLab API token of the
// team (TeamGitLabTokenKey), even when the user gives the same value.
//
//	openhub.git.<host>/<path>.token   the token (or password)
//	openhub.git.<host>/<path>.user    the user name (oauth2 for GitLab)
//
// <path> is the path of the repository without .git and without the slashes
// around it, <host> the host (and port) in lower case: the same repository
// written with or without .git has the same keys. They are given to git by
// GIT_ASKPASS for every operation on the team-state (see teamstate).

// GitRepoKeyBase is the common part of the keys of the repository remote, ""
// when remote is not an HTTP(S) URL (SSH remotes have no token).
func GitRepoKeyBase(remote string) string {
	u, err := url.Parse(strings.TrimSpace(remote))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return ""
	}
	path := strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git")
	return "openhub.git." + strings.ToLower(u.Host) + "/" + path
}

// TeamGitTokenKey is the keychain key of the git token of the repository
// remote ("" when remote is not an HTTP(S) URL).
func TeamGitTokenKey(remote string) string {
	if base := GitRepoKeyBase(remote); base != "" {
		return base + ".token"
	}
	return ""
}

// TeamGitUserKey is the keychain key of the git user name of the repository
// remote ("" when remote is not an HTTP(S) URL).
func TeamGitUserKey(remote string) string {
	if base := GitRepoKeyBase(remote); base != "" {
		return base + ".user"
	}
	return ""
}
