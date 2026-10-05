package bundle

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/datichb/openhub/cli/internal/workflow"
)

// Chain skills generated from an oh/v1 workflow (P1-T11). They replace the
// static skills of the same reference in the bundle (inlined or on demand),
// so that every agent reads the enchaînement of the workflow it runs in,
// never a hard-coded one.
//
// The bundle does not depend on the session mode (sessions of every mode
// share a server group): the generated text lists every allowed mode, and
// the session prompt states the mode.

//go:embed templates/*.md.tmpl
var templateFS embed.FS

// Skill references generated from the workflow.
const (
	// WorkflowMapSkill is the full map, read by the conductor (P1-T12).
	WorkflowMapSkill = "workflow/workflow-map"
	// Legacy references kept for the agents that already load them.
	workflowModesSkill     = "orchestrator/orchestrator-workflow-modes"
	workflowReferenceSkill = "shared/hub-workflow-reference"
	orchestratorModesSkill = "orchestrator/orchestrator-modes"
	ticketRoutingSkill     = "orchestrator/orchestrator-ticket-routing"
)

var generatedTemplates = map[string]string{
	WorkflowMapSkill:       "workflow-map.md.tmpl",
	workflowModesSkill:     "orchestrator-workflow-modes.md.tmpl",
	workflowReferenceSkill: "hub-workflow-reference.md.tmpl",
	orchestratorModesSkill: "orchestrator-modes.md.tmpl",
	ticketRoutingSkill:     "orchestrator-ticket-routing.md.tmpl",
}

// generatedLang is the language of the generated text (the hub content is
// written in French). Fixed so that the bundle hash does not depend on the
// UI language.
const generatedLang = "fr"

// mapData is the template view of a workflow.
type mapData struct {
	ID, Description, Entry string
	Risk                   workflow.Risk
	Beads                  []string
	Agents                 []agentRow
	// ActiveAgents feeds the legacy orchestrator-modes / ticket-routing templates.
	ActiveAgents   []agentRow
	Modes          []string
	DefaultMode    string
	Checkpoints    []checkpointRow
	Remote         bool
	MaxConsecutive int
	Outputs        []outputRow
}

type agentRow struct {
	ID, AgentID string
	Role        workflow.AgentRole
	Mode        workflow.AgentMode
	Entry       bool
	After       string
	Calls       []string
}

type checkpointRow struct {
	ID, Label, Description, Condition string
	Mandatory                         bool
	Behaviors                         []workflow.CheckpointBehavior // one per allowed mode
	Remote                            workflow.RemotePolicy
}

type outputRow struct {
	ID    string
	Type  workflow.OutputType
	Label string
}

// GenerateWorkflowSkills renders the chain skills of spec. graph is the
// delegation graph of the bundle; modeOf gives the mode of a catalogue agent
// (used when the workflow does not set one).
func GenerateWorkflowSkills(spec *workflow.Spec, graph map[string][]string, modeOf func(id string) workflow.AgentMode) (map[string]string, error) {
	data := buildMapData(spec, graph, modeOf)
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"codes": func(ss []string) string {
			out := make([]string, len(ss))
			for i, s := range ss {
				out[i] = "`" + s + "`"
			}
			return strings.Join(out, ", ")
		},
		"hasAgent": func(agents []agentRow, id string) bool {
			for _, a := range agents {
				if a.ID == id {
					return true
				}
			}
			return false
		},
		"planningAgent": func(agents []agentRow) string {
			for _, want := range []string{"planner", "pathfinder"} {
				for _, a := range agents {
					if a.ID == want {
						return want
					}
				}
			}
			return "planning agent"
		},
	}).ParseFS(templateFS, "templates/*.md.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parsing workflow skill templates: %w", err)
	}
	out := make(map[string]string, len(generatedTemplates))
	for ref, name := range generatedTemplates {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
			return nil, fmt.Errorf("generating %s: %w", ref, err)
		}
		out[ref] = buf.String()
	}
	return out, nil
}

func buildMapData(spec *workflow.Spec, graph map[string][]string, modeOf func(string) workflow.AgentMode) mapData {
	d := mapData{
		ID:          spec.ID,
		Description: strings.TrimSpace(spec.Description.Text(generatedLang)),
		Entry:       spec.EntryAgent(),
		Risk:        spec.Risk,
		Modes:       spec.AllowedModes(),
		DefaultMode: spec.DefaultMode(),
		Remote:      spec.AllowsRuntime(workflow.RuntimeRemote),
	}
	if spec.Beads != nil {
		d.Beads = spec.Beads.Allow
	}
	if cb := spec.CircuitBreaker; cb != nil && cb.MaxConsecutiveSubagents != nil {
		d.MaxConsecutive = *cb.MaxConsecutiveSubagents
	}
	for _, id := range spec.Members() {
		ref, listed := spec.Agents.Get(id)
		row := agentRow{ID: id, AgentID: id, Role: workflow.RoleWorkflow, Entry: id == d.Entry, Calls: graph[id]}
		if listed {
			row.Role, row.After, row.Mode = ref.Role, ref.After, ref.Mode
		}
		if row.Mode == "" && modeOf != nil {
			row.Mode = modeOf(id)
		}
		if row.Mode == "" {
			row.Mode = workflow.ModePrimary
		}
		d.Agents = append(d.Agents, row)
	}
	d.ActiveAgents = d.Agents
	for _, id := range spec.Checkpoints.Keys() {
		cp, _ := spec.Checkpoints.Get(id)
		row := checkpointRow{
			ID:          id,
			Label:       cp.Label.Text(generatedLang),
			Description: strings.TrimSpace(cp.Description.Text(generatedLang)),
			Condition:   cp.Condition,
			Mandatory:   cp.IsMandatory(),
			Remote:      cp.RemotePolicy(),
		}
		for _, m := range d.Modes {
			b := cp.Behavior(m)
			if row.Mandatory && b == workflow.BehaviorSkip {
				b = workflow.BehaviorPause // a mandatory checkpoint is never skipped
			}
			row.Behaviors = append(row.Behaviors, b)
		}
		d.Checkpoints = append(d.Checkpoints, row)
	}
	for _, o := range spec.Outputs {
		d.Outputs = append(d.Outputs, outputRow{ID: o.ID, Type: o.Type, Label: o.Label.Text(generatedLang)})
	}
	return d
}
