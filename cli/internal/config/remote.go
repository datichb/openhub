package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// RemoteConfig configures the remote runtime (GitLab CI runners, v5 phase 5).
type RemoteConfig struct {
	// Targets are the oh-runner projects, one per GitLab instance and group.
	Targets []RemoteTarget `mapstructure:"targets" toml:"targets,omitempty"`
	// Projects maps an oh project ID to a target name, overriding the
	// automatic choice (instance and group of the project's git remote).
	Projects map[string]string `mapstructure:"projects" toml:"projects,omitempty"`
}

// RemoteTarget is one oh-runner project. Secrets are referenced by keychain
// key names, never stored here.
type RemoteTarget struct {
	// Name identifies the target (kebab-case, unique).
	Name string `mapstructure:"name" toml:"name"`
	// URL is the GitLab instance (https://gitlab.com, https://gitlab.example.com).
	URL string `mapstructure:"url" toml:"url"`
	// Group is the full path of the group whose projects the target serves.
	Group string `mapstructure:"group" toml:"group"`
	// RunnerProject is the full path of the oh-runner project (default <group>/oh-runner).
	RunnerProject string `mapstructure:"runner_project" toml:"runner_project,omitempty"`
	// RunnerProjectID is the numeric ID of the oh-runner project (set by setup).
	RunnerProjectID int64 `mapstructure:"runner_project_id" toml:"runner_project_id,omitempty"`
	// TokenKey is the keychain key of the API token of the member (api scope):
	// setup, sending packages, triggering, following pipelines, artifacts.
	TokenKey string `mapstructure:"token_key" toml:"token_key,omitempty"`
	// TriggerKey is the keychain key of the pipeline trigger token.
	TriggerKey string `mapstructure:"trigger_key" toml:"trigger_key,omitempty"`
	// Tag is the runner tag of the oh jobs (default "oh").
	Tag string `mapstructure:"tag" toml:"tag,omitempty"`
	// Builder builds the project image on the runner: kaniko (default) | dind.
	Builder string `mapstructure:"builder" toml:"builder,omitempty"`
	// Arch is the runner architecture: amd64 (default) | arm64.
	Arch string `mapstructure:"arch" toml:"arch,omitempty"`
	// Timeout of the run job (GitLab duration, default 3h).
	Timeout string `mapstructure:"timeout" toml:"timeout,omitempty"`
	// Binaries maps "<oh version>/<arch>" to the generic package version
	// (SHA-256) of a development oh binary uploaded by setup.
	Binaries map[string]string `mapstructure:"binaries" toml:"binaries,omitempty"`
}

var remoteTargetNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// Validate checks the required fields.
func (t RemoteTarget) Validate() error {
	if !remoteTargetNameRe.MatchString(t.Name) {
		return fmt.Errorf("invalid remote target name %q (lowercase letters, digits and dashes)", t.Name)
	}
	u, err := url.Parse(t.URL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || (u.Path != "" && u.Path != "/") {
		return fmt.Errorf("invalid GitLab URL %q (https://host)", t.URL)
	}
	if strings.Trim(t.Group, "/") == "" || strings.Contains(t.Group, "..") {
		return fmt.Errorf("invalid GitLab group %q", t.Group)
	}
	return nil
}

// RunnerProjectPath returns the oh-runner project path (default <group>/oh-runner).
func (t RemoteTarget) RunnerProjectPath() string {
	if t.RunnerProject != "" {
		return t.RunnerProject
	}
	return strings.Trim(t.Group, "/") + "/oh-runner"
}

// Host returns the host[:port] of the instance.
func (t RemoteTarget) Host() string {
	u, err := url.Parse(t.URL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Host)
}

// DefaultRemoteTokenKey is the keychain key of a target's API token.
func DefaultRemoteTokenKey(target string) string { return "openhub.remote." + target + ".token" }

// DefaultRemoteTriggerKey is the keychain key of a target's trigger token.
func DefaultRemoteTriggerKey(target string) string { return "openhub.remote." + target + ".trigger" }

// TokenKeyOrDefault returns TokenKey or the default key of the target.
func (t RemoteTarget) TokenKeyOrDefault() string {
	if t.TokenKey != "" {
		return t.TokenKey
	}
	return DefaultRemoteTokenKey(t.Name)
}

// TriggerKeyOrDefault returns TriggerKey or the default key of the target.
func (t RemoteTarget) TriggerKeyOrDefault() string {
	if t.TriggerKey != "" {
		return t.TriggerKey
	}
	return DefaultRemoteTriggerKey(t.Name)
}

// Target returns the target named name (nil if absent).
func (r *RemoteConfig) Target(name string) *RemoteTarget {
	for i := range r.Targets {
		if r.Targets[i].Name == name {
			return &r.Targets[i]
		}
	}
	return nil
}

// Upsert adds or replaces the target with the same name.
func (r *RemoteConfig) Upsert(t RemoteTarget) {
	if cur := r.Target(t.Name); cur != nil {
		*cur = t
		return
	}
	r.Targets = append(r.Targets, t)
}

// GitRemote is a parsed git remote URL.
type GitRemote struct {
	Host string // host[:port] (lowercase; SSH port dropped)
	Path string // full project path, without .git
}

var scpLikeRe = regexp.MustCompile(`^(?:[^@/]+@)?([^:/]+):(.+)$`)

// ParseGitRemote parses https://host/group/p.git, ssh://git@host:2222/group/p.git
// and git@host:group/p.git.
func ParseGitRemote(raw string) (GitRemote, error) {
	raw = strings.TrimSpace(raw)
	var host, path string
	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return GitRemote{}, err
		}
		host = u.Host
		if u.Scheme == "ssh" || u.Scheme == "git+ssh" {
			host = u.Hostname()
		}
		path = u.Path
	} else if m := scpLikeRe.FindStringSubmatch(raw); m != nil {
		host, path = m[1], m[2]
	} else {
		return GitRemote{}, fmt.Errorf("unrecognized git remote %q", raw)
	}
	path = strings.TrimSuffix(strings.Trim(path, "/"), ".git")
	if host == "" || path == "" {
		return GitRemote{}, fmt.Errorf("unrecognized git remote %q", raw)
	}
	return GitRemote{Host: strings.ToLower(host), Path: path}, nil
}

// MatchTarget chooses the target of a project: the explicit Projects entry,
// else the target of the same instance whose group contains the project
// (the deepest group wins). nil when none matches.
func (r *RemoteConfig) MatchTarget(projectID string, remote GitRemote) *RemoteTarget {
	if name, ok := r.Projects[projectID]; ok {
		return r.Target(name)
	}
	var best *RemoteTarget
	for i := range r.Targets {
		t := &r.Targets[i]
		if !sameHost(t.Host(), remote.Host) {
			continue
		}
		g := strings.Trim(t.Group, "/")
		if !strings.HasPrefix(remote.Path, g+"/") {
			continue
		}
		if best == nil || len(g) > len(strings.Trim(best.Group, "/")) {
			best = t
		}
	}
	return best
}

// sameHost compares hosts, ignoring the port of the HTTP side (SSH remotes
// carry no HTTP port).
func sameHost(a, b string) bool {
	strip := func(h string) string {
		if i := strings.LastIndexByte(h, ':'); i >= 0 {
			return h[:i]
		}
		return h
	}
	return a == b || strip(a) == strip(b)
}
