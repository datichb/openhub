package bundle

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// effectiveRules are the rules the tool applies to an agent of a bundle:
// its own rules, then the guard of the bundle (last).
func effectiveRules(t *testing.T, entry, agent string) []sessionspec.PermissionRule {
	t.Helper()
	b, err := Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), Spec: agentSpec(t, repoHub(t), entry), Provider: "bedrock"})
	require.NoError(t, err)
	for _, a := range b.Spec.Agents {
		if a.ID == agent {
			return append(append([]sessionspec.PermissionRule(nil), a.Permissions...), b.Spec.AgentGuard...)
		}
	}
	t.Fatalf("agent %s not in the bundle of %s", agent, entry)
	return nil
}

// N3 (v5 critical, DC2): the cheap ways around the shell rules of the
// analysis (S2.3 and audit S-12) are refused to every shipped agent, as the
// tool matches them (simulator of lot 0).
func TestGlobalGuard(t *testing.T) {
	bypasses := []string{
		// destructive git anywhere in the command
		"X=1 git checkout app/store.py", "env git checkout app/store.py", "sh -c 'git checkout app/store.py'",
		"git -c a.b=c checkout app/store.py", "git -C /tmp/p restore x", "true && git reset --hard",
		// git options and environment that run a program or write a file
		"git log --output=/tmp/x", "git fetch --upload-pack='touch /tmp/p' origin", "git diff --ext-diff",
		"GIT_EXTERNAL_DIFF=/tmp/x git diff", "git -c core.fsmonitor=/tmp/x status", "GIT_DIR=/tmp/x git status",
		"git push --receive-pack=/tmp/x", "GIT_SSH_COMMAND=/tmp/x git fetch", "git --config-env=core.pager=X log",
		// hooks
		"git commit -qn -m x", "git commit -an -m x", "git commit -aqn -m x", "git commit -sn -m x", "git commit --no-veri -m x",
		"rm -rf .beads/hooks", "BD_GIT_HOOK=1 bd hooks run post-merge x", "echo x > .husky/pre-commit",
		// the oh control plane
		"curl --unix-socket ~/.oh/run/ohd.sock http://ohd/v1/workflow/checkpoint", "cat ~/.oh/run/server-passwords/x.pw",
		"cat ~/.oh/oh.db", "ls ~/.oh/bundles", "cat ~/.oh/sessions/ses_a/reports/rp-1.md", "echo forgé > $HOME/.oh/sessions/ses_a/reports/rp-2.md", "ps eww -p 123", "cat /proc/1/environ", "oh mcp serve workflow", "/usr/local/bin/oh mcp serve workflow",
		// commands that run others
		"find . -name '*.go' -exec rm {} ;", "find . -delete",
		// persistence and credentials outside the project
		"echo 'export PATH=/x' >> ~/.zshenv", "cat ~/.ssh/id_rsa", "cat $HOME/.aws/credentials", "cat ~/.kube/config",
		"cp x ~/.gitconfig", "crontab -l",
		// system installations
		"npm install --location=global x", "npm i x -g", "pnpm -g add x", "yarn global add x", "uv pip install --system x",
		"pip install x --break-system-packages", "sudo rm -rf /",
		// out of a container
		"docker run --privileged img", "docker run -v /:/host img", "docker run --mount type=bind,src=/,dst=/h img",
		"docker run --network host img", "docker run -v /var/run/docker.sock:/s img",
	}
	agents := [][2]string{
		{"developer", "developer"}, {"debugger", "debugger"}, {"benchmarker", "benchmarker"},
		{"reviewer", "reviewer"}, {"operator", "operator"}, {"orchestrator-dev", "orchestrator-dev"},
	}
	for _, ag := range agents {
		rules := effectiveRules(t, ag[0], ag[1])
		for _, cmd := range bypasses {
			assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rules, cmd), "%s: %s", ag[1], cmd)
		}
		for _, path := range []string{".git", "sub/.git", ".git/config", ".beads/redirect", ".beads/hooks/pre-commit"} {
			assert.Equal(t, sessionspec.EffectDeny, ToolEffect(rules, sessionspec.ActionEdit, path), "%s: edit %s", ag[1], path)
		}
		for _, path := range []string{".husky/pre-commit", ".pre-commit-config.yaml", "lefthook.yml"} {
			assert.NotEqual(t, sessionspec.EffectAllow, ToolEffect(rules, sessionspec.ActionEdit, path), "%s: edit %s asks", ag[1], path)
		}
		// L4: no agent reads nor forges a stored report.
		for _, action := range []string{sessionspec.ActionRead, sessionspec.ActionEdit} {
			assert.Equal(t, sessionspec.EffectDeny, ToolEffect(rules, action, "/Users/ada/.oh/sessions/ses_a/reports/rp-1.md"), "%s: %s a report", ag[1], action)
		}
	}

	// The hook files ask only for an agent that may edit them: a read-only
	// agent keeps its refusal (a global ask would loosen it).
	for _, path := range []string{".husky/pre-commit", "web/.husky/pre-commit", ".pre-commit-config.yaml", "lefthook.yml"} {
		assert.Equal(t, sessionspec.EffectAsk, ToolEffect(effectiveRules(t, "developer", "developer"), sessionspec.ActionEdit, path), "developer: edit %s", path)
		for _, ag := range [][2]string{{"reviewer", "reviewer"}, {"debugger", "debugger"}, {"operator", "operator"}} {
			assert.Equal(t, sessionspec.EffectDeny, ToolEffect(effectiveRules(t, ag[0], ag[1]), sessionspec.ActionEdit, path), "%s: edit %s", ag[1], path)
		}
	}

	// What a developer must still run (developer_permissions_test.go and
	// the forms close to the refused ones).
	dev := effectiveRules(t, "developer", "developer")
	for _, cmd := range []string{
		"python3 -m pytest tests/ -v", "python3 --version", "python3 -m venv .venv", ".venv/bin/pip install -r requirements.txt",
		".venv/bin/python -m pytest", "venv/bin/pytest -q", "uv run pytest", "uv sync", "uv pip install -r requirements.txt", "poetry run pytest",
		"poetry install", "pip3 list", "pip show fastapi", "npm install", "npm test", "npx vitest run", "pnpm exec vitest", "go test ./...",
		"bundle exec rspec", "git commit -m 'feat: x'", "git checkout -b feat/x", "git stash list",
		"git log -- internal/gitguard.go", "git commit --no-edit", "git switch -c feat/x", "git diff src/pushNotifications.ts",
		"git status && go test ./... 2>&1 | wc -l", "rg TODO", "grep -rn TODO src", "find . -name '*.go'",
	} {
		assert.Equal(t, sessionspec.EffectAllow, ShellEffect(dev, cmd), cmd)
	}
	assert.NotEqual(t, sessionspec.EffectDeny, ShellEffect(dev, "docker run --rm img"), "docker run asks")
	assert.Equal(t, sessionspec.EffectAllow, ToolEffect(dev, sessionspec.ActionEdit, "src/app.go"))
}

// N3: within a pattern map of the frontmatter, rules are ordered by
// literal length: a short deny loses against a longer allow. This is why
// the refusals that matter are written in Go (global guard), not in the
// frontmatter.
func TestFrontmatterDenyOrdering(t *testing.T) {
	rules := ConvertPermissions(map[string]interface{}{"bash": map[string]interface{}{
		"*": "deny", "*--upload-pack*": "deny", "git fetch origin*": "allow",
	}})
	assert.Equal(t, sessionspec.EffectAllow, ShellEffect(rules, "git fetch origin --upload-pack=/tmp/x"), "the longer allow wins in the frontmatter")
	guarded := append(rules, GlobalShellGuard("")...)
	assert.Equal(t, sessionspec.EffectDeny, ShellEffect(guarded, "git fetch origin --upload-pack=/tmp/x"), "the global guard comes last")
}

// N3 (audit S-19): the context-mode tools that run code never get more
// than the shell of the agent.
func TestCtxToolsFollowBash(t *testing.T) {
	rules := ConvertPermissions(map[string]interface{}{
		"bash": "deny", "ctx_execute": "allow", "ctx_execute_file": "allow", "ctx_batch_execute": "allow", "ctx_search": "allow",
	})
	for _, tool := range []string{"ctx_execute", "ctx_execute_file", "ctx_batch_execute"} {
		e, _ := evaluate(filter(rules, tool), "*")
		assert.Equal(t, sessionspec.EffectDeny, e, tool)
	}
	e, _ := evaluate(filter(rules, "ctx_search"), "*")
	assert.Equal(t, sessionspec.EffectAllow, e, "search runs no code")

	rules = ConvertPermissions(map[string]interface{}{"bash": map[string]interface{}{"*": "ask", "git *": "allow"}, "ctx_execute": "allow"})
	e, _ = evaluate(filter(rules, "ctx_execute"), "*")
	assert.Equal(t, sessionspec.EffectAsk, e, "an allow-list shell asks: so does ctx_execute")
	rules = ConvertPermissions(map[string]interface{}{"ctx_execute": "allow"})
	e, _ = evaluate(filter(rules, "ctx_execute"), "*")
	assert.Equal(t, sessionspec.EffectDeny, e, "no shell rule: refused")
}

func filter(rules []sessionspec.PermissionRule, action string) []sessionspec.PermissionRule {
	var out []sessionspec.PermissionRule
	for _, r := range rules {
		if r.Action == action {
			out = append(out, r)
		}
	}
	return out
}

// N3 (audit S-12): commands that create a commit without `git commit` are
// locked with it.
func TestCommitLockCoversCommitLikeCommands(t *testing.T) {
	wf := &sessionspec.WorkflowRuntime{ID: "t", DefaultMode: "manuel", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-2", Behaviors: map[string]string{"manuel": "pause"}, Unlocks: []string{sessionspec.UnlockCommit}},
	}}
	allow := []sessionspec.PermissionRule{{Action: sessionspec.ActionShell, Resource: "*", Effect: sessionspec.EffectAllow}}
	locked := append(allow, SessionRules(wf, "manuel", nil)...)
	for _, cmd := range []string{"git commit -m x", "git revert HEAD", "git pull origin main", "git merge feat/x", "git cherry-pick abc", "git rebase main", "env git -c a=b commit -m x"} {
		assert.Equal(t, sessionspec.EffectDeny, ShellEffect(locked, cmd), cmd)
	}
	open := append(allow, SessionRules(wf, "manuel", &domain.CheckpointState{Unlocks: map[string]domain.CheckpointUnlock{sessionspec.UnlockCommit: {Checkpoint: "cp-2"}}})...)
	assert.Equal(t, sessionspec.EffectAllow, ShellEffect(open, "git revert HEAD"))
}

// N3 (probe U5): a session rule that allows would open what the guard of an
// agent refuses: the session rules of oh never allow the shell or edits.
func TestSessionRulesNeverAllowShell(t *testing.T) {
	wf := &sessionspec.WorkflowRuntime{ID: "t", DefaultMode: "manuel",
		Checkpoints: []sessionspec.CheckpointDef{
			{ID: "cp-1", Behaviors: map[string]string{"manuel": "auto"}},
			{ID: "cp-2", Behaviors: map[string]string{"manuel": "pause"}, Unlocks: []string{sessionspec.UnlockCommit, sessionspec.UnlockClose, sessionspec.UnlockApply}},
		},
		Gates: []sessionspec.AgentGate{{Agent: "developer", After: "cp-1"}},
	}
	for _, st := range []*domain.CheckpointState{nil, {Breaker: true}, {Passed: map[string]time.Time{"cp-1": time.Now()}}} {
		for _, r := range SessionRules(wf, "manuel", st) {
			if r.Action == sessionspec.ActionShell || r.Action == sessionspec.ActionEdit {
				assert.Equal(t, sessionspec.EffectDeny, r.Effect, "%+v", r)
			}
		}
	}
}

// An agent's shell cannot approve an MCP URL imposed by the team nor change
// the personal settings of the user: the commands that do are refused, with
// the variants already handled for `oh mcp serve` (full path of the binary,
// env, options before or inside the command). The reads stay allowed.
func TestGlobalGuardRefusesOhCommandsThatChangeTrustAndSettings(t *testing.T) {
	forms := [][]string{
		{"mcp", "serve", "jira"},
		{"mcp", "trust", "jira", "--yes"},
		{"mcp", "untrust", "jira"},
		{"mcp", "setup", "gitlab"},
		{"config", "set", "llm.default_provider", "x"},
		{"config", "unset", "llm.default_provider"},
		{"config", "model", "default", "m"},
		{"config", "model", "family", "f", "m"},
		{"config", "model", "agent", "developer", "m"},
		{"config", "model", "unset", "agent", "developer"},
		{"service", "setup"},
		{"service", "remove", "gitlab", "--force"},
	}
	rules := GlobalShellGuard("")
	for _, f := range forms {
		line := strings.Join(f, " ")
		variants := []string{
			"oh " + line,
			"/usr/local/bin/oh " + line,
			"~/.local/bin/oh " + line,
			"env oh " + line,
			"env -i HOME=/tmp /usr/local/bin/oh " + line,
			"X=1 oh " + line,
			"oh --no-tui " + line,
			"oh --log-format json " + line,
			"oh -v " + line,
			"true && oh " + line,
			"sh -c 'oh " + line + "'",
			"echo | oh " + line,
		}
		// options between the words of the command
		variants = append(variants, "oh "+f[0]+" --verbose "+strings.Join(f[1:], " "))
		if len(f) > 2 && f[0] == "config" && f[1] == "model" {
			variants = append(variants, "oh config --x model "+strings.Join(f[2:], " "), "oh config model --x "+strings.Join(f[2:], " "))
		}
		for _, v := range variants {
			assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rules, v), v)
		}
	}

	for _, cmd := range []string{
		"oh mcp status", "oh mcp status --project my-project", "oh mcp list", "oh --no-tui mcp status", "/usr/local/bin/oh mcp status",
		"oh config get llm.default_provider", "oh config list", "oh config path", "oh config model show", "oh config model show --project p",
		"oh doctor", "oh version", "oh session list", "oh project list",
	} {
		assert.NotEqual(t, sessionspec.EffectDeny, ShellEffect(rules, cmd), cmd)
	}
}

// Every shipped agent receives them (the guard is added to all).
func TestShippedAgentsRefuseTrustAndConfigCommands(t *testing.T) {
	for _, ag := range [][2]string{{"developer", "developer"}, {"reviewer", "reviewer"}, {"operator", "operator"}} {
		rules := effectiveRules(t, ag[0], ag[1])
		for _, cmd := range []string{"oh mcp trust jira --yes", "env oh config set mcp.jira.url https://x", "/usr/local/bin/oh service setup", "oh mcp untrust jira"} {
			assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rules, cmd), "%s: %s", ag[1], cmd)
		}
	}
}
