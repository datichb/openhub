package bundle

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
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
// bundle, the server passwords, the database, the session folders: the
// reports of workflow_report are read and filed through oh only, L4).
var controlPlaneDeny = []string{
	"*ohd.sock*", "*--unix-socket*", "*.oh/run*", "*.oh/servers*", "*.oh/bundles*", "*.oh/oh.db*", "*.oh/sessions*",
	// the environment of other processes of the user (the tool server holds
	// its password and tokens there)
	"*/proc/*/environ*", "*ps *eww*", "*ps *-E*", "*ps *axe*",
}

// ohCommandPatterns returns the shell patterns of an oh command written with
// the given words (`mcp`, `trust`): the words in a row, or separated by
// options (`oh --no-tui mcp trust`, `oh mcp --x trust`). The leading `*` also
// covers the full path of the binary, `env`, `sh -c '…'` and an environment
// prefix.
func ohCommandPatterns(words ...string) []string {
	var out []string
	for gaps := 0; gaps < 1<<len(words); gaps++ {
		p := "*oh"
		for i, w := range words {
			if gaps&(1<<i) != 0 {
				p += " *"
			}
			p += " " + w
		}
		out = append(out, p+"*")
	}
	return out
}

// ohControlDeny: oh commands an agent must not run from its shell. `mcp serve`
// starts a server with the tokens of the user. The others change what the user
// trusts or his personal settings, which a prompt-injected agent could
// otherwise use to approve a malicious MCP URL (`mcp trust`), swap a model, a
// token or a workflow setting for its own: the secrets (`secrets set|delete`),
// a secret read in clear or the cleanup of the keychain (`secrets get|cleanup`),
// the MCP services (`mcp enable|disable|reset`), the language and the web
// search (`config language|websearch`) and the restrictions of the sessions
// (`budget set|unset|raise`: an agent would lift its own limit). Reads (`mcp
// status`, `config get`, `config list`, `config model show`, `secrets list`,
// `budget show`) stay allowed; `config language` and `config websearch` are
// refused whole, their reading form included (the pattern cannot tell them
// apart).
var ohControlDeny = func() []string {
	out := ohCommandPatterns("mcp", "serve")
	for _, words := range [][]string{
		{"mcp", "trust"}, {"mcp", "untrust"}, {"mcp", "setup"},
		{"config", "set"}, {"config", "unset"},
		{"config", "model", "default"}, {"config", "model", "family"}, {"config", "model", "agent"}, {"config", "model", "unset"},
		{"service", "setup"}, {"service", "remove"},
		{"secrets", "set"}, {"secrets", "delete"}, {"secrets", "get"}, {"secrets", "cleanup"},
		{"mcp", "enable"}, {"mcp", "disable"}, {"mcp", "reset"},
		{"config", "language"}, {"config", "websearch"},
		{"budget", "set"}, {"budget", "unset"}, {"budget", "raise"},
	} {
		out = append(out, ohCommandPatterns(words...)...)
	}
	return out
}()

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
	lists := [][]string{controlPlaneDeny, ohControlDeny, gitEnvDeny, hookDeny, commitNoVerifyDeny(), execDeny, outsideDeny, installDeny, dockerDeny, credentialDeny, scratchShellDeny}
	for _, l := range lists {
		for _, p := range l {
			add(p, sessionspec.EffectDeny)
		}
	}
	if home := customOhHome(ohHome); home != "" {
		for _, sub := range []string{"run", "servers", "bundles", "oh.db", "sessions"} {
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
// worktree, the git directory and the Beads data files that redirect the
// database are never edited. The hooks of the project ask, per agent
// (HookEditGuard): a global ask would loosen the deny of a read-only agent.
func GlobalEditGuard() []sessionspec.PermissionRule {
	var out []sessionspec.PermissionRule
	add := func(p string, eff sessionspec.Effect) {
		out = append(out, sessionspec.PermissionRule{Action: sessionspec.ActionEdit, Resource: p, Effect: eff})
	}
	for _, p := range []string{".git", ".git/*", ".beads/redirect", ".beads/config.yaml", ".beads/hooks/*"} {
		add(p, sessionspec.EffectDeny)
		add("*/"+p, sessionspec.EffectDeny)
	}
	return out
}

// hookEditPatterns are the hook files of the project, each with a path it
// matches (to read the agent's own effect on it).
var hookEditPatterns = []struct{ pattern, sample string }{
	{".husky/*", ".husky/pre-commit"},
	{".githooks/*", ".githooks/pre-commit"},
	{"lefthook*.yml", "lefthook.yml"},
	{".lefthook*.yml", ".lefthook.yml"},
	{".pre-commit-config.yaml", ".pre-commit-config.yaml"},
}

// HookEditGuard returns, for an agent whose own rules let it edit the hook
// files, rules that ask before each edit of them; an agent that may not edit
// them keeps its refusal.
func HookEditGuard(own []sessionspec.PermissionRule) []sessionspec.PermissionRule {
	var out []sessionspec.PermissionRule
	for _, h := range hookEditPatterns {
		for _, p := range []string{h.pattern, "*/" + h.pattern} {
			sample := h.sample
			if p != h.pattern {
				sample = "sub/" + h.sample
			}
			if ToolEffect(own, sessionspec.ActionEdit, sample) == sessionspec.EffectDeny {
				continue
			}
			out = append(out, sessionspec.PermissionRule{Action: sessionspec.ActionEdit, Resource: p, Effect: sessionspec.EffectAsk})
		}
	}
	return out
}

// ── Session scratch (SCR) ───────────────────────────────────────────────────
//
// A workflow gives an agent a scratch folder for the session
// (`agents.<id>.scratch: { readers: […] }`, ScratchDir): outside the
// repository and the documentation, deleted with the session (oh gc). Its
// owner reads and writes it, the declared readers only read it, and only in
// this session: never another session's, parallel sessions included.
//
// The tool asks two permissions before a file tool touches a path outside
// the project: the outside directory (ActionOutsideDir, on `<dir>/*`), then
// the tool action on the path. The agent rules know the agent and not the
// session (a bundle is shared by the sessions of its group), the session
// rules know the session and not the agent, and they come last: so the
// agent is checked on the tool action, the session on the outside
// directory. The guards, in this order, each after the rules of the agent:
//
//  0. sessionDataDeny (L4): the broader refusal of the session data of oh
//     (`*.oh/sessions*`: reports of workflow_report, transcripts, session
//     environment) for the file tools; it is emitted first of scratchDeny (ScratchGuard, ScratchAgentGuard) and of
//     ScratchSessionRules, so that the scratch exceptions still come after
//     it. Added to GlobalEditGuard or to any rule of the global guard
//     (applied by the oh plugin whatever the order, a deny first) it would
//     refuse the owner too: only its shell part may go in GlobalShellGuard
//     (the scratch folders are refused to the shell anyway);
//  1. global refusal of `*.oh/sessions/*/scratch/*`: file tools and outside
//     directory (in a workflow with scratch folders, ScratchAgentGuard for
//     every agent and ScratchSessionRules for every agent of the session;
//     without, ScratchGuard in the global guard), shell (scratchShellDeny
//     in GlobalShellGuard, the owner included: the tool matches a command
//     on its raw text, a shell allow by path would allow any command naming
//     the folder);
//  2. owner: read and write of its folder (ScratchAgentGuard), outside
//     directory of the folder of this session only (ScratchSessionRules);
//  3. readers: read only of the same folder (ScratchAgentGuard), same
//     outside directory (this session only).
//
// A search (glob, grep, list) names a pattern, not a path: only its outside
// directory names the folder, and the session rules allow it to every agent
// of the session. ScratchScopedGuard refuses it to the agents that are
// neither owner nor reader (oh plugin, applied per agent, last); without the
// plugin, an agent of the same session could search the folder (never one
// of another session).

// ActionOutsideDir is the permission asked before a tool touches a path
// outside the project (on the directory, `<dir>/*`).
const ActionOutsideDir = "external_directory"

// Search tools: their resource is a pattern, their path is checked as an
// outside directory.
const (
	actionGlob = "glob"
	actionGrep = "grep"
	actionList = "list"
)

// ScratchDirName is the folder of the scratch folders in a session folder.
const ScratchDirName = "scratch"

// ScratchDir is the scratch folder of an agent for a session:
// <sessionsDir>/<session>/scratch/<agent>.
func ScratchDir(sessionsDir, sessionID, agent string) string {
	return filepath.Join(sessionsDir, sessionID, ScratchDirName, agent)
}

// sessionDataDeny: refusals of the session data of oh broader than the
// scratch folders (layer 0 above), for the file tools and the outside
// directory: no agent reads nor forges a stored report (L4), a transcript
// or the environment of a session. The custom oh home is added by
// sessionDataPatterns, the sessions folder of the session rules by
// ScratchSessionRules.
var sessionDataDeny = []string{"*.oh/sessions*"}

// sessionDataPatterns are the layer 0 patterns, with the custom oh home.
func sessionDataPatterns(ohHome string) []string {
	out := append([]string(nil), sessionDataDeny...)
	if home := customOhHome(ohHome); home != "" {
		out = append(out, "*"+filepath.ToSlash(filepath.Join(home, "sessions"))+"*")
	}
	return out
}

// scratchShellDeny: shell commands naming a scratch folder (layer 1).
var scratchShellDeny = []string{"*.oh/sessions/*/scratch*"}

// scratchFileActions are the actions refused on any scratch folder; the
// owner and the readers get all but edit back on theirs.
var scratchFileActions = []string{sessionspec.ActionRead, sessionspec.ActionEdit, actionGlob, actionGrep, actionList, ActionOutsideDir}

// scratchRoots are the patterns of the session folders of oh: generic, and
// the custom oh home when it is not ~/.oh.
func scratchRoots(ohHome string) []string {
	roots := []string{"*.oh/sessions/"}
	if home := customOhHome(ohHome); home != "" {
		roots = append(roots, "*"+filepath.ToSlash(filepath.Join(home, "sessions"))+"/")
	}
	return roots
}

// scratchDeny returns the refusal of every scratch folder (layers 0 and 1)
// on the file tools and the outside directory.
func scratchDeny(ohHome string) []sessionspec.PermissionRule {
	var out []sessionspec.PermissionRule
	patterns := sessionDataPatterns(ohHome)
	for _, root := range scratchRoots(ohHome) {
		patterns = append(patterns, root+"*/"+ScratchDirName+"/*")
	}
	for _, p := range patterns {
		for _, a := range scratchFileActions {
			out = append(out, sessionspec.PermissionRule{Action: a, Resource: p, Effect: sessionspec.EffectDeny})
		}
	}
	return out
}

// ScratchGuard returns the scratch rules of the global guard of a bundle:
// in a workflow without scratch folder, no agent has an exception, so the
// refusal (layers 0 and 1) is given once for all agents. Nil otherwise
// (ScratchAgentGuard).
func ScratchGuard(wf *sessionspec.WorkflowRuntime, ohHome string) []sessionspec.PermissionRule {
	if len(scratchOf(wf)) > 0 {
		return nil
	}
	return scratchDeny(ohHome)
}

// ScratchAgentGuard returns, in a workflow with scratch folders, the scratch
// rules of an agent (layers 0 to 3), appended last to its rules: every
// agent is refused the scratch folders; the owner of a folder reads and
// writes it, its readers read it. The session is checked by
// ScratchSessionRules. Nil without scratch folder (ScratchGuard).
func ScratchAgentGuard(agent string, wf *sessionspec.WorkflowRuntime, ohHome string) []sessionspec.PermissionRule {
	if len(scratchOf(wf)) == 0 {
		return nil
	}
	out := scratchDeny(ohHome)
	for _, sc := range scratchOf(wf) {
		owner := sc.Owner == agent
		if !owner && !slices.Contains(sc.Readers, agent) {
			continue
		}
		for _, root := range scratchRoots(ohHome) {
			dir := root + "*/" + ScratchDirName + "/" + sc.Owner + "/*"
			for _, a := range scratchFileActions {
				if a == ActionOutsideDir || (a == sessionspec.ActionEdit && !owner) {
					continue // outside directory: ScratchSessionRules
				}
				out = append(out, sessionspec.PermissionRule{Action: a, Resource: dir, Effect: sessionspec.EffectAllow})
			}
		}
	}
	return out
}

// ScratchSessionRules returns the scratch rules of a session (layers 0 to
// 3 on the outside directory), appended last to its session rules: the
// folders of this session are reachable, those of the other sessions are
// not. Nil for a workflow without scratch folder (ScratchAgentGuard
// already refuses them all).
func ScratchSessionRules(wf *sessionspec.WorkflowRuntime, sessionsDir, sessionID string) []sessionspec.PermissionRule {
	scratch := scratchOf(wf)
	if len(scratch) == 0 {
		return nil
	}
	var out []sessionspec.PermissionRule
	add := func(p string, eff sessionspec.Effect) {
		out = append(out, sessionspec.PermissionRule{Action: ActionOutsideDir, Resource: p, Effect: eff})
	}
	for _, p := range sessionDataDeny {
		add(p, sessionspec.EffectDeny)
	}
	if sessionsDir != "" {
		add(filepath.ToSlash(sessionsDir)+"*", sessionspec.EffectDeny)
	}
	add("*.oh/sessions/*/"+ScratchDirName+"/*", sessionspec.EffectDeny)
	if sessionsDir == "" || sessionID == "" {
		return out
	}
	add(filepath.ToSlash(filepath.Join(sessionsDir, "*", ScratchDirName))+"/*", sessionspec.EffectDeny)
	for _, sc := range scratch {
		add(filepath.ToSlash(ScratchDir(sessionsDir, sessionID, sc.Owner))+"/*", sessionspec.EffectAllow)
	}
	return out
}

// ScratchScopedGuard returns the per-agent refusals of the scratch folders
// of wf (searches by an agent that is neither owner nor reader, edits by
// another agent than the owner), applied last by the oh plugin.
func ScratchScopedGuard(wf *sessionspec.WorkflowRuntime, ohHome string) []sessionspec.ScopedRule {
	var out []sessionspec.ScopedRule
	for _, sc := range scratchOf(wf) {
		readers := append([]string{sc.Owner}, sc.Readers...)
		for _, root := range scratchRoots(ohHome) {
			dir := root + "*/" + ScratchDirName + "/" + sc.Owner + "/*"
			for _, a := range scratchFileActions {
				except := readers
				if a == sessionspec.ActionEdit {
					except = []string{sc.Owner}
				}
				out = append(out, sessionspec.ScopedRule{
					PermissionRule: sessionspec.PermissionRule{Action: a, Resource: dir, Effect: sessionspec.EffectDeny},
					Except:         except,
				})
			}
		}
	}
	return out
}

// ScopedGuardFor returns the scoped guard rules that apply to agent.
func ScopedGuardFor(agent string, rules []sessionspec.ScopedRule) []sessionspec.PermissionRule {
	var out []sessionspec.PermissionRule
	for _, r := range rules {
		if !slices.Contains(r.Except, agent) {
			out = append(out, r.PermissionRule)
		}
	}
	return out
}

// scratchDefs returns the scratch folders of a workflow (nil = none).
func scratchDefs(s *workflow.Spec) []sessionspec.ScratchDef {
	if s == nil {
		return nil
	}
	var out []sessionspec.ScratchDef
	for _, id := range s.ScratchOwners() {
		a, _ := s.Agents.Get(id)
		out = append(out, sessionspec.ScratchDef{Owner: id, Readers: append([]string(nil), a.Scratch.Readers...)})
	}
	return out
}

func scratchOf(wf *sessionspec.WorkflowRuntime) []sessionspec.ScratchDef {
	if wf == nil {
		return nil
	}
	return wf.Scratch
}

// scratchNote tells the owner and the readers of a scratch folder where to
// find it (the path depends on the session: session context entry
// oh.scratch). "" for the other agents.
func scratchNote(agent string, wf *sessionspec.WorkflowRuntime) string {
	var owns bool
	var reads []string
	for _, sc := range scratchOf(wf) {
		if sc.Owner == agent {
			owns = true
		} else if slices.Contains(sc.Readers, agent) {
			reads = append(reads, "`"+sc.Owner+"`")
		}
	}
	var parts []string
	if owns {
		parts = append(parts, "tu as un dossier de brouillons pour cette session (lecture et écriture)")
	}
	if len(reads) > 0 {
		parts = append(parts, "tu peux lire, sans les modifier, les brouillons de "+strings.Join(reads, ", ")+" de cette session")
	}
	if len(parts) == 0 {
		return ""
	}
	return "\n\n> Brouillons de session (oh) : " + strings.Join(parts, " ; ") +
		". Chemin : entrée `oh.scratch` du contexte de session. Hors dépôt, supprimés avec la session : jamais cités dans la documentation ni le wiki.\n"
}
