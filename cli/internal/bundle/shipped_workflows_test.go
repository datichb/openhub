package bundle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"text/template/parse"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/workflow/hubcat"
)

// Shipped workflows (<repo>/workflows): every one is valid against the hub,
// its prompt renders, its free-text inputs are delimited (O11), and the
// bundle compiled from it is pinned by golden files
// (go test ./internal/bundle -run ShippedWorkflow -update).

func shippedWorkflows(t *testing.T) (string, *workflow.MemCatalog, workflow.Env) {
	t.Helper()
	hub := repoHub(t)
	cat, diags := hubcat.LoadWorkflows(hub)
	require.Empty(t, diags, "loading %s/workflows", hub)
	require.NotEmpty(t, cat.Refs(), "no shipped workflow")
	hc, err := hubcat.New(hub)
	require.NoError(t, err)
	env := hc.Env()
	env.Skills = NewSkillCatalog(hub)
	return hub, cat, env
}

func TestShippedWorkflowsAreValid(t *testing.T) {
	_, cat, env := shippedWorkflows(t)
	for _, ref := range cat.Refs() {
		t.Run(ref.ID, func(t *testing.T) {
			_, diags := workflow.Check(cat, ref, nil, env)
			assert.Empty(t, diags, "oh workflow validate %s", ref.ID)
		})
	}
}

// sampleInputs gives a value to every input (all) or to the required ones.
func sampleInputs(s *workflow.Spec, all bool) map[string]any {
	out := map[string]any{}
	for _, k := range s.Inputs.Keys() {
		in, _ := s.Inputs.Get(k)
		if !all && (!in.Required || in.Default != nil) {
			continue
		}
		switch in.Type {
		case workflow.InputBeadsID:
			out[k] = "bd-1"
		case workflow.InputBeadsIDs:
			out[k] = "bd-2,bd-3"
		case workflow.InputBool:
			out[k] = "true"
		case workflow.InputInt:
			out[k] = "1"
		case workflow.InputEnum:
			out[k] = in.Values[len(in.Values)-1]
		case workflow.InputBranch:
			out[k] = "feat/exemple"
		case workflow.InputPath:
			out[k] = "src/exemple"
		default:
			out[k] = "Exemple de valeur pour " + k
		}
	}
	return out
}

// extraSamples are additional prompt renderings for branches of a template
// that the full and minimal samples do not reach.
var extraSamples = map[string]map[string]map[string]any{
	"sweep": {
		"manual-custom": {"goal": "Migrer les appels dépréciés", "strategy": "manual", "tasks": "tâche 1\ntâche 2", "verify": "custom", "verify_cmd": "make check"},
		"tests":         {"goal": "Migrer les appels dépréciés", "verify": "tests"},
	},
	"review":     {"mode-only": {"review_mode": "adversarial"}},
	"onboarding": {"refresh": {"refresh": "true"}},
}

func TestShippedWorkflowPrompts(t *testing.T) {
	hub, cat, _ := shippedWorkflows(t)
	hc, err := hubcat.New(hub)
	require.NoError(t, err)
	for _, ref := range cat.Refs() {
		t.Run(ref.ID, func(t *testing.T) {
			doc, _ := cat.Lookup(ref)
			variants := map[string]map[string]any{
				"full":    sampleInputs(doc.Spec, true),
				"minimal": sampleInputs(doc.Spec, false),
			}
			for name, inputs := range extraSamples[ref.ID] {
				variants[name] = inputs
			}
			for _, variant := range sortedKeys(variants) {
				r, diags := workflow.ResolveSpec(cat, ref, &workflow.SessionOptions{Inputs: variants[variant]})
				require.False(t, diags.HasErrors(), "%v", diags)
				mode := r.Mode
				if mode == "" {
					mode = r.Spec.DefaultMode()
				}
				out, err := workflow.RenderPrompt(r, hc, workflow.PromptContext{
					Project: "demo", Location: "/src/demo", Mode: mode, Runtime: "local", Lang: "fr", Workflow: ref.ID,
				})
				require.NoError(t, err)
				assert.Contains(t, out, "Mode de workflow : "+mode, "contract with the entry agent")
				assert.Contains(t, out, "Langue de réponse : fr")
				assertGolden(t, filepath.Join("workflows", ref.ID, "prompt-"+variant+".md"), out)
			}
		})
	}
}

// TestShippedWorkflowTextInputsAreDelimited checks that string and text
// inputs only reach the prompt through `data` (O11); conditions may test them.
func TestShippedWorkflowTextInputsAreDelimited(t *testing.T) {
	hub, cat, _ := shippedWorkflows(t)
	hc, err := hubcat.New(hub)
	require.NoError(t, err)
	for _, ref := range cat.Refs() {
		r, diags := workflow.ResolveSpec(cat, ref, nil)
		require.False(t, diags.HasErrors(), "%v", diags)
		src, err := r.ReadPromptSource(hc)
		require.NoError(t, err)
		free := map[string]bool{}
		for _, k := range r.Spec.Inputs.Keys() {
			in, _ := r.Spec.Inputs.Get(k)
			if in.Type == workflow.InputString || in.Type == workflow.InputText {
				free[k] = true
			}
		}
		for _, name := range undelimitedFields(t, src, free) {
			t.Errorf("%s: input %q is written without data", ref.ID, name)
		}
	}
}

func undelimitedFields(t *testing.T, src string, free map[string]bool) []string {
	t.Helper()
	tr := parse.New("prompt")
	tr.Mode = parse.SkipFuncCheck
	trees := map[string]*parse.Tree{}
	_, err := tr.Parse(src, "", "", trees)
	require.NoError(t, err)
	var bad []string
	var walk func(n parse.Node, cond bool)
	walk = func(n parse.Node, cond bool) {
		switch x := n.(type) {
		case *parse.ListNode:
			if x != nil {
				for _, c := range x.Nodes {
					walk(c, cond)
				}
			}
		case *parse.ActionNode:
			walk(x.Pipe, cond)
		case *parse.IfNode:
			walk(x.Pipe, true)
			walk(x.List, cond)
			walk(x.ElseList, cond)
		case *parse.RangeNode:
			walk(x.Pipe, false)
			walk(x.List, cond)
			walk(x.ElseList, cond)
		case *parse.WithNode:
			walk(x.Pipe, false)
			walk(x.List, cond)
			walk(x.ElseList, cond)
		case *parse.PipeNode:
			if x != nil {
				for _, c := range x.Cmds {
					walk(c, cond)
				}
			}
		case *parse.CommandNode:
			if id, ok := x.Args[0].(*parse.IdentifierNode); ok && id.Ident == "data" {
				return
			}
			for _, a := range x.Args {
				walk(a, cond)
			}
		case *parse.FieldNode:
			if !cond && free[x.Ident[0]] {
				bad = append(bad, x.Ident[0])
			}
		}
	}
	for _, tree := range trees {
		walk(tree.Root, false)
	}
	return bad
}

// bundleSummary is the golden view of a compiled bundle: everything but
// the agent bodies (pinned through the generated skills) and the hash.
type bundleSummary struct {
	EntryAgent    string              `json:"entry_agent"`
	Agents        []agentSummary      `json:"agents"`
	Skills        []string            `json:"skills"`
	SubagentGraph map[string][]string `json:"subagent_graph"`
	MaxDepth      int                 `json:"max_depth"`
	CodeMode      bool                `json:"code_mode"`
	Plugins       []string            `json:"plugins,omitempty"`
}

type agentSummary struct {
	ID          string   `json:"id"`
	Mode        string   `json:"mode"`
	Model       string   `json:"model,omitempty"`
	Permissions []string `json:"permissions"`
}

func TestShippedWorkflowBundles(t *testing.T) {
	hub, cat, env := shippedWorkflows(t)
	hc, err := hubcat.New(hub)
	require.NoError(t, err)
	for _, ref := range cat.Refs() {
		t.Run(ref.ID, func(t *testing.T) {
			r, diags := workflow.Check(cat, ref, nil, env) // as the launch: selectable entries expanded
			require.False(t, diags.HasErrors(), "%v", diags)
			b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec})
			require.NoError(t, err)

			s := b.Spec
			sum := bundleSummary{EntryAgent: s.EntryAgent, SubagentGraph: s.SubagentGraph, MaxDepth: s.MaxDepth, CodeMode: s.CodeMode}
			for _, a := range s.Agents {
				as := agentSummary{ID: a.ID, Mode: a.Mode, Permissions: []string{}}
				if a.Model != nil {
					as.Model = a.Model.Provider + "/" + a.Model.Model
				}
				for _, p := range a.Permissions {
					as.Permissions = append(as.Permissions, p.Action+" "+p.Resource+" "+string(p.Effect))
				}
				sum.Agents = append(sum.Agents, as)
			}
			for _, sk := range s.Skills {
				sum.Skills = append(sum.Skills, sk.ID)
			}
			sort.Strings(sum.Skills)
			for _, p := range s.Plugins {
				sum.Plugins = append(sum.Plugins, p.ID)
			}
			data, err := json.MarshalIndent(sum, "", "  ")
			require.NoError(t, err)
			assertGolden(t, filepath.Join("workflows", ref.ID, "bundle.json"), string(data)+"\n")

			_, graph := workflow.SubagentGraph(r.Spec, hc)
			gen, err := GenerateWorkflowSkills(r.Spec, graph, func(id string) workflow.AgentMode {
				info, _ := hc.Agent(id)
				return info.Mode
			})
			require.NoError(t, err)
			for skill, content := range gen {
				assertGolden(t, filepath.Join("workflows", ref.ID, strings.ReplaceAll(skill, "/", "__")+".md"), content)
			}
		})
	}
}

func assertGolden(t *testing.T, name, content string) {
	t.Helper()
	golden := filepath.Join("testdata", "golden", name)
	if *updateGolden {
		require.NoError(t, os.MkdirAll(filepath.Dir(golden), 0o755))
		require.NoError(t, os.WriteFile(golden, []byte(content), 0o644))
		return
	}
	want, err := os.ReadFile(golden)
	require.NoError(t, err, "missing golden file (go test ./internal/bundle -run ShippedWorkflow -update)")
	assert.Equal(t, string(want), content, name)
}
