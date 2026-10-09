package bundle

import (
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Global guard (v5 critical N3, DC2): shell and edit rules added last to
// every agent, after its own rules and the other guards, whatever its
// permission base allows, also in sessions without locks. They are refused
// in Go, not in the frontmatter: within a pattern map, rules are ordered by
// literal length (ConvertPermissions), so a short deny of the frontmatter
// loses against a longer allow.
//
// What they are: guard rails against the mistakes of a model and against
// cheap tries. What they are not: a sandbox. The tool matches each command
// of a line on its raw text (probes U1–U5, shellsim.go): quotes and spaces
// are not normalized, `export X=…` is not checked, an interpreter or a build
// tool allowed to the agent runs anything. Real confinement comes from the
// checkpoint validation seen by oh, the gateways, the container and the
// credentials given to the session.
//
// Every pattern starts with `*`: environment prefixes (`X=y git …`), `env`,
// `sh -c '…'` and global options (`git -c k=v …`) come first. False
// positives are accepted (`ls ~/.ssh`, a commit message holding ` -an`).

// controlPlaneDeny: the oh daemon, its socket and data (another session's
// bundle, the server passwords, the database).
var controlPlaneDeny = []string{
	"*ohd.sock*", "*--unix-socket*", "*.oh/run*", "*.oh/servers*", "*.oh/bundles*", "*.oh/oh.db*",
	"*oh mcp serve*", "*oh * mcp * serve*",
}

// gitEnvDeny: git environment and transport options that run a program or
// redirect the repository (also refused by name; `export` escapes the name
// check: the oh git calls do not trust the repository anyway, N5).
var gitEnvDeny = []string{
	"*GIT_DIR*", "*GIT_WORK_TREE*", "*GIT_EXTERNAL_DIFF*", "*GIT_SSH*", "*GIT_EDITOR*", "*GIT_SEQUENCE_EDITOR*",
	"*GIT_ASKPASS*", "*GIT_EXEC_PATH*", "*GIT_PAGER*", "*GIT_INDEX_FILE*", "*GIT_TEMPLATE_DIR*",
	"*--upload-pack*", "*--receive-pack*", "*git*push*--exec*", "*git*--output*", "*fsmonitor*", "*git*--ext-diff*",
	"*git*--textconv*", "*git -c *", "*git -C * -c *", "*git*--config-env*", "*BD_GIT_HOOK*",
}

// hookDeny completes HookGuard: shortened `--no-verify`, `-n` among the
// short options of `git commit`, the hook managers of the project.
var hookDeny = []string{
	"*git*--no-ve*", "*.husky/*", "*.githooks/*", "*.beads/hooks*", "*lefthook*install*", "*pre-commit*uninstall*",
}

// commitShortOptions are the short options of `git commit` most often
// combined with -n (`-an`, `-qn`, `-aqn`…). Letters only: `--no-edit` must
// stay allowed. Not exhaustive (`-Xn` with another letter, three options
// before n): a guard rail.
const (
	commitShortOptions = "aqsSvpuezi"
	commitPairOptions  = "aqsv"
)

// commitNoVerifyDeny returns the patterns of `-n` combined with other short
// options of `git commit` (`-n` and `-an` alone are in HookGuard).
func commitNoVerifyDeny() []string {
	var out []string
	for _, x := range commitShortOptions {
		if x != 'a' {
			out = append(out, "*git*commit* -"+string(x)+"n*")
		}
	}
	for _, x := range commitPairOptions {
		for _, y := range commitPairOptions {
			out = append(out, "*git*commit* -"+string(x)+string(y)+"n*")
		}
	}
	return out
}

// execDeny: commands that run others outside the rules of the agent.
var execDeny = []string{
	"*find * -exec*", "*find *-execdir*", "*find *-ok *", "*find *-okdir*", "*find *-delete*", "*find *-fprint*", "*find *-fls*",
	"*xargs*sh *", "*xargs*bash *",
}

// outsideDeny: persistence and credentials outside the project (an external
// directory is checked for `cd` only, probe U4: a redirection or an argument
// writes anywhere).
var outsideDeny = []string{
	"*/.zshenv*", "*/.zshrc*", "*/.zprofile*", "*/.zlogin*", "*/.bashrc*", "*/.bash_profile*", "*/.bash_login*", "*/.profile*",
	"*/.ssh*", "*/.gitconfig*", "*/.config/git/*", "*LaunchAgents*", "*LaunchDaemons*", "*crontab*", "*/.aws/*", "*/.kube/*",
	"*/.docker/config.json*", "*/.netrc*", "*/.npmrc*", "*/.pypirc*", "*/.gnupg*", "*/.config/gh/*", "*/.git-credentials*",
	"*security *find-*password*", "*security *dump-keychain*",
}

// installDeny: system-wide installations, whatever the order of options.
var installDeny = []string{
	"*npm* -g*", "*npm*--global*", "*npm*--location=global*", "*pnpm* -g*", "*pnpm*--global*", "*yarn* global *",
	"*bun* -g*", "*bun*--global*", "*uv*--system*", "*--break-system-packages*", "*sudo *",
}

// dockerDeny: ways out of a container to the machine.
var dockerDeny = []string{
	"*docker*--privileged*", "*docker*--pid*host*", "*docker*--network*host*", "*docker*--net*host*", "*docker*--cap-add*",
	"*docker*--security-opt*", "*docker*--device*", "*docker*.sock*", "*docker* -v /*", "*docker* -v ~*", "*docker*--volume*/*",
	"*docker*--mount*", "*docker*--userns*host*", "*podman*--privileged*", "*podman* -v /*", "*podman*--volume*/*", "*podman*--mount*",
}

// CommitLockExtra are the commands that create a commit without matching
// `*git*commit*` (audit v5 S-12): refused with it while `commit` is locked.
var CommitLockExtra = []string{
	"*git* revert*", "*git* pull*", "*git* merge*", "*git* cherry-pick*", "*git* am *", "*git* rebase*", "*git* stash pop*",
}

// GlobalShellGuard returns the global shell rules of every agent (the git
// guard, GitShellGuard, already matches anywhere in a command). ohHome, when
// it is not ~/.oh, is refused like ~/.oh.
func GlobalShellGuard(ohHome string) []sessionspec.PermissionRule {
	var out []sessionspec.PermissionRule
	add := func(p string, eff sessionspec.Effect) {
		out = append(out, sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: p, Effect: eff})
	}
	lists := [][]string{controlPlaneDeny, gitEnvDeny, hookDeny, commitNoVerifyDeny(), execDeny, outsideDeny, installDeny, dockerDeny}
	for _, l := range lists {
		for _, p := range l {
			add(p, sessionspec.EffectDeny)
		}
	}
	if home := customOhHome(ohHome); home != "" {
		for _, sub := range []string{"run", "servers", "bundles", "oh.db"} {
			add("*"+filepath.ToSlash(filepath.Join(home, sub))+"*", sessionspec.EffectDeny)
		}
	}
	return out
}

// customOhHome returns the oh home to refuse by its path when the generic
// `*.oh/…` patterns do not cover it.
func customOhHome(home string) string {
	home = strings.TrimRight(filepath.ToSlash(home), "/")
	if home == "" || strings.HasSuffix(home, "/.oh") {
		return ""
	}
	return home
}

// GlobalEditGuard returns the global edit rules: the git pointer file of a
// worktree and the git directory are never edited; the hooks of the project
// and the Beads data files that redirect the database ask.
func GlobalEditGuard() []sessionspec.PermissionRule {
	var out []sessionspec.PermissionRule
	add := func(p string, eff sessionspec.Effect) {
		out = append(out, sessionspec.PermissionRule{Action: sessionspec.ActionEdit, Resource: p, Effect: eff})
	}
	for _, p := range []string{".husky/*", ".githooks/*", "lefthook*.yml", ".lefthook*.yml", ".pre-commit-config.yaml"} {
		add(p, sessionspec.EffectAsk)
		add("*/"+p, sessionspec.EffectAsk)
	}
	for _, p := range []string{".git", ".git/*", ".beads/redirect", ".beads/config.yaml", ".beads/hooks/*"} {
		add(p, sessionspec.EffectDeny)
		add("*/"+p, sessionspec.EffectDeny)
	}
	return out
}
