package gateway

// Git hooks of Beads (A19): `bd init` installs hooks that call
// `bd hooks run <hook>` with BD_GIT_HOOK=1. In a session they reach the
// fake bd, hence the gateway: they run whatever the workflow beads.allow
// says, so that a commit is never blocked (nor the hooks bypassed) because
// of the allow-list.

// gitHooks are the hooks of bd 1.3 (bd hooks run --help).
var gitHooks = map[string]bool{
	"pre-commit": true, "post-merge": true, "post-checkout": true, "pre-push": true, "prepare-commit-msg": true,
}

// gitHookAllow is the allow-list of a command run by a Beads git hook.
var gitHookAllow = []string{"hooks run"}

// isGitHook reports whether argv is `bd hooks run <known hook> …` (global
// flags before `hooks` are checked by checkBeads).
func isGitHook(argv []string) bool {
	for i, a := range argv {
		if a == "hooks" {
			return i+2 < len(argv) && argv[i+1] == "run" && gitHooks[argv[i+2]]
		}
	}
	return false
}
