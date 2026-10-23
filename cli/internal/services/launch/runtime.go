package launch

import (
	"os/exec"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// RemoteExperimental reports whether the remote mode is enabled (it is
// experimental in 5.0: off unless `[remote] experimental = true`).
func RemoteExperimental(cfg *config.Config) bool {
	return cfg != nil && cfg.Remote.Experimental
}

// RuntimePrefs are the preferred runtimes of a launch without --runtime,
// most specific first: the project default, then the Settings one. The
// workflow keeps the last word (`runtime.allowed`, then `runtime.default`).
func RuntimePrefs(cfg *config.Config, p *domain.Project) []string {
	var def string
	if p != nil && p.ExecConfig != nil {
		def = p.ExecConfig.DefaultRuntime
	}
	prefs := []string{def}
	if cfg != nil {
		prefs = append(prefs, cfg.Execution.Runtime)
	}
	return prefs
}

// PickRuntime chooses the runtime of a launch from the preferences
// (project, Settings) and the workflow; the remote runtime is not chosen by
// default while it is disabled, unless the workflow allows nothing else (the
// launch is then refused with the experimental message).
func PickRuntime(cfg *config.Config, sp *workflow.Spec, project *domain.Project) workflow.Runtime {
	rt := sp.PickRuntime(RuntimePrefs(cfg, project)...)
	if rt != workflow.RuntimeRemote || RemoteExperimental(cfg) {
		return rt
	}
	for _, r := range sp.AllowedRuntimes() {
		if r != workflow.RuntimeRemote {
			return r
		}
	}
	return rt
}

// gitIdentityEnv is the git identity of the machine for the shell of a
// container session: the container HOME is a project volume (the user git
// config is not there), so commits would fail without it. Read from the
// project (its local config, then the global one); empty when unset.
func gitIdentityEnv(dir string) map[string]string {
	get := func(key string) string {
		out, err := exec.Command("git", "-C", dir, "config", "--get", key).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	env := map[string]string{}
	if name := get("user.name"); name != "" {
		env["GIT_AUTHOR_NAME"], env["GIT_COMMITTER_NAME"] = name, name
	}
	if email := get("user.email"); email != "" {
		env["GIT_AUTHOR_EMAIL"], env["GIT_COMMITTER_EMAIL"] = email, email
	}
	return env
}
