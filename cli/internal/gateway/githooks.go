package gateway

import (
	"regexp"
	"strings"
)

// Git hooks of Beads (A19): `bd init` installs hooks that call
// `bd hooks run <hook>` with BD_GIT_HOOK=1. In a session they reach the
// fake bd, hence the gateway: they run whatever the workflow beads.allow
// says, so that a commit is never blocked (nor the hooks bypassed) because
// of the allow-list.
//
// The marker BD_GIT_HOOK is set by whoever runs bd (audit v5, S-06): the
// hook mode needs a one-time hook token that the daemon issues only to a
// fake bd that git runs from the hook of the session location (hooktoken.go),
// and only for `hooks run <hook>` with the arguments git passes to it.

// gitHooks are the hooks of bd 1.3 (bd hooks run --help) and the shape of
// the arguments git passes to each of them.
var gitHooks = map[string]func(args []string) bool{
	"pre-commit": func(a []string) bool { return len(a) == 0 },
	"prepare-commit-msg": func(a []string) bool {
		switch len(a) {
		case 1:
			return plainArg(a[0])
		case 2:
			return plainArg(a[0]) && commitMsgSources[a[1]] && a[1] != "commit"
		case 3:
			return plainArg(a[0]) && a[1] == "commit" && objectID.MatchString(a[2])
		}
		return false
	},
	"post-merge": func(a []string) bool { return len(a) == 1 && (a[0] == "0" || a[0] == "1") },
	"post-checkout": func(a []string) bool {
		return len(a) == 3 && objectID.MatchString(a[0]) && objectID.MatchString(a[1]) && (a[2] == "0" || a[2] == "1")
	},
	"pre-push": func(a []string) bool { return len(a) == 2 && plainArg(a[0]) && plainArg(a[1]) },
}

var (
	objectID         = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	commitMsgSources = map[string]bool{"message": true, "template": true, "merge": true, "squash": true, "commit": true}
)

// plainArg is a path or a remote: not a flag, on one line.
func plainArg(s string) bool {
	return s != "" && !strings.HasPrefix(s, "-") && !strings.ContainsAny(s, "\x00\n\r")
}

// gitHookAllow is the allow-list of a command run by a Beads git hook.
var gitHookAllow = []string{"hooks run"}

// GitHookName returns the hook of argv when it is exactly `bd hooks run
// <known hook> <arguments git passes to it>` (no other flag), "" otherwise.
func GitHookName(argv []string) string {
	if len(argv) < 3 || argv[0] != "hooks" || argv[1] != "run" {
		return ""
	}
	shape, ok := gitHooks[argv[2]]
	if !ok || !shape(argv[3:]) {
		return ""
	}
	return argv[2]
}
