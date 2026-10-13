package bundle

import (
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// SCR: session scratch folders (globalguard.go). The debug workflow gives
// the debugger a folder, read by the developer.

const scratchSessions = "/home/u/.oh/sessions"

func scratchBundle(t *testing.T, id, ohHome string) *Bundle {
	t.Helper()
	hub, cat, env := shippedWorkflows(t)
	r, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: id}, nil, env)
	require.False(t, diags.HasErrors(), "%v", diags)
	b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec, OhHome: ohHome})
	require.NoError(t, err)
	return b
}

// scratchAgent is an agent of a bundle in a session: its rules, the session
// rules of that session (last), and the guard of the oh plugin (applied
// after, a deny first; plugin=false is the mode without it).
type scratchAgent struct {
	rules, guard []sessionspec.PermissionRule
}

func scratchAgentOf(t *testing.T, b *Bundle, agent, sessionsDir, session string, plugin bool) scratchAgent {
	t.Helper()
	var own []sessionspec.PermissionRule
	for _, a := range b.Spec.Agents {
		if a.ID == agent {
			own = a.Permissions
		}
	}
	require.NotNil(t, own, agent)
	session0 := append(SessionRules(b.Spec.Workflow, "manuel", nil), ScratchSessionRules(b.Spec.Workflow, sessionsDir, session)...)
	sa := scratchAgent{rules: append(append([]sessionspec.PermissionRule(nil), own...), session0...)}
	if plugin {
		sa.guard = append(append([]sessionspec.PermissionRule(nil), b.Spec.AgentGuard...), ScopedGuardFor(agent, b.Spec.ScopedGuard)...)
	} else {
		// Without the plugin the guard is rendered last of the agent rules,
		// before the session rules; the scoped guard is not rendered.
		sa.rules = append(append(append([]sessionspec.PermissionRule(nil), own...), b.Spec.AgentGuard...), session0...)
	}
	return sa
}

func (a scratchAgent) effect(action, resource string) sessionspec.Effect {
	for _, r := range a.guard {
		if r.Effect == sessionspec.EffectDeny && toolMatch(action, r.Action) && toolMatch(resource, r.Resource) {
			return sessionspec.EffectDeny
		}
	}
	return ToolEffect(a.rules, action, resource)
}

// file is what the tool decides for a file tool on a path outside the
// project (opencode 2.0.20, FileAccess): the outside directory `<dir>/*`,
// then the action on the path.
func (a scratchAgent) file(action, p string) sessionspec.Effect {
	return worst(a.effect(ActionOutsideDir, path.Dir(p)+"/*"), a.effect(action, p))
}

// search is a glob or a grep in dir: the outside directory, then the
// action on the pattern.
func (a scratchAgent) search(action, dir, pattern string) sessionspec.Effect {
	return worst(a.effect(ActionOutsideDir, dir+"/*"), a.effect(action, pattern))
}

func (a scratchAgent) shell(line string) sessionspec.Effect {
	return worst(ShellEffect(a.rules, line), ShellEffect(a.guard, line))
}

func worst(effects ...sessionspec.Effect) sessionspec.Effect {
	out := sessionspec.EffectAllow
	for _, e := range effects {
		if restrictiveness(e) > restrictiveness(out) {
			out = e
		}
	}
	return out
}

func TestScratchOwnerReaderOthers(t *testing.T) {
	b := scratchBundle(t, "debug", "")
	require.Equal(t, []sessionspec.ScratchDef{{Owner: "debugger", Readers: []string{"developer"}}}, b.Spec.Workflow.Scratch)
	dir := ScratchDir(scratchSessions, "ses_a", "debugger")
	file := dir + "/repro_test.go"

	for _, plugin := range []bool{true, false} {
		owner := scratchAgentOf(t, b, "debugger", scratchSessions, "ses_a", plugin)
		assert.Equal(t, sessionspec.EffectAllow, owner.file(sessionspec.ActionEdit, file), "the owner writes (plugin %v)", plugin)
		assert.Equal(t, sessionspec.EffectAllow, owner.file(sessionspec.ActionRead, file), "the owner reads")
		assert.Equal(t, sessionspec.EffectAllow, owner.search(actionGrep, dir, "panic"), "the owner searches")
		assert.Equal(t, sessionspec.EffectDeny, owner.effect(sessionspec.ActionEdit, "src/app.go"), "its own edit refusal stays elsewhere")
		assert.Equal(t, sessionspec.EffectDeny, owner.file(sessionspec.ActionEdit, ScratchDir(scratchSessions, "ses_a", "developer")+"/x"), "no folder of another agent")

		reader := scratchAgentOf(t, b, "developer", scratchSessions, "ses_a", plugin)
		assert.Equal(t, sessionspec.EffectAllow, reader.file(sessionspec.ActionRead, file), "the reader reads (plugin %v)", plugin)
		assert.Equal(t, sessionspec.EffectDeny, reader.file(sessionspec.ActionEdit, file), "the reader does not write")
		assert.Equal(t, sessionspec.EffectAllow, reader.search(actionGlob, dir, "*.go"), "the reader searches")
		assert.Equal(t, sessionspec.EffectAllow, reader.effect(sessionspec.ActionEdit, "src/app.go"), "its own rules stay elsewhere")

		other := scratchAgentOf(t, b, "documentarian", scratchSessions, "ses_a", plugin)
		assert.Equal(t, sessionspec.EffectDeny, other.file(sessionspec.ActionRead, file), "another agent does not read (plugin %v)", plugin)
		assert.Equal(t, sessionspec.EffectDeny, other.file(sessionspec.ActionEdit, file), "another agent does not write")
		if plugin {
			assert.Equal(t, sessionspec.EffectDeny, other.search(actionGrep, dir, "panic"), "another agent does not search")
			assert.Equal(t, sessionspec.EffectDeny, other.search(actionList, dir, dir), "another agent does not list")
		}
	}

	// Shell: refused to every agent, the owner included.
	for _, agent := range []string{"debugger", "developer", "documentarian"} {
		a := scratchAgentOf(t, b, agent, scratchSessions, "ses_a", true)
		for _, cmd := range []string{"cat " + file, "cat ~/.oh/sessions/ses_a/scratch/debugger/repro_test.go", "ls $HOME/.oh/sessions/ses_a/scratch", "rm -rf " + dir} {
			assert.Equal(t, sessionspec.EffectDeny, a.shell(cmd), "%s: %s", agent, cmd)
		}
	}
}

// Another session is refused, the owner and the reader included (parallel
// sessions of the same bundle share the agent rules).
func TestScratchOtherSession(t *testing.T) {
	b := scratchBundle(t, "debug", "")
	file := ScratchDir(scratchSessions, "ses_a", "debugger") + "/repro_test.go"
	for _, plugin := range []bool{true, false} {
		for _, agent := range []string{"debugger", "developer", "documentarian"} {
			a := scratchAgentOf(t, b, agent, scratchSessions, "ses_b", plugin)
			assert.Equal(t, sessionspec.EffectDeny, a.file(sessionspec.ActionRead, file), "%s in ses_b reads ses_a (plugin %v)", agent, plugin)
			assert.Equal(t, sessionspec.EffectDeny, a.file(sessionspec.ActionEdit, file), "%s in ses_b writes ses_a", agent)
			assert.Equal(t, sessionspec.EffectDeny, a.search(actionGrep, path.Dir(file), "x"), "%s in ses_b searches ses_a", agent)
		}
		// Its own folder in ses_b stays open to the owner.
		owner := scratchAgentOf(t, b, "debugger", scratchSessions, "ses_b", plugin)
		assert.Equal(t, sessionspec.EffectAllow, owner.file(sessionspec.ActionEdit, ScratchDir(scratchSessions, "ses_b", "debugger")+"/x.sh"))
	}
	// A session whose rules are not set yet (subagent session before the
	// daemon applies them): the agent rules refuse every outside directory.
	own := scratchAgentOf(t, b, "debugger", scratchSessions, "", true)
	own.rules = nil
	for _, a := range b.Spec.Agents {
		if a.ID == "debugger" {
			own.rules = a.Permissions
		}
	}
	assert.Equal(t, sessionspec.EffectDeny, own.file(sessionspec.ActionEdit, file))
}

// The guards come after the rules of the agent: an agent that allows
// everything is still refused, in this order (refusal, owner, readers).
func TestScratchGuardOrder(t *testing.T) {
	wf := &sessionspec.WorkflowRuntime{Scratch: []sessionspec.ScratchDef{{Owner: "a", Readers: []string{"b"}}}}
	allowAll := []sessionspec.PermissionRule{}
	for _, action := range scratchFileActions {
		allowAll = append(allowAll, sessionspec.PermissionRule{Action: action, Resource: "*", Effect: sessionspec.EffectAllow})
	}
	agentOf := func(agent, session string) scratchAgent {
		rules := append(append(append([]sessionspec.PermissionRule(nil), allowAll...), ScratchAgentGuard(agent, wf, "")...), ScratchSessionRules(wf, scratchSessions, session)...)
		return scratchAgent{rules: rules, guard: ScopedGuardFor(agent, ScratchScopedGuard(wf, ""))}
	}
	file := ScratchDir(scratchSessions, "s1", "a") + "/notes.md"
	assert.Equal(t, sessionspec.EffectAllow, agentOf("a", "s1").file(sessionspec.ActionEdit, file))
	assert.Equal(t, sessionspec.EffectDeny, agentOf("b", "s1").file(sessionspec.ActionEdit, file), "the reader's own allow does not win")
	assert.Equal(t, sessionspec.EffectAllow, agentOf("b", "s1").file(sessionspec.ActionRead, file))
	assert.Equal(t, sessionspec.EffectDeny, agentOf("c", "s1").file(sessionspec.ActionRead, file), "another agent's own allow does not win")
	assert.Equal(t, sessionspec.EffectDeny, agentOf("a", "s2").file(sessionspec.ActionRead, file), "another session")
	assert.Equal(t, sessionspec.EffectDeny, agentOf("a", "s1").file(sessionspec.ActionEdit, ScratchDir(scratchSessions, "s1", "zz")+"/x"), "an undeclared folder")

	// Order of the layers: the refusal first, then the exceptions.
	rules := ScratchAgentGuard("a", wf, "")
	last := map[sessionspec.Effect]int{}
	first := map[sessionspec.Effect]int{}
	for i, r := range rules {
		if _, ok := first[r.Effect]; !ok {
			first[r.Effect] = i
		}
		last[r.Effect] = i
	}
	assert.Less(t, last[sessionspec.EffectDeny], first[sessionspec.EffectAllow], "refusals before the owner's exception")
	session := ScratchSessionRules(wf, scratchSessions, "s1")
	require.NotEmpty(t, session)
	assert.Equal(t, sessionspec.EffectDeny, session[0].Effect)
	assert.Equal(t, sessionspec.PermissionRule{Action: ActionOutsideDir, Resource: scratchSessions + "/s1/scratch/a/*", Effect: sessionspec.EffectAllow}, session[len(session)-1])

	// A broader refusal of the session data (layer 0, to come) keeps the
	// exceptions after it.
	saved := sessionDataDeny
	sessionDataDeny = []string{"*.oh/sessions*"}
	defer func() { sessionDataDeny = saved }()
	assert.Equal(t, sessionspec.EffectAllow, agentOf("a", "s1").file(sessionspec.ActionEdit, file))
	assert.Equal(t, sessionspec.EffectAllow, agentOf("b", "s1").file(sessionspec.ActionRead, file))
	assert.Equal(t, sessionspec.EffectDeny, agentOf("a", "s1").file(sessionspec.ActionRead, scratchSessions+"/s1/env.json"))
	assert.Equal(t, sessionspec.EffectDeny, agentOf("c", "s1").file(sessionspec.ActionRead, scratchSessions+"/s1/context.json"))
}

// A workflow without scratch folder refuses them all, once in the global
// guard; nothing is added to the agents nor to the session rules.
func TestScratchWithoutFolder(t *testing.T) {
	b := scratchBundle(t, "ticket", "")
	assert.Empty(t, b.Spec.ScopedGuard)
	assert.Nil(t, ScratchSessionRules(b.Spec.Workflow, scratchSessions, "ses_a"))
	file := ScratchDir(scratchSessions, "ses_a", "debugger") + "/repro_test.go"
	for _, a := range b.Spec.Agents {
		for _, r := range a.Permissions {
			assert.NotContains(t, r.Resource, "/scratch", a.ID)
		}
		for _, plugin := range []bool{true, false} {
			ag := scratchAgentOf(t, b, a.ID, scratchSessions, "ses_a", plugin)
			assert.Equal(t, sessionspec.EffectDeny, ag.file(sessionspec.ActionRead, file), "%s (plugin %v)", a.ID, plugin)
			assert.Equal(t, sessionspec.EffectDeny, ag.file(sessionspec.ActionEdit, file), a.ID)
			assert.Equal(t, sessionspec.EffectDeny, ag.shell("cat "+file), a.ID)
		}
	}
}

// A custom oh home is refused by its path too.
func TestScratchCustomOhHome(t *testing.T) {
	home := "/data/oh"
	sessions := home + "/sessions"
	b := scratchBundle(t, "debug", home)
	file := ScratchDir(sessions, "ses_a", "debugger") + "/x.sh"
	owner := scratchAgentOf(t, b, "debugger", sessions, "ses_a", true)
	assert.Equal(t, sessionspec.EffectAllow, owner.file(sessionspec.ActionEdit, file))
	assert.Equal(t, sessionspec.EffectDeny, owner.shell("cat "+file))
	other := scratchAgentOf(t, b, "documentarian", sessions, "ses_a", true)
	assert.Equal(t, sessionspec.EffectDeny, other.file(sessionspec.ActionRead, file))
	reader := scratchAgentOf(t, b, "developer", sessions, "ses_b", true)
	assert.Equal(t, sessionspec.EffectDeny, reader.file(sessionspec.ActionRead, file), "another session")

	tb := scratchBundle(t, "ticket", home)
	dev := scratchAgentOf(t, tb, "developer", sessions, "ses_a", true)
	assert.Equal(t, sessionspec.EffectDeny, dev.file(sessionspec.ActionRead, file))
}

// The owner and the readers are told where their folder is; the others
// are not.
func TestScratchNote(t *testing.T) {
	b := scratchBundle(t, "debug", "")
	for _, a := range b.Spec.Agents {
		note := strings.Contains(a.Body, "Brouillons de session (oh)")
		switch a.ID {
		case "debugger":
			assert.True(t, note)
			assert.Contains(t, a.Body, "lecture et écriture")
		case "developer":
			assert.True(t, note)
			assert.Contains(t, a.Body, "sans les modifier, les brouillons de `debugger`")
		default:
			assert.False(t, note, a.ID)
		}
		if note {
			assert.Contains(t, a.Body, "`oh.scratch`")
			assert.NotContains(t, a.Body, ".oh/sessions/", "the path is given by the session context")
		}
	}
}
