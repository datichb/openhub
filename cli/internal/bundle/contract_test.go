package bundle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// contractOf returns the session contract of a composed prompt.
func contractOf(body string) string {
	i := strings.Index(body, contractHeading)
	if i < 0 {
		return ""
	}
	rest := body[i+2:]
	for _, end := range []string{"\n\n> Brouillons", "\n\n---\n\n# Project instructions\n"} {
		if j := strings.Index(rest, end); j >= 0 {
			rest = rest[:j+1]
		}
	}
	return rest
}

// L3: the session contract of every agent of every shipped workflow is
// pinned in testdata/golden/contracts/<workflow>/<agent>.md
// (go test ./internal/bundle -run ShippedWorkflowContracts -update).
func TestShippedWorkflowContracts(t *testing.T) {
	hub, cat, env := shippedWorkflows(t)
	for _, ref := range cat.Refs() {
		t.Run(ref.ID, func(t *testing.T) {
			r, diags := workflow.Check(cat, ref, nil, env)
			require.False(t, diags.HasErrors(), "%v", diags)
			b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec})
			require.NoError(t, err)
			dir := filepath.Join("testdata", "golden", "contracts", ref.ID)
			want := map[string]bool{}
			for _, a := range b.Spec.Agents {
				c := contractOf(a.Body)
				require.NotEmpty(t, c, "%s/%s: no session contract", ref.ID, a.ID)
				assert.Equal(t, 1, strings.Count(a.Body, contractHeading), "%s/%s: one contract", ref.ID, a.ID)
				want[a.ID+".md"] = true
				assertGolden(t, filepath.Join("contracts", ref.ID, a.ID+".md"), c)
			}
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			for _, e := range entries {
				if !want[e.Name()] {
					if *updateGolden {
						require.NoError(t, os.Remove(filepath.Join(dir, e.Name())))
						continue
					}
					t.Errorf("%s: stale contract %s (-update removes it)", ref.ID, e.Name())
				}
			}
		})
	}
}

// L3 (CP-R11): every checkpoint of a shipped workflow says who passes it
// (`by:`), with members of the workflow that may call workflow_checkpoint,
// and the runtime of the bundle carries it.
func TestCheckpointByMembers(t *testing.T) {
	hub, cat, env := shippedWorkflows(t)
	for _, ref := range cat.Refs() {
		r, diags := workflow.Check(cat, ref, nil, env)
		require.False(t, diags.HasErrors(), "%v", diags)
		members := r.Spec.Members()
		// R7: a method checkpoint is in the bundle of its method; R6: a
		// fan-out checkpoint in the bundle of a fan-out session.
		type kind struct {
			method workflow.Method
			fanout bool
		}
		bundles := map[kind]*Bundle{}
		for _, id := range r.Spec.Checkpoints.Keys() {
			cp, _ := r.Spec.Checkpoints.Get(id)
			if !assert.NotEmpty(t, cp.By, "%s: checkpoint %s has no by:", ref.ID, id) {
				continue
			}
			k := kind{method: cp.Method, fanout: cp.Fanout == workflow.FanoutOnly}
			b := bundles[k]
			if b == nil {
				var err error
				b, err = Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec, Method: cp.Method, Fanout: k.fanout})
				require.NoError(t, err)
				bundles[k] = b
			}
			for _, agent := range cp.By {
				assert.Contains(t, members, agent, "%s: %s.by", ref.ID, id)
				def := findAgent(b.Spec.Agents, agent)
				if assert.NotNil(t, def, "%s: %s.by: %s not in the bundle", ref.ID, id, agent) {
					assert.NotEqual(t, "deny", string(ToolEffect(def.Permissions, CheckpointAction(), "*")),
						"%s: %s passes %s but may not call workflow_checkpoint", ref.ID, agent, id)
					assert.Contains(t, def.Body, "`"+id+"`", "%s: the contract of %s names %s", ref.ID, agent, id)
				}
			}
			for _, c := range b.Spec.Workflow.Checkpoints {
				if c.ID == id {
					assert.Equal(t, cp.By, c.By, "%s: runtime by of %s", ref.ID, id)
				}
			}
		}
	}
}

// CP-R11: an agent's contract and prompt name only its own checkpoints.
func TestContractOwnCheckpointsOnly(t *testing.T) {
	impl := shippedBundle(t, "implement-spec")
	dev := contractOf(agentBody(t, impl, "orchestrator-dev"))
	for _, id := range []string{"cp-plan", "cp-1", "cp-2", "cp-3", "cp-final"} {
		assert.Contains(t, dev, "`"+id+"`")
		assert.NotContains(t, agentBody(t, impl, "developer"), "**"+id+"**", "developer sees the row of %s", id)
	}
	spec := shippedBundle(t, "spec")
	conductor := agentBody(t, spec, "conductor")
	for _, id := range []string{"cp-understanding", "cp-spec", "cp-tickets"} {
		assert.Contains(t, contractOf(conductor), "`"+id+"`")
		assert.NotContains(t, agentBody(t, spec, "planner"), "**"+id+"** — ", "planner sees the row of %s", id)
	}
	assert.Contains(t, contractOf(agentBody(t, spec, "planner")), "aucun n'est à toi")
	assert.Contains(t, contractOf(agentBody(t, spec, "planner")), "`## Retour vers conductor`")
	// libre (orchestrator, feature-migration): no checkpoint at all.
	libre := shippedBundle(t, "libre")
	assert.Contains(t, contractOf(agentBody(t, libre, "pathfinder")), "`## Retour vers orchestrator`")
}

func TestCheckpointByDeduced(t *testing.T) {
	s := &workflow.Spec{Entry: &workflow.Entry{Agent: "lead"}}
	s.Agents.Set("lead", workflow.AgentRef{Role: workflow.RoleWorkflow})
	s.Agents.Set("coord", workflow.AgentRef{Role: workflow.RoleWorkflow})
	s.Agents.Set("dev", workflow.AgentRef{Role: workflow.RoleWorkflow, After: "cp-start"})
	s.Checkpoints.Set("cp-start", workflow.CheckpointSpec{})
	s.Checkpoints.Set("cp-end", workflow.CheckpointSpec{})
	s.Checkpoints.Set("cp-set", workflow.CheckpointSpec{By: []string{"coord"}})
	graph := map[string][]string{"lead": {"coord"}, "coord": {"dev"}}
	assert.Equal(t, []string{"coord"}, s.CheckpointBy("cp-start", graph), "the caller of the gated agent")
	assert.Equal(t, []string{"lead"}, s.CheckpointBy("cp-end", graph), "the entry agent")
	assert.Equal(t, []string{"coord"}, s.CheckpointBy("cp-set", graph), "by: as written")
}

// L3: the contract says whether the agent may ask the user.
func TestContractQuestion(t *testing.T) {
	quick := contractOf(agentBody(t, shippedBundle(t, "quick"), "developer"))
	assert.Contains(t, quick, "outil `question` pour les choix")
	assert.NotContains(t, quick, "## Retour vers")

	ticket := contractOf(agentBody(t, shippedBundle(t, "ticket"), "developer"))
	assert.Contains(t, ticket, "tu n'as pas l'outil `question` (réservé à l'agent d'entrée). Mets chaque décision dans `### Questions bloquantes`")
	assert.Contains(t, ticket, "`## Retour vers orchestrator-dev`")

	brief := contractOf(agentBody(t, shippedBundle(t, "brief-enrich"), "brief-enricher"))
	assert.Contains(t, brief, "headless")
	assert.NotContains(t, brief, "outil `question` pour")
}

// p4-fixes: a pure subagent (no entry role in the workflow) is refused the
// question tool by a deny on "*", the last rule of the action, so that the
// tool hides it (Tool.snapshot of opencode 2.0.20); an entry agent, also
// when other agents may launch it, keeps it.
func TestQuestionRefusedToPureSubagents(t *testing.T) {
	questionEffect := func(b *Bundle, id string) (sessionspec.Effect, bool) {
		for _, a := range b.Spec.Agents {
			if a.ID != id {
				continue
			}
			var last *sessionspec.PermissionRule
			for i, r := range a.Permissions {
				if toolMatch(sessionspec.ActionQuestion, r.Action) {
					last = &a.Permissions[i]
				}
			}
			if last == nil {
				return "", false
			}
			return last.Effect, last.Resource == "*"
		}
		t.Fatalf("agent %s not in the bundle", id)
		return "", false
	}
	for wf, ids := range map[string][]string{
		"spec": {"planner", "pathfinder", "designer"}, "implement-spec": {"developer", "reviewer"},
		"libre": {"pathfinder", "planner", "designer", "orchestrator-dev", "reviewer", "developer"}, "ticket": {"developer", "reviewer"},
		"sweep": {"developer"}, "review-feedback": {"developer"},
	} {
		b := shippedBundle(t, wf)
		for _, id := range ids {
			eff, star := questionEffect(b, id)
			assert.Equal(t, sessionspec.EffectDeny, eff, "%s/%s", wf, id)
			assert.True(t, star, "%s/%s: the deny is on *, so the tool is hidden", wf, id)
			assert.Contains(t, contractOf(agentBody(t, b, id)), "tu n'as pas l'outil `question`", "%s/%s", wf, id)
		}
	}
	for wf, id := range map[string]string{"review": "reviewer", "libre": "orchestrator", "spec": "conductor", "implement-spec": "orchestrator-dev"} {
		eff, _ := questionEffect(shippedBundle(t, wf), id)
		assert.NotEqual(t, sessionspec.EffectDeny, eff, "%s/%s keeps the question tool", wf, id)
	}
	// p4-debts: the developer has the tool where the user talks to it (entry
	// agent of quick, independent primary agent of debug), as its contract
	// says; the base no longer refuses it.
	for _, wf := range []string{"quick", "debug"} {
		b := shippedBundle(t, wf)
		eff, star := questionEffect(b, "developer")
		assert.Equal(t, sessionspec.EffectAllow, eff, "%s/developer", wf)
		assert.True(t, star, "%s/developer", wf)
		assert.Contains(t, contractOf(agentBody(t, b, "developer")), "outil `question` pour les choix", "%s/developer", wf)
	}
}

func TestContractFixRounds(t *testing.T) {
	ticket := shippedBundle(t, "ticket")
	assert.Contains(t, contractOf(agentBody(t, ticket, "orchestrator-dev")), "au plus 3 (developer → reviewer)")
	assert.NotContains(t, contractOf(agentBody(t, ticket, "developer")), "Tours de correction")
}

func TestSessionContractTemplate(t *testing.T) {
	_, err := contractTemplate()
	require.NoError(t, err)
	env := whenEnv{roles: map[string]bool{roleSubagent: true}, mcp: map[string]bool{"workflow": true}, flags: map[string]bool{}}
	got := sessionContract(Request{}, nil, "a", map[string][]string{"a": {"b"}}, map[string][]string{"a": {"x", "y"}}, env, nil)
	assert.True(t, strings.HasPrefix(got, contractHeading), got)
	assert.Contains(t, got, "`## Retour vers x` quand `x` t'invoque, `## Retour vers y` quand `y` t'invoque")
	assert.Contains(t, got, "Agents que tu peux lancer** (outil `task`) : `b`")
	assert.Contains(t, got, "`workflow_report` disponible : dépose ton rapport complet")
	assert.NotContains(t, got, "Checkpoints", "no workflow, no checkpoint line")
	assert.NotContains(t, got, "Wiki du projet", "no wiki server yet")
}

// L5: a caller's contract lists the return statuses of the agents it may
// launch, from their payload skills; the agent itself is left out.
func TestContractCalleeStatuses(t *testing.T) {
	env := whenEnv{roles: map[string]bool{roleEntry: true}, mcp: map[string]bool{}, flags: map[string]bool{}}
	statuses := func(agent string) []string {
		return map[string][]string{"b": {"fait", "bloqué"}, "a": {"x"}}[agent]
	}
	got := sessionContract(Request{}, nil, "a", map[string][]string{"a": {"a", "b", "c"}}, nil, env, statuses)
	assert.Contains(t, got, "**Statuts de leurs retours**")
	assert.Contains(t, got, "  - `b` : `fait`, `bloqué`")
	assert.NotContains(t, got, "  - `a` :", "its own statuses")
	assert.NotContains(t, got, "  - `c` :", "no payload, no line")

	none := sessionContract(Request{}, nil, "a", map[string][]string{"a": {"c"}}, nil, env, statuses)
	assert.NotContains(t, none, "Statuts de leurs retours")

	ticket := contractOf(agentBody(t, shippedBundle(t, "ticket"), "orchestrator-dev"))
	for _, a := range []string{"developer", "reviewer", "documentarian"} {
		assert.Contains(t, ticket, "  - `"+a+"` : `", "orchestrator-dev reads the statuses of %s", a)
	}
	assert.NotContains(t, contractOf(agentBody(t, shippedBundle(t, "ticket"), "documentarian")), "Statuts de leurs retours")
}

// p5-fixes: an agent whose rules refuse the usual helpers of a command
// line is told, in its contract, which ones and that a line holding one is
// refused whole; an agent without shell is not.
func TestContractRestrictedShell(t *testing.T) {
	const line = "- **Shell** : "
	planner := contractOf(agentBody(t, shippedBundle(t, "spec"), "planner"))
	assert.Contains(t, planner, line+"`head`, `tail`, `jq`, `echo`, `which`, `cat` te sont refusés")
	assert.Contains(t, planner, "relance la commande seule")

	developer := contractOf(agentBody(t, shippedBundle(t, "ticket"), "developer"))
	assert.Contains(t, developer, line+"`head`, `tail`, `jq` te sont refusés", "echo, which and cat are allowed to the developer")

	assert.NotContains(t, contractOf(agentBody(t, shippedBundle(t, "spec"), "conductor")), line, "no shell")

	allow := func(p string) sessionspec.PermissionRule {
		return sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: p, Effect: sessionspec.EffectAllow}
	}
	deny := sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: "*", Effect: sessionspec.EffectDeny}
	assert.Equal(t, []string{"tail", "jq", "which", "cat"}, refusedShellHelpers([]sessionspec.PermissionRule{deny, allow("head*"), allow("echo *")}))
	assert.Empty(t, refusedShellHelpers([]sessionspec.PermissionRule{allow("*")}))
}

// p5-fixes: workflow_outputs is hidden (deny on "*", last rule of the
// action) in a workflow without an output the agents may declare: the
// debugger of incident called it; it stays allowed where an output is
// declarable (debug: tickets).
func TestOutputsHiddenWithoutDeclarableOutput(t *testing.T) {
	last := func(b *Bundle, id string) sessionspec.Effect {
		for _, a := range b.Spec.Agents {
			if a.ID != id {
				continue
			}
			eff := sessionspec.Effect("")
			for _, r := range a.Permissions {
				if r.Action == actionOutputs && r.Resource == "*" {
					eff = r.Effect
				}
			}
			return eff
		}
		t.Fatalf("agent %s not in the bundle", id)
		return ""
	}
	incident := shippedBundle(t, "incident")
	for _, id := range []string{"debugger", "developer"} {
		assert.Equal(t, sessionspec.EffectDeny, last(incident, id), "incident/%s", id)
	}
	assert.Equal(t, sessionspec.Effect(""), last(shippedBundle(t, "debug"), "debugger"), "debug declares its tickets")

	recorded := &workflow.Spec{Outputs: []workflow.Output{{ID: "spec", Type: "path", Record: "spec"}}}
	assert.Len(t, OutputsGuard(recorded), 1, "only outputs recorded by oh")
	assert.Nil(t, OutputsGuard(nil))
}
